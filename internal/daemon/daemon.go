// Package daemon holds a datavase session for a terminal that comes and goes.
//
// It does not know what it is serving. A Session is three methods — set a
// screen, run, stop — which is all a terminal interface needs to expose and
// deliberately less than tview or internal/ui. That keeps this package
// testable against a stand-in, and keeps the interface free of any notion
// that it might be running headless.
//
// It does not know about sockets either: Serve takes an
// io.ReadWriteCloser, so net.Pipe exercises the whole exchange.
package daemon

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/Ahngbeom/datavase/internal/snapshot"
	"github.com/gdamore/tcell/v2"
)

// Session is what is being held. *ui.App satisfies it as it stands.
type Session interface {
	SetScreen(tcell.Screen)
	Run() error
	Stop()
}

// State is what a second client needs to know before it may take the session
// somewhere else.
type State struct {
	DataSource string
	Busy       bool
}

// Stateful is a Session that can answer from another goroutine. A Session
// that does not implement it, or that does not answer in time, is treated as
// busy: refusing is the smaller mistake.
type Stateful interface {
	State(context.Context) (State, error)
}

// Switcher is a Session that can be moved to another datasource.
type Switcher interface {
	SwitchTo(name string)
}

// stateTimeout bounds the wait for a Stateful answer during a handshake. A
// client is sitting at a prompt for this long, so it is short, and expiring
// means refusing rather than assuming.
const stateTimeout = 2 * time.Second

// startTimeout bounds building the session, which opens a real database
// connection and may raise an SSH tunnel first. It matches what dv gives a
// connection attempt everywhere else, so a host behind a dropped VPN fails
// here the same way it fails when dv runs on its own — and, because admit
// holds admitMu across Start, an unbounded wait here would leave every later
// attach queued behind it for the life of the process.
const startTimeout = 15 * time.Second

// byeTimeout bounds the goodbye sent when the session ends. The message is a
// courtesy — a client also learns of the end from the socket closing — so a
// terminal that has stopped reading costs a moment at shutdown rather than a
// server process that never exits.
const byeTimeout = time.Second

type Options struct {
	// Version is what a client must match exactly.
	Version string

	// BuildFingerprint is version.BuildFingerprint(): empty outside a
	// development build. It is compared only when both this and the client's
	// Hello.BuildFingerprint are non-empty, so a release server — which never
	// sets this — is never refused over it.
	BuildFingerprint string

	// Start builds the session the first client asked for, returning it and
	// anything the user should be told about how it was built — a schema
	// cache that would not open, a worktree that no longer exists. Those
	// would otherwise land in a log nobody reads.
	//
	// The context bounds building only, not the session that comes out of
	// it: whatever Start opens must outlive the call and is owned by the
	// returned Session from then on.
	Start func(context.Context, proto.Hello) (Session, []string, error)
}

// Server holds one session and at most one client.
type Server struct {
	opts Options

	// admitMu serialises the whole of admit: the "does a session exist, and
	// if not, start one" decision. It is separate from mu, which guards
	// cheap state reads used elsewhere in the package (Detach, Stop) and
	// must stay uncontended by something as slow as Start, which opens a
	// real database connection.
	admitMu sync.Mutex

	mu         sync.Mutex
	session    Session
	screen     *screen.Screen
	dataSource string
	client     *conn

	runOnce sync.Once
	runErr  error
	ended   chan struct{}
}

func New(opts Options) *Server {
	return &Server{opts: opts, ended: make(chan struct{})}
}

// Wait blocks until the session ends and returns why.
func (s *Server) Wait() error {
	<-s.ended
	return s.runErr
}

// Detach drops the current client and keeps the session.
func (s *Server) Detach() {
	s.mu.Lock()
	c := s.client
	s.client = nil
	scr := s.screen
	s.mu.Unlock()

	if scr != nil {
		scr.Detach()
	}
	if c != nil {
		c.close()
	}
}

// Stop ends the session. Wait returns once it has.
func (s *Server) Stop() {
	s.mu.Lock()
	session := s.session
	s.mu.Unlock()

	if session != nil {
		session.Stop()
		return
	}
	// Nothing was ever started, so there is nothing to unwind.
	s.finish(nil)
}

// farewell tells the attached terminal that the session it was showing has
// ended, so it exits on a message that says why rather than on a socket that
// simply stopped — which is indistinguishable, from the client's side, from
// the server having died.
func (s *Server) farewell() {
	s.mu.Lock()
	c := s.client
	s.mu.Unlock()
	if c == nil {
		return
	}

	// send is a write with no deadline, and this goroutine is the one that
	// must reach finish for the process to exit at all. Bound it: a client
	// that has stopped draining its socket cannot be allowed to hold the
	// session open past its own end.
	sent := make(chan struct{})
	go func() {
		c.send(proto.ToClient{Kind: proto.KindBye, Bye: &proto.Bye{Reason: proto.ByeQuit}})
		close(sent)
	}()
	select {
	case <-sent:
	case <-time.After(byeTimeout):
	}
}

func (s *Server) finish(err error) {
	s.runOnce.Do(func() {
		s.runErr = err
		close(s.ended)
	})
}

// state asks the session where it is, treating silence as busy.
func (s *Server) state(session Session, known string) State {
	st, ok := session.(Stateful)
	if !ok {
		return State{DataSource: known, Busy: true}
	}

	ctx, cancel := context.WithTimeout(context.Background(), stateTimeout)
	defer cancel()

	got, err := st.State(ctx)
	if err != nil {
		return State{DataSource: known, Busy: true}
	}
	return got
}

// Info is the tier of a snapshot that comes from this process. It cannot
// fail, which is what lets dv status answer whatever the session is doing.
func (s *Server) Info(dv string, started, now time.Time) snapshot.Server {
	s.mu.Lock()
	attached := s.client != nil
	s.mu.Unlock()

	return snapshot.Server{
		PID:            os.Getpid(),
		StartedAt:      started.UTC().Format(time.RFC3339),
		UptimeSeconds:  int(now.Sub(started).Seconds()),
		ClientAttached: attached,
		DV:             dv,
	}
}

// contextDone is a context that ends when ch closes.
func contextDone(ch <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ch
		cancel()
	}()
	return ctx
}
