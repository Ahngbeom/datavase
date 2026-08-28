package daemon_test

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ahngbeom/datavase/internal/daemon"
)

// shortTempDir is t.TempDir() without the test's own name folded into the
// path: sockaddr_un caps a unix socket path around 104 bytes, and a long
// subtest name plus t.TempDir()'s directory is enough to blow that budget on
// macOS's default TMPDIR, which fails the bind for a reason that has nothing
// to do with what these tests are checking.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dvsock")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// The socket belongs beside the schema cache and the query history, which are
// the other things that are runtime state rather than configuration.
func TestSocketPathFollowsTheStateDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	got, err := daemon.SocketPath()
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	if want := filepath.Join(dir, "datavase", "dv.sock"); got != want {
		t.Errorf("SocketPath() = %q, want %q", got, want)
	}
}

// A crash leaves a socket file behind. Treating the file as proof of a
// running server would mean dv never starts again until someone deletes it by
// hand.
func TestAStaleSocketFileIsReplaced(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "dv.sock")

	// A socket file with nothing behind it: listen, then close the listener
	// without removing the file.
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	ln.Close()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	got, err := daemon.Listen(path)
	if err != nil {
		t.Fatalf("Listen over a stale socket: %v", err)
	}
	defer got.Close()
}

// Two servers on one socket would each hold a session and neither would know
// about the other.
func TestListenRefusesWhenAServerAnswers(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "dv.sock")

	first, err := daemon.Listen(path)
	if err != nil {
		t.Fatalf("first Listen: %v", err)
	}
	defer first.Close()
	go func() {
		for {
			c, err := first.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	if _, err := daemon.Listen(path); !errors.Is(err, daemon.ErrAlreadyRunning) {
		t.Errorf("second Listen returned %v, want ErrAlreadyRunning", err)
	}
}

// The socket is a way into a session that can read production databases.
func TestSocketIsPrivateToItsOwner(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "dv.sock")

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
		t.Errorf("mode = %04o, want 0600; the umask decides what a socket is created with", perm)
	}
}
