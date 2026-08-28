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
	"sync"
	"time"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
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

type Options struct {
	// Version is what a client must match exactly.
	Version string

	// Start builds the session the first client asked for, returning it and
	// anything the user should be told about how it was built — a schema
	// cache that would not open, a worktree that no longer exists. Those
	// would otherwise land in a log nobody reads.
	Start func(proto.Hello) (Session, []string, error)
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
	warnings   []string
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

// contextDone is a context that ends when ch closes.
func contextDone(ch <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ch
		cancel()
	}()
	return ctx
}
