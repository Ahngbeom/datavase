package attach_test

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/attach"
	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/gdamore/tcell/v2"
)

// echoSession writes each rune it is sent along the top row, so a test can
// see a keystroke make the whole trip and come back as a cell.
type echoSession struct{ scr tcell.Screen }

func (e *echoSession) SetScreen(s tcell.Screen) { e.scr = s }
func (e *echoSession) Stop()                    { e.scr.Fini() }

func (e *echoSession) Run() error {
	x := 0
	for {
		ev := e.scr.PollEvent()
		if ev == nil {
			return nil
		}
		if k, ok := ev.(*tcell.EventKey); ok && k.Key() == tcell.KeyRune {
			e.scr.SetContent(x, 0, k.Rune(), nil, tcell.StyleDefault)
			x++
			e.scr.Show()
		}
	}
}

// row reads a row off a snapshot taken after the screen was last drawn.
func row(t *testing.T, snap tcell.SimulationScreen, y, width int) string {
	t.Helper()
	cells, w, _ := snap.GetContents()
	var b strings.Builder
	for x := 0; x < width && x < w; x++ {
		runes := cells[y*w+x].Runes
		if len(runes) == 0 {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(runes[0])
	}
	return b.String()
}

// snapshotScreen wraps a SimulationScreen so a test can poll what it shows
// without racing the goroutine drawing it: real GetContents hands back the
// live front buffer by reference, and attach.Run's receive loop keeps
// mutating that same buffer on every frame. Copying it out under a mutex on
// every Show gives the test a read that -race accepts.
type snapshotScreen struct {
	tcell.SimulationScreen

	mu            sync.Mutex
	cells         []tcell.SimCell
	width, height int
}

func (s *snapshotScreen) Show() {
	s.SimulationScreen.Show()
	cells, w, h := s.SimulationScreen.GetContents()
	cp := make([]tcell.SimCell, len(cells))
	copy(cp, cells)

	s.mu.Lock()
	s.cells, s.width, s.height = cp, w, h
	s.mu.Unlock()
}

func (s *snapshotScreen) GetContents() ([]tcell.SimCell, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cells, s.width, s.height
}

// The whole point of the split: what the interface drew on one side is what
// the terminal shows on the other.
func TestWhatTheSessionDrewIsWhatTheTerminalShows(t *testing.T) {
	client, server := net.Pipe()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return &echoSession{}, nil, nil
		},
	})
	go srv.Serve(server)
	defer srv.Stop()

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim init: %v", err)
	}
	sim.SetSize(20, 4)
	snap := &snapshotScreen{SimulationScreen: sim}

	done := make(chan error, 1)
	go func() { done <- attach.Run(client, attach.Options{Version: "0.7.0", Screen: snap}) }()

	sim.InjectKey(tcell.KeyRune, 'h', tcell.ModNone)
	sim.InjectKey(tcell.KeyRune, 'i', tcell.ModNone)

	deadline := time.After(5 * time.Second)
	for {
		if got := row(t, snap, 0, 2); got == "hi" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("top row is %q after five seconds, want \"hi\"", row(t, snap, 0, 2))
		case <-time.After(20 * time.Millisecond):
		}
	}

	client.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("attach.Run did not return after the connection closed")
	}
}

// A refusal the user never sees is a dv that exits saying nothing.
func TestRejectionComesBackAsAnError(t *testing.T) {
	client, server := net.Pipe()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			t.Error("the session was built for a client that should have been refused")
			return &echoSession{}, nil, nil
		},
	})
	go srv.Serve(server)

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim init: %v", err)
	}

	err := attach.Run(client, attach.Options{Version: "0.6.3", Screen: sim})
	if err == nil {
		t.Fatal("attaching to a server of another version returned no error")
	}
	if !strings.Contains(err.Error(), "0.6.3") {
		t.Errorf("the error does not say which versions disagreed: %v", err)
	}
}

// syncBuffer guards a bytes.Buffer so the test goroutine can read stderr
// while attach.Run is still writing to it, without -race reporting a data
// race on the buffer itself.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// A schema cache that would not open costs completion, and the user has to be
// told that in a place they will read.
func TestWarningsReachStderr(t *testing.T) {
	client, server := net.Pipe()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return &echoSession{}, []string{"completion disabled: read-only state directory"}, nil
		},
	})
	go srv.Serve(server)
	defer srv.Stop()

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim init: %v", err)
	}

	var stderr syncBuffer
	done := make(chan error, 1)
	go func() {
		done <- attach.Run(client, attach.Options{Version: "0.7.0", Screen: sim, Err: &stderr})
	}()

	deadline := time.After(5 * time.Second)
	for !strings.Contains(stderr.String(), "completion disabled") {
		select {
		case <-deadline:
			t.Fatalf("stderr is %q after five seconds", stderr.String())
		case <-time.After(20 * time.Millisecond):
		}
	}

	client.Close()
	<-done
}
