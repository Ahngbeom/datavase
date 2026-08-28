//go:build integration

package daemon_test

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/attach"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/Ahngbeom/datavase/internal/ui"
	"github.com/gdamore/tcell/v2"
)

const testVersion = "test"

// sessionAdapter makes *ui.App satisfy daemon.Stateful, the same conversion
// cmd/dv's buildSession does in production. Without it session.(Stateful)
// fails silently at runtime and every session looks busy regardless of
// whether a statement is running — which would make
// TestAnotherDataSourceIsRefusedWhileAStatementRuns pass for the wrong
// reason: refusing because Stateful is broken, not because the daemon read a
// real statement in flight.
type sessionAdapter struct{ *ui.App }

func (a sessionAdapter) State(ctx context.Context) (daemon.State, error) {
	s, err := a.App.State(ctx)
	return daemon.State{DataSource: s.DataSource, Busy: s.Busy}, err
}

// realSession opens the integration datasource and builds the interface on
// it, the way the server does when the first client arrives.
func realSession(t *testing.T, cfg *config.Config) daemon.Session {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	sess, err := session.Open(context.Background(), ds, password)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	keys, err := keymap.FromConfig("datagrip", nil)
	if err != nil {
		t.Fatalf("keymap: %v", err)
	}
	return sessionAdapter{ui.New(sess, cfg, ui.Deps{Keys: keys})}
}

func integrationConfig(t *testing.T) *config.Config {
	t.Helper()
	ds, _ := testmysql.DataSource(t)
	return &config.Config{DataSources: []config.DataSource{*ds}}
}

// snapshotScreen wraps a SimulationScreen so a test can read what it shows
// without racing the goroutine drawing it: real GetContents hands back the
// live front buffer by reference, and attach.Run's receive loop keeps
// mutating that same buffer on every frame. Copying it out under a mutex on
// every Show gives the test a read that -race accepts, and one that can never
// see half of one frame and half of the next.
//
// internal/attach's own tests carry the same wrapper. It cannot be shared:
// the two live in different test packages, and an unexported helper does not
// cross that boundary.
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

// screenText joins a simulation screen into one string, so a test can ask
// whether something is on it without caring where.
func screenText(sim tcell.SimulationScreen) string {
	cells, w, h := sim.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			runes := cells[y*w+x].Runes
			if len(runes) == 0 {
				b.WriteRune(' ')
				continue
			}
			b.WriteRune(runes[0])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func typeInto(sim tcell.SimulationScreen, s string) {
	for _, r := range s {
		sim.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
}

func press(t *testing.T, sim tcell.SimulationScreen, action keymap.Action) {
	t.Helper()
	m, err := keymap.FromConfig("datagrip", nil)
	if err != nil {
		t.Fatalf("keymap: %v", err)
	}
	bindings := m.Bindings(action)
	if len(bindings) == 0 {
		t.Fatalf("%s has no binding", action)
	}
	b := bindings[0]
	sim.InjectKey(b.Key, b.Rune, b.Mods)
}

func attachTo(t *testing.T, srv *daemon.Server, w, h int) (tcell.SimulationScreen, func()) {
	t.Helper()

	client, server := net.Pipe()
	go srv.Serve(server)

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim init: %v", err)
	}
	sim.SetSize(w, h)
	snap := &snapshotScreen{SimulationScreen: sim}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = attach.Run(client, attach.Options{Version: testVersion, Screen: snap})
	}()

	return snap, func() {
		client.Close()
		<-done
	}
}

func waitForScreen(t *testing.T, sim tcell.SimulationScreen, want string, within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		if strings.Contains(screenText(sim), want) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("%q never appeared. screen was:\n%s", want, screenText(sim))
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// The reason for all of this: a statement that is streaming does not care
// that the terminal went away.
func TestAStatementSurvivesTheTerminal(t *testing.T) {
	cfg := integrationConfig(t)
	srv := daemon.New(daemon.Options{
		Version: testVersion,
		Start: func(context.Context, proto.Hello) (daemon.Session, []string, error) {
			return realSession(t, cfg), nil, nil
		},
	})
	t.Cleanup(srv.Stop)

	sim, leave := attachTo(t, srv, 100, 30)
	typeInto(sim, "SELECT SLEEP(3), 6*7 AS answer")
	press(t, sim, keymap.ActionRun)

	// Go away while it is still running.
	time.Sleep(500 * time.Millisecond)
	leave()

	// Come back after it must have finished.
	time.Sleep(4 * time.Second)
	again, leaveAgain := attachTo(t, srv, 100, 30)
	defer leaveAgain()

	waitForScreen(t, again, "42", 10*time.Second)
}

// Switching datasource closes the session it leaves, and closing the session
// kills the statement this whole arrangement exists to keep.
func TestAnotherDataSourceIsRefusedWhileAStatementRuns(t *testing.T) {
	cfg := integrationConfig(t)
	// A second name for the same server, so the test has somewhere to ask to
	// go without needing a second container.
	other := cfg.DataSources[0]
	other.Name = "elsewhere"
	cfg.DataSources = append(cfg.DataSources, other)

	srv := daemon.New(daemon.Options{
		Version: testVersion,
		Start: func(context.Context, proto.Hello) (daemon.Session, []string, error) {
			return realSession(t, cfg), nil, nil
		},
	})
	t.Cleanup(srv.Stop)

	first, firstServer := net.Pipe()
	defer first.Close()
	go srv.Serve(firstServer)

	enc, dec := proto.NewEncoder(first), proto.NewDecoder(first)
	h := proto.ToServer{Kind: proto.KindHello, Hello: &proto.Hello{
		Version:    testVersion,
		Caps:       screenCaps(100, 30),
		DataSource: cfg.DataSources[0].Name,
	}}
	if err := enc.ToServer(h); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if welcome, err := dec.ToClient(); err != nil || welcome.Kind != proto.KindWelcome {
		t.Fatalf("welcome = %v, err = %v", welcome.Kind, err)
	}

	// Start something long, through the same keys a person would use.
	for _, r := range "SELECT SLEEP(5)" {
		_ = enc.ToServer(proto.ToServer{Kind: proto.KindKey, Key: &proto.Key{Key: tcell.KeyRune, Rune: r}})
	}
	m, err := keymap.FromConfig("datagrip", nil)
	if err != nil {
		t.Fatalf("keymap: %v", err)
	}
	run := m.Bindings(keymap.ActionRun)[0]
	_ = enc.ToServer(proto.ToServer{Kind: proto.KindKey, Key: &proto.Key{Key: run.Key, Rune: run.Rune, Mods: run.Mods}})

	time.Sleep(time.Second)

	second, secondServer := net.Pipe()
	defer second.Close()
	go srv.Serve(secondServer)

	h2 := h
	hello2 := *h.Hello
	hello2.DataSource = "elsewhere"
	h2.Hello = &hello2
	if err := proto.NewEncoder(second).ToServer(h2); err != nil {
		t.Fatalf("second hello: %v", err)
	}

	got, err := proto.NewDecoder(second).ToClient()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Kind != proto.KindReject {
		t.Fatalf("Kind = %v, want KindReject", got.Kind)
	}
	if !strings.Contains(got.Reject.Reason, "dv server stop") {
		t.Errorf("the refusal does not say what to do instead: %q", got.Reject.Reason)
	}
}
