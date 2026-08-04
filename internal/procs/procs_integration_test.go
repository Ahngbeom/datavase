//go:build integration

package procs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func open(t *testing.T, user, password string) *db.Conn {
	t.Helper()

	ds, ownPassword := testmysql.DataSource(t)
	if user != "" {
		ds.User, ownPassword = user, password
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, ownPassword, "")
	if err != nil {
		t.Fatalf("db.Open(%s) error = %v", ds.User, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// The listing has to show the connection that is actually working, which is
// the one it is opened to find.
func TestListSeesAStatementRunningOnAnotherConnection(t *testing.T) {
	watcher := open(t, "", "")
	busy := open(t, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// A statement held open on a connection of its own, cancelled when the
	// test ends rather than waited out.
	sleeping, cancelSleep := context.WithCancel(context.Background())
	defer cancelSleep()

	streams := make(chan *db.Stream, 1)
	go func() {
		stream := busy.Query(sleeping, "SELECT SLEEP(10)", db.Options{})
		streams <- stream
		defer stream.Close()
		for range stream.Events {
		}
	}()
	// Cancel rather than only dropping the context: the context detaches the
	// client and leaves the server running the statement to the end, on a
	// server the rest of the suite is sharing.
	defer func() {
		if stream := <-streams; stream != nil {
			stream.Cancel()
		}
	}()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		listing, err := List(ctx, watcher)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if !listing.Complete {
			t.Fatal("the test user cannot see other connections; the listing would prove nothing")
		}

		for i, p := range listing.Processes {
			if !p.Working() || !strings.Contains(p.SQL, "SLEEP(10)") {
				continue
			}

			// Ordered ahead of every idle connection. Deliberately not "first
			// in the list": the suite runs its packages in parallel against
			// one server, so another test's statement may well be older and
			// belongs above this one — which is the ordering working, not
			// failing.
			for _, other := range listing.Processes[i+1:] {
				if !other.Working() {
					continue
				}
				if other.Elapsed > p.Elapsed {
					t.Errorf("a shorter statement sorted above a longer one:\n%+v", listing.Processes)
				}
			}
			for j, before := range listing.Processes[:i] {
				if !before.Working() {
					t.Errorf("an idle connection at %d sorted above the running statement at %d:\n%+v",
						j, i, listing.Processes)
				}
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("the running statement never appeared in the listing")
}

// Without the PROCESS privilege the server answers with the user's own
// connections and no error at all, so the listing has to say the view is
// partial rather than let a short list read as a quiet server.
func TestAUserWithoutThePrivilegeIsToldTheViewIsPartial(t *testing.T) {
	grantLimitedUser(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	full, err := List(ctx, open(t, "", ""))
	if err != nil {
		t.Fatalf("List() as the test user error = %v", err)
	}
	if !full.Complete {
		t.Error("the test user has PROCESS and the listing says otherwise")
	}

	limited, err := List(ctx, open(t, limitedUser, limitedPassword))
	if err != nil {
		t.Fatalf("List() as the limited user error = %v", err)
	}
	if limited.Complete {
		t.Error("a user without PROCESS was told the listing was complete")
	}
}

const (
	limitedUser     = "dv_procs_limited"
	limitedPassword = "limited"
)

// grantLimitedUser makes a user that may connect and read, and may not see
// anyone else's connections.
func grantLimitedUser(t *testing.T) {
	t.Helper()

	conn := open(t, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, stmt := range []string{
		"CREATE USER IF NOT EXISTS '" + limitedUser + "'@'%' IDENTIFIED BY '" + limitedPassword + "'",
		"GRANT SELECT ON *.* TO '" + limitedUser + "'@'%'",
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Skipf("cannot create the limited user (%v); this server does not let the test user grant", err)
		}
	}
}

// Stopping someone else's statement, which is the operation this exists for.
func TestKillStopsAnotherConnectionsStatement(t *testing.T) {
	watcher := open(t, "", "")
	victim := open(t, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	running, stopRunning := context.WithCancel(context.Background())
	defer stopRunning()

	failed := make(chan error, 1)
	go func() {
		stream := victim.Query(running, "SELECT SLEEP(25)", db.Options{})
		defer stream.Close()
		for range stream.Events {
		}
		failed <- stream.Err()
	}()

	id := waitForStatement(t, ctx, watcher, "SLEEP(25)")
	if err := Kill(ctx, watcher, id, StopStatement); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	select {
	case err := <-failed:
		if err == nil {
			t.Error("the statement finished on its own rather than being stopped")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the statement was not stopped")
	}

	// The connection survives a statement-level kill, which is the whole
	// difference between the two.
	if err := runOne(ctx, victim); err != nil {
		t.Errorf("the connection did not survive having its statement stopped: %v", err)
	}
}

// Killing our own connection would take cancellation and every catalog read
// with it. It is refused rather than attempted.
func TestKillRefusesThisSessionsOwnConnections(t *testing.T) {
	conn := open(t, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	listing, err := List(ctx, conn)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var ours uint64
	for _, p := range listing.Processes {
		if conn.Owns(p.ID) {
			ours = p.ID
			break
		}
	}
	if ours == 0 {
		t.Fatal("none of the listed connections is ours; the test would prove nothing")
	}

	if err := Kill(ctx, conn, ours, StopConnection); !errors.Is(err, ErrOwnConnection) {
		t.Errorf("Kill(own connection) = %v, want ErrOwnConnection", err)
	}
	// And it still works, which is what the refusal was protecting.
	if err := runOne(ctx, conn); err != nil {
		t.Errorf("our own connection stopped working: %v", err)
	}
}

// runOne sends the smallest statement there is, and reports whether it landed.
func runOne(ctx context.Context, conn *db.Conn) error {
	stream := conn.Query(ctx, "SELECT 1", db.Options{})
	defer stream.Close()

	for range stream.Events {
	}
	return stream.Err()
}

// waitForStatement finds the connection running a statement, or fails.
func waitForStatement(t *testing.T, ctx context.Context, conn *db.Conn, contains string) uint64 {
	t.Helper()

	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		listing, err := List(ctx, conn)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		for _, p := range listing.Processes {
			if p.Working() && strings.Contains(p.SQL, contains) && !conn.Owns(p.ID) {
				return p.ID
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no connection is running a statement containing %q", contains)
	return 0
}
