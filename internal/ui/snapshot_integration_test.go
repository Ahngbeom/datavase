//go:build integration

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
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
