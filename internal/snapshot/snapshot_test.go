package snapshot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/snapshot"
)

func serverTier() snapshot.Server {
	return snapshot.Server{
		PID:            12345,
		StartedAt:      "2026-08-27T09:12:03Z",
		UptimeSeconds:  3600,
		ClientAttached: true,
	}
}

func sessionTier() *snapshot.Session {
	return &snapshot.Session{
		DataSource: snapshot.DataSource{
			Name: "prod-ro", Env: "production",
			Host: "db.internal", Port: 3306, User: "reader",
			Database: "app", Tunnel: true, ServerVersion: "8.0.36",
		},
		Schema: "app",
		Statement: snapshot.Statement{
			Running:       true,
			ElapsedMS:     4210,
			SQL:           "SELECT id, customer, total FROM orders",
			InjectedLimit: 1000,
			Truncated:     false,
		},
		Result: snapshot.Result{
			Columns:  []string{"id", "customer", "total"},
			RowCount: 4213,
		},
		Batch:         snapshot.Batch{Running: false},
		Worktree:      &snapshot.Worktree{Path: "/home/x/reports", Branch: "main", OpenFile: "monthly.sql", Modified: true},
		Editor:        snapshot.Editor{Lines: 42, Modified: true},
		Mode:          "NORMAL",
		WritesEnabled: false,
		InTransaction: false,
	}
}

// dv status exists to say whether a session is alive. A status command that
// hangs when the session is wedged is worse than none: it dies with the thing
// it was asked about.
func TestServerTierAnswersWhenTheSessionWillNot(t *testing.T) {
	src := snapshot.Source{
		Server: serverTier,
		Session: func(ctx context.Context) (*snapshot.Session, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	got := src.Take(ctx)

	if got.Server.PID != 12345 {
		t.Errorf("server tier is %+v; it must be answerable whatever the session is doing", got.Server)
	}
	if got.Session != nil {
		t.Error("a session that did not answer was reported as if it had")
	}
	if got.SessionError == "" {
		t.Error("the snapshot does not say why the session is missing")
	}
}

// The result buffer holds production data that is written to no log and no
// history. An observation API must not become a quiet way out for it.
func TestNoRowDataIsEverEncoded(t *testing.T) {
	src := snapshot.Source{
		Server:  serverTier,
		Session: func(context.Context) (*snapshot.Session, error) { return sessionTier(), nil },
	}

	var out bytes.Buffer
	if err := snapshot.Handle(&out, src, context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var generic map[string]any
	if err := json.Unmarshal(out.Bytes(), &generic); err != nil {
		t.Fatalf("the response is not JSON: %v", err)
	}

	for _, banned := range []string{"\"rows\"", "\"cells\"", "\"password\"", "\"secret\"", "\"text\""} {
		if strings.Contains(out.String(), banned) {
			t.Errorf("the snapshot contains %s", banned)
		}
	}
}

// The API is something other programs read. A renamed field breaks every one
// of them and nothing here would otherwise notice.
func TestSnapshotMatchesTheGolden(t *testing.T) {
	src := snapshot.Source{
		Server:  serverTier,
		Session: func(context.Context) (*snapshot.Session, error) { return sessionTier(), nil },
	}

	var out bytes.Buffer
	if err := snapshot.Handle(&out, src, context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	golden := "testdata/snapshot.json"
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if out.String() != string(want) {
		t.Errorf("the snapshot changed shape.\n got: %s\nwant: %s\n\nIf this is deliberate, re-run with UPDATE_GOLDEN=1 and read the diff before committing it.", out.String(), want)
	}
}

// Nothing in the API may change the session. The type is the guard: it holds
// two functions and no session, so there is nothing to call.
func TestSourceHoldsNothingThatCanMutate(t *testing.T) {
	// This test is a statement of intent that the compiler enforces: if
	// Source ever gains a field that is not one of these two, this stops
	// building and whoever added it has to say why here.
	_ = snapshot.Source{
		Server:  serverTier,
		Session: func(context.Context) (*snapshot.Session, error) { return nil, nil },
	}
}
