package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/Ahngbeom/datavase/internal/snapshot"
	"github.com/gdamore/tcell/v2"
)

// fakeSession is a minimal statefulSession stand-in: enough to run
// closingSession to completion without a real interface or database behind
// it.
type fakeSession struct{ err error }

func (f *fakeSession) SetScreen(tcell.Screen)                      {}
func (f *fakeSession) Stop()                                       {}
func (f *fakeSession) Run() error                                  { return f.err }
func (f *fakeSession) SwitchTo(string)                             {}
func (f *fakeSession) State(context.Context) (daemon.State, error) { return daemon.State{}, nil }

// fakeCloser records whether Close was called, standing in for the
// *catalog.Cache and *history.Store handles buildSession actually opens.
type fakeCloser struct{ closed bool }

func (c *fakeCloser) Close() error { c.closed = true; return nil }

// The cache and history handles buildSession opens must not outlive the
// session that used them: a daemon-hosted dv server has no defer to close
// them the way openUI does for the monolithic path, only the moment Run
// returns.
func TestClosingSessionClosesHandlesWhenRunReturns(t *testing.T) {
	cache, hist := &fakeCloser{}, &fakeCloser{}
	s := closingSession{
		statefulSession: &fakeSession{},
		closers:         []io.Closer{cache, hist},
	}

	if err := s.Run(); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if !cache.closed {
		t.Error("the cache handle was never closed")
	}
	if !hist.closed {
		t.Error("the history handle was never closed")
	}
}

// wedgedSession simulates a session whose own goroutine has stopped
// responding to anything: Stop returns, but nothing makes Run return, which
// is what leaves the server process — and the socket dv server stop writes
// to — up long after the stop request was sent.
type wedgedSession struct{ unblock chan struct{} }

func (w *wedgedSession) SetScreen(tcell.Screen) {}
func (w *wedgedSession) Stop()                  {}
func (w *wedgedSession) Run() error             { <-w.unblock; return nil }

// cooperativeSession is what Stop is supposed to produce: Run returns as
// soon as it is asked to, the way the graceful stop path expects.
type cooperativeSession struct{ done chan struct{} }

func (c *cooperativeSession) SetScreen(tcell.Screen) {}
func (c *cooperativeSession) Stop()                  { close(c.done) }
func (c *cooperativeSession) Run() error             { <-c.done; return nil }

// shortStateDir leaves room under a unix socket's ~104-byte sun_path limit
// for "/datavase/dv-api.sock": t.TempDir() folds the test's own name into the
// path, which is often enough on its own to blow that budget under macOS's
// default TMPDIR.
func shortStateDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dvcmd")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// startTestServer wires a real dv.sock and dv-api.sock the way runServer
// does, with sess as the one session a first client's Hello admits. The
// listeners come down when the session ends, in the order runServer's own
// defers give a real one — without that, the graceful stop path would be
// polling a socket that a real server would already have removed.
func startTestServer(t *testing.T, sess daemon.Session) (sockPath string) {
	t.Helper()

	t.Setenv("XDG_STATE_HOME", shortStateDir(t))

	sockPath, err := daemon.SocketPath()
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	ln, err := daemon.Listen(sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	apiPath, err := daemon.APISocketPath()
	if err != nil {
		t.Fatalf("APISocketPath: %v", err)
	}
	apiLn, err := daemon.Listen(apiPath)
	if err != nil {
		t.Fatalf("Listen (api): %v", err)
	}

	srv := daemon.New(daemon.Options{
		Version: "test",
		Start: func(context.Context, proto.Hello) (daemon.Session, []string, error) {
			return sess, nil, nil
		},
	})

	started := time.Now()
	go daemon.ServeAPI(apiLn, snapshot.Source{
		Server:  func() snapshot.Server { return srv.Info("test", started, time.Now()) },
		Session: func(context.Context) (*snapshot.Session, error) { return nil, nil },
	})
	go srv.Accept(ln)
	go func() {
		srv.Wait()
		ln.Close()
		os.Remove(sockPath)
		apiLn.Close()
		os.Remove(apiPath)
	}()

	admitOneClient(t, sockPath)
	return sockPath
}

// admitOneClient sends the Hello that starts the session, then disconnects —
// the way a client that has already left looks to a later `dv server stop`
// run as a separate invocation.
func admitOneClient(t *testing.T, sockPath string) {
	t.Helper()

	conn, err := daemon.Dial(sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	err = proto.NewEncoder(conn).ToServer(proto.ToServer{
		Kind: proto.KindHello,
		Hello: &proto.Hello{
			Version: "test",
			Caps:    screen.Caps{Width: 80, Height: 24, Colors: 256, CharacterSet: "UTF-8"},
		},
	})
	if err != nil {
		t.Fatalf("hello: %v", err)
	}
	got, err := proto.NewDecoder(conn).ToClient()
	if err != nil {
		t.Fatalf("welcome: %v", err)
	}
	if got.Kind != proto.KindWelcome {
		t.Fatalf("Kind = %v, want KindWelcome", got.Kind)
	}
}

// The ordinary case: a session that answers Stop must actually end, and
// `dv server stop` must report that rather than the deadline this test would
// otherwise have to wait out.
func TestStopEndsASessionThatAnswers(t *testing.T) {
	startTestServer(t, &cooperativeSession{done: make(chan struct{})})

	if err := stopServer(false); err != nil {
		t.Fatalf("stopServer(false) = %v, want nil", err)
	}
}

// This is the defect: dv.sock is served by the same loop the session runs
// in, so a wedged session leaves the stop write with nothing reading it back
// and dv server stop returning success while the process stays up. The fix
// is a deadline that turns silence into a reported pid rather than a hang.
func TestStopNamesThePIDWhenTheSessionWontEnd(t *testing.T) {
	orig := stopDeadline
	stopDeadline = 300 * time.Millisecond
	t.Cleanup(func() { stopDeadline = orig })

	unblock := make(chan struct{})
	t.Cleanup(func() { close(unblock) })
	startTestServer(t, &wedgedSession{unblock: unblock})

	err := stopServer(false)
	if err == nil {
		t.Fatal("stopServer(false) = nil, want an error naming the pid to reach with --force")
	}

	wantPID := fmt.Sprintf("pid %d", os.Getpid())
	if !strings.Contains(err.Error(), wantPID) {
		t.Errorf("error = %q, want it to name %s", err, wantPID)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q, want it to point at --force", err)
	}
}

// --force must reach the pid the observation API actually reports rather
// than one dv works out on its own — that API is the only thing that can see
// the server process from outside a session that may itself be unable to
// answer.
func TestForceSignalsThePIDFromTheObservationSnapshot(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", shortStateDir(t))

	// A disposable process receives the real SIGTERM --force sends, so the
	// test exercises the actual signal rather than a stand-in for it — this
	// test's own process must never be the target.
	victim := exec.Command("sleep", "30")
	if err := victim.Start(); err != nil {
		t.Fatalf("starting the victim process: %v", err)
	}
	t.Cleanup(func() { _ = victim.Process.Kill() })

	apiPath, err := daemon.APISocketPath()
	if err != nil {
		t.Fatalf("APISocketPath: %v", err)
	}
	apiLn, err := daemon.Listen(apiPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { apiLn.Close() })

	go daemon.ServeAPI(apiLn, snapshot.Source{
		Server:  func() snapshot.Server { return snapshot.Server{PID: victim.Process.Pid} },
		Session: func(context.Context) (*snapshot.Session, error) { return nil, nil },
	})

	if err := stopServer(true); err != nil {
		t.Fatalf("stopServer(true) = %v, want nil", err)
	}

	done := make(chan error, 1)
	go func() { done <- victim.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the victim process was never signalled")
	}
}
