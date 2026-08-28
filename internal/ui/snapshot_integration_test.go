//go:build integration

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// Reading a.running or the connection from another goroutine is a race. This
// is the check that the answer comes from the interface's own goroutine and
// arrives whole.
func TestSnapshotReportsTheLiveSession(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	got, err := h.app.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if got.DataSource.Name == "" {
		t.Error("the snapshot does not name the datasource")
	}
	if got.DataSource.ServerVersion == "" {
		t.Error("the snapshot does not carry the server version")
	}
	if got.Statement.Running {
		t.Error("a fresh session reports a statement running")
	}
}

// A caller deciding whether to wait must be able to stop waiting.
func TestSnapshotHonoursItsDeadline(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	if _, err := h.app.Snapshot(ctx); err == nil {
		t.Error("Snapshot ignored an expired context")
	}
}

// The elapsed time of a statement in flight is the number dv status prints in
// its headline, and status.elapsed only advances as row batches land. A
// statement that has produced no rows yet — a long ALTER, an Exec, a SELECT
// before its first chunk — must still be timed from when it was sent, not
// reported as however long the last one took.
func TestSnapshotTimesTheStatementThatIsStillRunning(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT SLEEP(30)")

	h.do(keymap.ActionRun)
	h.waitFor("the statement to start", func(a *App) bool { return a.running != nil })

	// SLEEP hands back its single row only at the end, so nothing will have
	// touched status.elapsed by the time this asks.
	const ran = 600 * time.Millisecond
	time.Sleep(ran)

	got, err := h.app.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	defer h.do(keymap.ActionCopyOrCancel)

	if !got.Statement.Running {
		t.Fatal("the snapshot does not report the statement as running")
	}
	// Half of the wait, so a loaded machine cannot fail this for being slow.
	if want := (ran / 2).Milliseconds(); got.Statement.ElapsedMS < want {
		t.Errorf("elapsed_ms = %d after %v of running, want at least %d",
			got.Statement.ElapsedMS, ran, want)
	}
	if got.Statement.ElapsedMS > 30_000 {
		t.Errorf("elapsed_ms = %d, longer than the statement has existed", got.Statement.ElapsedMS)
	}
}
