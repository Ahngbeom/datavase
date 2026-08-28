package daemon_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/snapshot"
)

// Reading what a session is doing must not need a turn: a status line, a
// script and a person can all be looking at once.
func TestSeveralReadersAtOnce(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "dv-api.sock")
	ln, err := daemon.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	src := snapshot.Source{
		Server:  func() snapshot.Server { return snapshot.Server{PID: 7, DV: "test"} },
		Session: func(context.Context) (*snapshot.Session, error) { return nil, nil },
	}
	go daemon.ServeAPI(ln, src)

	for i := 0; i < 3; i++ {
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("reader %d: %v", i, err)
		}

		var got snapshot.Snapshot
		if err := json.NewDecoder(conn).Decode(&got); err != nil {
			t.Fatalf("reader %d decode: %v", i, err)
		}
		conn.Close()

		if got.Server.PID != 7 {
			t.Errorf("reader %d got PID %d, want 7", i, got.Server.PID)
		}
	}
}

// The API socket is a way to read what a production session is doing.
func TestAPISocketIsPrivateToItsOwner(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "dv-api.sock")
	ln, err := daemon.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %04o, want 0600", perm)
	}
}
