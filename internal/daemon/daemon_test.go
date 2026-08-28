package daemon_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// echoSession is a stand-in for the interface: it writes each rune it is sent
// along the top row. It exists so the whole client/server exchange can be
// tested without a database, which the real interface needs.
type echoSession struct {
	scr  tcell.Screen
	done chan struct{}
}

func newEchoSession() *echoSession { return &echoSession{done: make(chan struct{})} }

func (e *echoSession) SetScreen(s tcell.Screen) { e.scr = s }
func (e *echoSession) Stop()                    { e.scr.Fini() }

func (e *echoSession) Run() error {
	defer close(e.done)
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

func testCaps() screen.Caps {
	return screen.Caps{
		Width: 20, Height: 4,
		Colors: 256, CharacterSet: "UTF-8", HasMouse: true,
	}
}

func hello(version string) proto.ToServer {
	return proto.ToServer{
		Kind: proto.KindHello,
		Hello: &proto.Hello{
			Version: version,
			Caps:    testCaps(),
			PID:     1234,
		},
	}
}

// A client and a server that disagree about the protocol have no safe way to
// find out which parts they share, so the server says so instead of guessing.
func TestVersionMismatchIsRefused(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			t.Error("the session was built for a client that should have been refused")
			return newEchoSession(), nil, nil
		},
	})
	go srv.Serve(server)

	if err := proto.NewEncoder(client).ToServer(hello("0.6.3")); err != nil {
		t.Fatalf("hello: %v", err)
	}

	got, err := proto.NewDecoder(client).ToClient()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Kind != proto.KindReject {
		t.Fatalf("Kind = %v, want KindReject", got.Kind)
	}
	if got.Reject == nil || got.Reject.Reason == "" {
		t.Error("the refusal came with no reason for the user to read")
	}
}

// dv server stop must end the session even from a connection that never
// sends a Hello — a stop request is not a client attaching, and treating it
// as one would mean it can only reach a session that is already free to
// accept a new client, which is exactly the case a stuck session needs it
// to work for.
func TestStopEndsTheSessionWithoutAHello(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			t.Error("a stop request waited for a Hello before it was honoured")
			return newEchoSession(), nil, nil
		},
	})
	go srv.Serve(server)

	if err := proto.NewEncoder(client).ToServer(proto.ToServer{Kind: proto.KindStop}); err != nil {
		t.Fatalf("stop: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- srv.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait() = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait never returned after a stop request")
	}
}

// Whatever the server could not set up while starting the session would
// otherwise land in a log nobody reads, and completion would simply look
// broken.
func TestWarningsTravelInTheWelcome(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return newEchoSession(), []string{"completion disabled: read-only state directory"}, nil
		},
	})
	go srv.Serve(server)
	defer srv.Stop()

	enc, dec := proto.NewEncoder(client), proto.NewDecoder(client)
	if err := enc.ToServer(hello("0.7.0")); err != nil {
		t.Fatalf("hello: %v", err)
	}

	got, err := dec.ToClient()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Kind != proto.KindWelcome || got.Welcome == nil {
		t.Fatalf("got %v, want a welcome", got.Kind)
	}
	if len(got.Welcome.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one", got.Welcome.Warnings)
	}
}

// A key that does not reach the session is a key the user pressed for
// nothing, and a frame that does not come back is a terminal showing the
// past.
func TestKeyGoesUpAndTheFrameComesBack(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return newEchoSession(), nil, nil
		},
	})
	go srv.Serve(server)
	defer srv.Stop()

	enc, dec := proto.NewEncoder(client), proto.NewDecoder(client)
	if err := enc.ToServer(hello("0.7.0")); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if welcome, err := dec.ToClient(); err != nil || welcome.Kind != proto.KindWelcome {
		t.Fatalf("welcome = %v, err = %v", welcome.Kind, err)
	}

	// The first frame is the whole screen, sent on attach.
	if first, err := dec.ToClient(); err != nil || first.Kind != proto.KindFrame {
		t.Fatalf("first frame = %v, err = %v", first.Kind, err)
	}

	if err := enc.ToServer(proto.ToServer{
		Kind: proto.KindKey,
		Key:  &proto.Key{Key: tcell.KeyRune, Rune: 'z'},
	}); err != nil {
		t.Fatalf("key: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("no frame carrying 'z' arrived within five seconds")
		default:
		}

		m, err := dec.ToClient()
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if m.Kind != proto.KindFrame || m.Frame == nil {
			continue
		}
		for _, c := range m.Frame.Cells {
			if c.X == 0 && c.Y == 0 && c.Main == 'z' {
				return
			}
		}
	}
}

// Losing the terminal is not the same as ending the session. If it were, an
// SSH connection dropping would take the statement with it, which is the
// thing this whole arrangement exists to stop.
func TestClientGoingAwayDoesNotStopTheSession(t *testing.T) {
	client, server := net.Pipe()

	session := newEchoSession()
	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return session, nil, nil
		},
	})
	go srv.Serve(server)
	defer srv.Stop()

	enc, dec := proto.NewEncoder(client), proto.NewDecoder(client)
	if err := enc.ToServer(hello("0.7.0")); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if _, err := dec.ToClient(); err != nil {
		t.Fatalf("welcome: %v", err)
	}

	client.Close()

	select {
	case <-session.done:
		t.Fatal("the session ended when the client went away")
	case <-time.After(500 * time.Millisecond):
	}
}

// A statement in flight is what a second dv must not take the session away
// from, and a session that will not answer is treated as busy: refusing is
// the smaller mistake.
func TestBusySessionRefusesAnotherDataSource(t *testing.T) {
	first, firstServer := net.Pipe()
	defer first.Close()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return &busySession{echoSession: newEchoSession()}, nil, nil
		},
	})
	go srv.Serve(firstServer)
	defer srv.Stop()

	enc := proto.NewEncoder(first)
	h := hello("0.7.0")
	h.Hello.DataSource = "local"
	if err := enc.ToServer(h); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if _, err := proto.NewDecoder(first).ToClient(); err != nil {
		t.Fatalf("welcome: %v", err)
	}

	second, secondServer := net.Pipe()
	defer second.Close()
	go srv.Serve(secondServer)

	h2 := hello("0.7.0")
	h2.Hello.DataSource = "prod"
	if err := proto.NewEncoder(second).ToServer(h2); err != nil {
		t.Fatalf("second hello: %v", err)
	}

	got, err := proto.NewDecoder(second).ToClient()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Kind != proto.KindReject {
		t.Fatalf("Kind = %v, want KindReject: a running statement must not be switched away from", got.Kind)
	}
}

// busySession always reports a statement in flight.
type busySession struct{ *echoSession }

func (b *busySession) State(context.Context) (daemon.State, error) {
	return daemon.State{DataSource: "local", Busy: true}, nil
}

func (b *busySession) SwitchTo(string) {}

// A stalled old client (still connected, but not draining its socket — a
// suspended process, a frozen SSH session) must not brick reconnection: a
// second dv attaching should win promptly, not hang forever behind a write
// nobody is reading.
func TestReplacingAStalledOldClientDoesNotHangTheNewAttach(t *testing.T) {
	oldClient, oldServer := net.Pipe()
	defer oldClient.Close()

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return newEchoSession(), nil, nil
		},
	})
	go srv.Serve(oldServer)
	defer srv.Stop()

	oldEnc, oldDec := proto.NewEncoder(oldClient), proto.NewDecoder(oldClient)
	if err := oldEnc.ToServer(hello("0.7.0")); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if _, err := oldDec.ToClient(); err != nil { // welcome
		t.Fatalf("welcome: %v", err)
	}
	if _, err := oldDec.ToClient(); err != nil { // the attach frame
		t.Fatalf("first frame: %v", err)
	}
	// The old client stops reading from here on, as though its terminal
	// were suspended — it never closes its end either.

	newClient, newServer := net.Pipe()
	defer newClient.Close()
	go srv.Serve(newServer)

	if err := proto.NewEncoder(newClient).ToServer(hello("0.7.0")); err != nil {
		t.Fatalf("hello: %v", err)
	}

	type result struct {
		msg proto.ToClient
		err error
	}
	welcome := make(chan result, 1)
	go func() {
		m, err := proto.NewDecoder(newClient).ToClient()
		welcome <- result{m, err}
	}()

	select {
	case r := <-welcome:
		if r.err != nil {
			t.Fatalf("decode: %v", r.err)
		}
		if r.msg.Kind != proto.KindWelcome {
			t.Fatalf("Kind = %v, want KindWelcome", r.msg.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the new client's attach hung behind a stalled old client's unread Bye")
	}
}

// Two connections racing to attach before any session exists must not both
// start one: whichever loses would open a database connection nothing ever
// closes.
func TestConcurrentFirstAttachStartsOnlyOneSession(t *testing.T) {
	var starts int32

	srv := daemon.New(daemon.Options{
		Version: "0.7.0",
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			atomic.AddInt32(&starts, 1)
			time.Sleep(150 * time.Millisecond)
			return newEchoSession(), nil, nil
		},
	})
	defer srv.Stop()

	clientA, serverA := net.Pipe()
	defer clientA.Close()
	clientB, serverB := net.Pipe()
	defer clientB.Close()

	go srv.Serve(serverA)
	go srv.Serve(serverB)

	var wg sync.WaitGroup
	wg.Add(2)
	for _, c := range []net.Conn{clientA, clientB} {
		go func(c net.Conn) {
			defer wg.Done()
			// Both clients attach as though neither knows the other is
			// there — the race admit must survive, not avoid by luck.
			_ = proto.NewEncoder(c).ToServer(hello("0.7.0"))
			// Whatever admit decided for this client — a welcome, a bye
			// from losing the race, or the connection simply closing —
			// reading it once means this client's admit call has returned.
			_, _ = proto.NewDecoder(c).ToClient()
		}(c)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for both racing attaches to settle")
	}

	if got := atomic.LoadInt32(&starts); got != 1 {
		t.Fatalf("Start was called %d times by two attaches racing to go first, want 1", got)
	}
}
