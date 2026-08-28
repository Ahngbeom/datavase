# Runtime Split, Part 2: The Runtime — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `dv` survive its terminal. A headless server holds the session;
a thin client attaches, detaches and re-attaches; a statement that is
streaming keeps streaming with nobody watching.

**Architecture:** `internal/daemon` runs a session against the
`internal/screen` proxy from Part 1 and serves one client over an
`io.ReadWriteCloser`. `internal/attach` owns the real terminal and does the
reverse. Neither knows about `ui`: the server drives three methods —
`SetScreen`, `Run`, `Stop` — that `*ui.App` already has, which is also what
lets the round-trip tests run without a database. `cmd/dv` wires them,
spawning a server when there is none and running monolithically when it
cannot.

**Tech Stack:** Go 1.26.4, `internal/screen` and `internal/proto` from Part 1,
`net` and `os/exec` from the standard library, `github.com/gdamore/tcell/v2`
for the client's real screen. No new third-party dependency.

**Spec:** `docs/superpowers/specs/2026-08-27-dv-runtime-design.md`

**Depends on:** `docs/superpowers/plans/2026-08-27-runtime-split-part1-screen-and-wire.md`, complete and merged.

## Global Constraints

- Repository `Ahngbeom/datavase`, default branch `main`. Work on a branch and
  open a PR; never push to `main`.
- `CGO_ENABLED=0` must stay viable. This plan adds no third-party dependency.
- **tcell is Apache-2.0; datavase is MIT. Do not copy tcell source.**
- `make lint` (`go vet ./...` and `gofmt -l .`) clean at every commit.
- `make test` passes at every commit. Only Task 7 needs a database, and it is
  behind `//go:build integration`.
- Every new package gets a doc comment stating what it deliberately does not
  know.
- **Comments say why, not what.** **Test names state the user-visible
  consequence.** **TDD**: failing test first, failing for the right reason;
  where a test could pass for the wrong reason, mutate the implementation and
  watch it fail.
- Nothing exists in production code for a test's sake.
- The socket lives at `$XDG_STATE_HOME/datavase/dv.sock`, mode `0600`.
  Existence is never the test for a live server — connecting is.
- **The server is an optional convenience of the same grade as the schema
  cache, the query history and the first-run marker.** Anything that stops it
  starting costs persistence and prints one line to stderr; it never costs the
  session.

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/keymap/action.go` | `ActionDetach`: the constant, both name maps, the help order. |
| `internal/keymap/binding.go` | `Ctrl+\` folded back to a rune, as `Ctrl+/` already is. |
| `internal/keymap/map.go` | The default binding. |
| `internal/ui/app.go` | `Deps.Detach`, the dispatch case, `State`, `SwitchTo`. |
| `internal/ui/dialog.go` | The `Other` help group. |
| `internal/ui/palette.go` | One palette entry. |
| `internal/daemon/daemon.go` | `Session`, `Stateful`, `Server`: the session's life. |
| `internal/daemon/serve.go` | One client: handshake, frames out, events in. |
| `internal/daemon/socket.go` | Socket path, stale detection, permissions, listener. |
| `internal/attach/attach.go` | The client: real screen, events up, frames down. |
| `internal/cli/cli.go` | `dv server`, `dv server stop`, `dv status`, `--no-session` in the usage. |
| `cmd/dv/main.go` | Spawn, attach, or run monolithically. |

---

### Task 1: The interface learns to leave without ending

`ActionDetach` exists, is bound, is discoverable, and calls a dependency that
may be absent. Nothing yet supplies that dependency, so this task changes what
`dv` can do and not what it does.

**Files:**
- Modify: `internal/keymap/action.go`, `internal/keymap/binding.go`, `internal/keymap/map.go`
- Modify: `internal/ui/app.go`, `internal/ui/dialog.go`, `internal/ui/palette.go`
- Test: `internal/ui/dialog_test.go` and `internal/ui/palette_test.go` already
  contain the checks that must fail first. Add
  `internal/keymap/detach_test.go`.

**Interfaces:**
- Consumes: nothing from Part 1.
- Produces:
  - `keymap.ActionDetach`, config name `"detach"`
  - `ui.Deps.Detach func()` — nil means this session has no server to leave
  - `func (a *App) State(ctx context.Context) (RuntimeState, error)` where
    `type RuntimeState struct { DataSource string; Busy bool }`
  - `func (a *App) SwitchTo(name string)`

- [ ] **Step 1: Add the action and watch the existing tests fail**

Edit `internal/keymap/action.go`. In the constant block, the `// Application.`
group becomes:

```go
	// Application.
	ActionHelp
	ActionDetach
	ActionQuit
```

Add to `actionNames`:

```go
	ActionDetach:            "detach",
```

Add to `descriptions`:

```go
	ActionDetach:            "leave the terminal, keeping the session running",
```

And in `order`, replace `ActionHelp, ActionQuit,` with:

```go
	ActionHelp, ActionDetach, ActionQuit,
```

- [ ] **Step 2: Run the tests and watch them fail**

Run: `go test ./internal/ui/ ./internal/keymap/`
Expected: `TestEveryActionAppearsOnTheHelpScreen` FAILS with `detach is
bindable but is not on the help screen`. The palette's coverage invariant
fails too, because the action has no binding at all yet and no palette entry.

These two tests were written before this work and are the reason a new key
cannot arrive undiscoverable. Watch them fail before fixing them.

- [ ] **Step 3: Bind it, and teach the map the control code**

In `internal/keymap/map.go`, beside the other `// Application.` bindings:

```go
	m.bind(ActionDetach,
		append(ctrlAndCmdRune('\\', 0),
			Binding{Key: tcell.KeyF10, Mods: tcell.ModShift})...)
```

`F1` through `F12` are all bound, so the fallback shares `F10` with quit under
`Shift`, the way `ActionRunAll` already shares `F5` with `ActionRun` — the
only other `Shift`+function key in the map. `Shift` is not one of
`chordMods`, so this satisfies the palette test's rule that every action have
a route a terminal cannot steal.

In `internal/keymap/binding.go`, add to `controlCodeRune`:

```go
	// Ctrl+\ arrives as the file separator, 0x1C.
	tcell.KeyCtrlBackslash: '\\',
```

Without this the Ctrl binding is dead: `normalize` folds legacy control codes
back to the letter that produced them, and it only lists the ones datavase
binds. `Ctrl+/` is in there for exactly the same reason.

- [ ] **Step 4: Put it on the help screen and in the palette**

In `internal/ui/dialog.go`, the last help group becomes:

```go
	{
		title:   "Other",
		actions: []keymap.Action{keymap.ActionHelp, keymap.ActionDetach, keymap.ActionQuit},
	},
```

Leave `startHere` alone. It is five entries and about the first ten minutes;
detaching is not a thing anyone needs before they have a session worth
keeping.

In `internal/ui/palette.go`, before the `quit` entry:

```go
		{
			name:     "detach",
			category: catOther,
			summary:  "leave the terminal, keeping the session running",
			covers:   keymap.ActionDetach,
			run:      (*App).detach,
		},
```

- [ ] **Step 5: Write the failing test for what the key does**

Create `internal/keymap/detach_test.go`:

```go
package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Ctrl+\ arrives from a terminal as a control code, not as a rune with a
// modifier. A binding registered under the rune and never folded back is a
// key that does nothing at all.
func TestCtrlBackslashReachesDetach(t *testing.T) {
	m, err := FromConfig("datagrip", nil)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}

	ev := tcell.NewEventKey(tcell.KeyCtrlBackslash, 0, tcell.ModCtrl)
	if got := m.Lookup(ev); got != ActionDetach {
		t.Errorf("Ctrl+\\ resolved to %s, want detach", got)
	}
}

// Detaching and quitting are neighbours on the same key. Binding one over the
// other would make leaving the terminal end the session it was meant to keep.
func TestDetachAndQuitAreDistinct(t *testing.T) {
	m, err := FromConfig("datagrip", nil)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}

	quit := tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone)
	detach := tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModShift)

	if got := m.Lookup(quit); got != ActionQuit {
		t.Errorf("F10 resolved to %s, want quit", got)
	}
	if got := m.Lookup(detach); got != ActionDetach {
		t.Errorf("Shift+F10 resolved to %s, want detach", got)
	}
}
```

- [ ] **Step 6: Run it and watch it fail, then implement the dispatch**

Run: `go test ./internal/keymap/ -run TestCtrl`
Expected: FAIL if `controlCodeRune` was not edited in Step 3. If it passes
immediately, remove the `KeyCtrlBackslash` line, watch it fail, and put it
back.

Now in `internal/ui/app.go`, add to `Deps`:

```go
	// Detach leaves the terminal without ending the session. Nil means there
	// is no server holding one — a monolithic run — and the action says so
	// rather than appearing dead.
	Detach func()
```

Add the matching field to `App` beside `connect`:

```go
	// detach leaves the terminal to whatever is holding the session. Nil in a
	// monolithic run.
	detach func()
```

Assign it in `New` wherever `connect` is assigned, then add the dispatch case
beside quit:

```go
	case keymap.ActionDetach:
		a.detach()
```

That will not compile against a nil field, which is the point. Write the
method instead, next to `quit` in `app.go`:

```go
// detach leaves the terminal and keeps the session.
//
// It asks about nothing. Quitting asks about an open transaction and an
// unsaved buffer because quitting destroys them; detaching destroys nothing,
// and a session left running is exactly what the person pressing this wants.
func (a *App) detach() {
	if a.detachFn == nil {
		a.notice("this session has no server to leave; it was started with --no-session")
		return
	}
	a.detachFn()
}
```

Name the field `detachFn` so it does not collide with the method. Update the
`Deps` assignment and the dispatch case accordingly.

- [ ] **Step 7: Run everything and watch it pass**

Run: `go test ./internal/ui/ ./internal/keymap/`
Expected: PASS, including `TestEveryActionAppearsOnTheHelpScreen` and the
palette coverage invariant that were failing at Step 2.

- [ ] **Step 8: Add what the daemon will need to ask**

Still in `internal/ui/app.go`:

```go
// RuntimeState is what something outside the interface may need to know
// before it changes the session underneath it.
type RuntimeState struct {
	DataSource string
	Busy       bool
}

// State reports the session's datasource and whether a statement is in
// flight.
//
// It goes through the interface's own goroutine because that is who owns
// a.running and the connection; reading them from anywhere else is a race.
// The context bounds the wait: a caller deciding whether it may take the
// session somewhere else must be able to give up and refuse, which is the
// safe answer when the interface is not talking.
func (a *App) State(ctx context.Context) (RuntimeState, error) {
	out := make(chan RuntimeState, 1)

	// The goroutine outlives this call if the interface never runs the
	// update. That is a wedged application, and one leaked goroutine is not
	// the problem worth solving in that situation.
	go a.app.QueueUpdate(func() {
		out <- RuntimeState{
			DataSource: a.conn.DataSource().Name,
			Busy:       a.running != nil,
		}
	})

	select {
	case s := <-out:
		return s, nil
	case <-ctx.Done():
		return RuntimeState{}, ctx.Err()
	}
}

// SwitchTo moves the session to a configured datasource by name.
//
// It hands off to the same switch the keyboard reaches, so an open
// transaction is asked about and a running statement is refused, in front of
// the person who will see the answer. Nothing is reported back: the interface
// is where the outcome belongs.
func (a *App) SwitchTo(name string) {
	go a.app.QueueUpdate(func() {
		for i := range a.cfg.DataSources {
			if a.cfg.DataSources[i].Name == name {
				a.switchTo(&a.cfg.DataSources[i])
				return
			}
		}
		a.notice(fmt.Sprintf("no datasource named %q", name))
	})
}
```

- [ ] **Step 9: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/...
git add internal/keymap/ internal/ui/
git commit -m "Give the interface a way to leave without ending

Detaching and quitting are different intentions and now different keys.
Quit keeps its meaning exactly — it ends the session and everything the
session was holding. Detach leaves the terminal and nothing else, and
asks about nothing on the way out: quitting asks about an open
transaction and an unsaved buffer because quitting destroys them, and
detaching destroys neither.

The dependency is nil for a monolithic run, and the status bar says so
rather than the key appearing dead. That is how switching datasource
already handles a session that cannot.

Every function key was taken, so the fallback shares F10 with quit under
Shift, as run-all already shares F5 with run. Ctrl+\\ needed a line in
controlCodeRune to arrive at all — it comes from a terminal as the file
separator, not as a rune with a modifier, and normalize only folds back
the control codes datavase binds.

State and SwitchTo are here for whatever ends up holding the session.
Both go through the interface's goroutine, because a.running and the
connection belong to it."
```

---

### Task 2: A server that holds the session and serves one client

**Files:**
- Create: `internal/daemon/daemon.go`, `internal/daemon/serve.go`
- Test: `internal/daemon/daemon_test.go`

**Interfaces:**
- Consumes: `screen.New`, `screen.Screen`, `screen.Sink`, `screen.Frame` and
  `proto.*` from Part 1.
- Produces:
  - `type Session interface { SetScreen(tcell.Screen); Run() error; Stop() }`
  - `type State struct { DataSource string; Busy bool }`
  - `type Stateful interface { State(context.Context) (State, error) }`
  - `type Switcher interface { SwitchTo(name string) }`
  - `type Options struct { Version string; Start func(proto.Hello) (Session, []string, error) }`
  - `func New(Options) *Server`
  - `func (s *Server) Serve(conn io.ReadWriteCloser)`
  - `func (s *Server) Wait() error`, `func (s *Server) Detach()`, `func (s *Server) Stop()`

- [ ] **Step 1: Write the failing test**

Create `internal/daemon/daemon_test.go`:

```go
package daemon_test

import (
	"context"
	"net"
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
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/daemon/`
Expected: build failure — the package does not exist.

- [ ] **Step 3: Write the session's life**

Create `internal/daemon/daemon.go`:

```go
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
	"io"
	"sync"

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
```

Add `"time"` to the imports.

- [ ] **Step 4: Write the client exchange**

Create `internal/daemon/serve.go`:

```go
package daemon

import (
	"fmt"
	"io"
	"sync"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// conn is one attached client.
type conn struct {
	rwc io.ReadWriteCloser
	enc *proto.Encoder

	frames *proto.FrameQueue
	msgs   chan proto.ToClient

	// writes serialises the two goroutines that write: frames come off the
	// queue, everything else off msgs, and they are independent of each
	// other.
	writes sync.Mutex

	closeOnce sync.Once
	done      chan struct{}
}

func newConn(rwc io.ReadWriteCloser) *conn {
	return &conn{
		rwc:    rwc,
		enc:    proto.NewEncoder(rwc),
		frames: proto.NewFrameQueue(),
		msgs:   make(chan proto.ToClient, 16),
		done:   make(chan struct{}),
	}
}

func (c *conn) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.frames.Close()
		_ = c.rwc.Close()
	})
}

func (c *conn) send(m proto.ToClient) {
	c.writes.Lock()
	defer c.writes.Unlock()
	if err := c.enc.ToClient(m); err != nil {
		// A write that fails means the terminal is gone. That is a detach,
		// not a shutdown: the reader will see the same thing and unwind.
		c.close()
	}
}

// Sink implementation. Nothing here blocks: the goroutine calling it is the
// one drawing the interface, and it is also the one reading rows off the
// database connection.

func (c *conn) Frame(f screen.Frame) { c.frames.Put(f) }

func (c *conn) post(m proto.ToClient) {
	select {
	case c.msgs <- m:
	case <-c.done:
	default:
		// Sixteen unread titles or bells means the terminal is not keeping
		// up with anything, and these are cosmetic. Frames are what must not
		// be lost, and they are not queued here.
	}
}

func (c *conn) SetTitle(t string)        { c.post(proto.ToClient{Kind: proto.KindTitle, Title: t}) }
func (c *conn) SetClipboard(b []byte)    { c.post(proto.ToClient{Kind: proto.KindSetClipboard, Clipboard: b}) }
func (c *conn) RequestClipboard()        { c.post(proto.ToClient{Kind: proto.KindRequestClipboard}) }
func (c *conn) Bell()                    { c.post(proto.ToClient{Kind: proto.KindBell}) }

var _ screen.Sink = (*conn)(nil)

func (c *conn) writeFrames() {
	for {
		f, ok := c.frames.Take(contextDone(c.done))
		if !ok {
			return
		}
		frame := f
		c.send(proto.ToClient{Kind: proto.KindFrame, Frame: &frame})
	}
}

func (c *conn) writeMessages() {
	for {
		select {
		case m := <-c.msgs:
			c.send(m)
		case <-c.done:
			return
		}
	}
}

// Serve runs one client to completion and returns when it goes away.
func (s *Server) Serve(rwc io.ReadWriteCloser) {
	dec := proto.NewDecoder(rwc)

	first, err := dec.ToServer()
	if err != nil {
		_ = rwc.Close()
		return
	}
	if first.Kind != proto.KindHello || first.Hello == nil {
		reject(rwc, "the first message was not a hello")
		return
	}
	h := *first.Hello

	if h.Version != s.opts.Version {
		reject(rwc, fmt.Sprintf(
			"the running dv server is %s; this dv is %s.\n\n  dv server stop   end that session and start again",
			s.opts.Version, h.Version))
		return
	}

	c, warnings, err := s.admit(h, rwc)
	if err != nil {
		reject(rwc, err.Error())
		return
	}

	c.send(proto.ToClient{
		Kind:    proto.KindWelcome,
		Welcome: &proto.Welcome{Version: s.opts.Version, Warnings: warnings},
	})

	go c.writeFrames()
	go c.writeMessages()

	s.mu.Lock()
	scr := s.screen
	s.mu.Unlock()

	scr.SetSize(h.Caps.Width, h.Caps.Height)
	scr.Attach(c)

	s.read(dec, scr, c)
}

// admit decides whether this client may have the session, starting one if
// there is none.
func (s *Server) admit(h proto.Hello, rwc io.ReadWriteCloser) (*conn, []string, error) {
	s.mu.Lock()

	if s.session == nil {
		start := s.opts.Start
		s.mu.Unlock()

		session, warnings, err := start(h)
		if err != nil {
			return nil, nil, err
		}

		scr := screen.New(h.Caps)
		session.SetScreen(scr)

		s.mu.Lock()
		s.session, s.screen = session, scr
		s.dataSource, s.warnings = h.DataSource, warnings
		c := newConn(rwc)
		s.client = c
		s.mu.Unlock()

		go func() { s.finish(session.Run()) }()
		return c, warnings, nil
	}

	session, known := s.session, s.dataSource
	s.mu.Unlock()

	if h.DataSource != "" && h.DataSource != known {
		st := s.state(session, known)
		if st.Busy {
			return nil, nil, fmt.Errorf(
				"a statement is running on %q.\n\n  dv              attach to it\n  dv server stop  end it and start again",
				st.DataSource)
		}
		if sw, ok := session.(Switcher); ok {
			sw.SwitchTo(h.DataSource)
			s.mu.Lock()
			s.dataSource = h.DataSource
			s.mu.Unlock()
		}
	}

	// The new client wins. The old terminal is told why so it can exit
	// cleanly rather than appear to have frozen.
	s.mu.Lock()
	old := s.client
	c := newConn(rwc)
	s.client = c
	s.mu.Unlock()

	if old != nil {
		old.send(proto.ToClient{Kind: proto.KindBye, Bye: &proto.Bye{Reason: proto.ByeReplaced}})
		old.close()
	}

	// Warnings are once per session, not once per attach.
	return c, nil, nil
}

// read pumps what the client sends into the screen until it stops.
func (s *Server) read(dec *proto.Decoder, scr *screen.Screen, c *conn) {
	defer func() {
		s.mu.Lock()
		mine := s.client == c
		if mine {
			s.client = nil
		}
		s.mu.Unlock()

		if mine {
			scr.Detach()
		}
		c.close()
	}()

	for {
		m, err := dec.ToServer()
		if err != nil {
			return
		}

		switch m.Kind {
		case proto.KindDetach:
			return
		case proto.KindKey:
			if m.Key != nil {
				scr.PostEventWait(tcell.NewEventKey(m.Key.Key, m.Key.Rune, m.Key.Mods))
			}
		case proto.KindMouse:
			if m.Mouse != nil {
				scr.PostEventWait(tcell.NewEventMouse(m.Mouse.X, m.Mouse.Y, m.Mouse.Buttons, m.Mouse.Mods))
			}
		case proto.KindResize:
			if m.Resize != nil {
				scr.SetSize(m.Resize.Width, m.Resize.Height)
				scr.Sync()
			}
		case proto.KindPaste:
			if m.Paste != nil {
				scr.PostEventWait(tcell.NewEventPaste(m.Paste.Start))
			}
		case proto.KindFocus:
			if m.Focus != nil {
				scr.PostEventWait(tcell.NewEventFocus(m.Focus.Focused))
			}
		case proto.KindClipboardData:
			scr.PostEventWait(tcell.NewEventClipboard(m.Clipboard))
		}
	}
}

func reject(rwc io.ReadWriteCloser, reason string) {
	_ = proto.NewEncoder(rwc).ToClient(proto.ToClient{
		Kind:   proto.KindReject,
		Reject: &proto.Reject{Reason: reason},
	})
	_ = rwc.Close()
}
```

`contextDone` turns the connection's done channel into a context, because
`proto.FrameQueue.Take` takes one. Put it in `daemon.go`:

```go
// contextDone is a context that ends when ch closes.
func contextDone(ch <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ch
		cancel()
	}()
	return ctx
}
```

- [ ] **Step 5: Run the tests and watch them pass**

Run: `go test ./internal/daemon/ -v -race`
Expected: all five PASS with no race.

- [ ] **Step 6: Prove the detach-is-not-shutdown test bites**

In `read`'s deferred function, add `s.Stop()` after `c.close()`. Run `go test
./internal/daemon/ -run TestClientGoingAway` and confirm it FAILS with `the
session ended when the client went away`. Remove it. This is the property the
whole design exists for; the test is what keeps a tidy-looking cleanup from
undoing it.

- [ ] **Step 7: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test -race ./internal/...
git add internal/daemon/
git commit -m "Hold the session for a terminal that comes and goes

The server does not know what it is holding. A Session is three methods
— set a screen, run, stop — which *ui.App already has and which tview
and internal/ui are nowhere near. That is what lets the whole exchange
be tested against a stand-in that echoes keystrokes, with no database
anywhere near it.

A read error, a write error, a killed client and a dropped SSH
connection are all the same event here, and that event is a detach. Only
the session ending ends the server, which is the difference the whole
arrangement exists to make.

A second client wins the terminal and the first is told why, so it exits
rather than appearing to freeze. A second client naming another
datasource is refused while a statement is running — and a session that
will not say whether it is running one is treated as though it is, since
refusing is the smaller mistake."
```

---

### Task 3: A client that owns the terminal, and the round trip

**Files:**
- Create: `internal/attach/attach.go`
- Test: `internal/attach/attach_test.go`

**Interfaces:**
- Consumes: `proto.*`, `screen.Style.Tcell` from Part 1; `daemon.*` from Task 2.
- Produces:
  - `type Options struct { Version, WorkDir, DataSource string; Screen tcell.Screen; Err io.Writer }`
  - `func Run(rwc io.ReadWriteCloser, opt Options) error`

- [ ] **Step 1: Write the failing test**

Create `internal/attach/attach_test.go`:

```go
package attach_test

import (
	"bytes"
	"net"
	"strings"
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

// row reads a row off a simulation screen.
func row(t *testing.T, sim tcell.SimulationScreen, y, width int) string {
	t.Helper()
	cells, w, _ := sim.GetContents()
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

	done := make(chan error, 1)
	go func() { done <- attach.Run(client, attach.Options{Version: "0.7.0", Screen: sim}) }()

	sim.InjectKey(tcell.KeyRune, 'h', tcell.ModNone)
	sim.InjectKey(tcell.KeyRune, 'i', tcell.ModNone)

	deadline := time.After(5 * time.Second)
	for {
		if got := row(t, sim, 0, 2); got == "hi" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("top row is %q after five seconds, want \"hi\"", row(t, sim, 0, 2))
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

	var stderr bytes.Buffer
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
```

Note that `stderr` is read from the test goroutine while `attach.Run` writes
to it from another. Run this file with `-race` and, if it reports on the
buffer, wrap it in a mutex-guarded `io.Writer` in the test. The production
code writes to `os.Stderr`, which needs no such guard.

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/attach/`
Expected: build failure — the package does not exist.

- [ ] **Step 3: Write the implementation**

Create `internal/attach/attach.go`:

```go
// Package attach is the half of dv that owns a terminal.
//
// It does not know that dv talks to a database, what a datasource is, or what
// any of the cells it draws mean. It sends what the terminal did and draws
// what comes back, which is what keeps it unaffected by anything the session
// on the other side chooses to do.
package attach

import (
	"errors"
	"fmt"
	"io"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

type Options struct {
	// Version must match the server's exactly.
	Version string

	// WorkDir and DataSource are what this invocation asked for, passed on so
	// the server can build or move the session.
	WorkDir    string
	DataSource string

	// Screen is the terminal. Nil makes a real one, which is the ordinary
	// case; a test supplies a simulation screen and keeps it afterwards.
	Screen tcell.Screen

	// Err is where the server's warnings are printed. Nil discards them.
	Err io.Writer
}

// Run attaches over rwc and returns when the session ends, the client is
// detached, or the server refuses.
func Run(rwc io.ReadWriteCloser, opt Options) error {
	scr := opt.Screen
	owned := false
	if scr == nil {
		made, err := tcell.NewScreen()
		if err != nil {
			return fmt.Errorf("terminal: %w", err)
		}
		if err := made.Init(); err != nil {
			return fmt.Errorf("terminal: %w", err)
		}
		scr, owned = made, true
	}
	if owned {
		defer scr.Fini()
	}

	scr.EnableMouse()
	scr.EnablePaste()

	enc, dec := proto.NewEncoder(rwc), proto.NewDecoder(rwc)

	w, h := scr.Size()
	err := enc.ToServer(proto.ToServer{
		Kind: proto.KindHello,
		Hello: &proto.Hello{
			Version: opt.Version,
			Caps: screen.Caps{
				Width: w, Height: h,
				Colors:       scr.Colors(),
				CharacterSet: scr.CharacterSet(),
				HasMouse:     scr.HasMouse(),
			},
			WorkDir:    opt.WorkDir,
			DataSource: opt.DataSource,
			PID:        pid(),
		},
	})
	if err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	first, err := dec.ToClient()
	if err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	switch first.Kind {
	case proto.KindReject:
		if first.Reject == nil {
			return errors.New("the server refused without saying why")
		}
		return errors.New(first.Reject.Reason)
	case proto.KindWelcome:
	default:
		return fmt.Errorf("the server answered the handshake with %v", first.Kind)
	}

	if opt.Err != nil && first.Welcome != nil {
		for _, w := range first.Welcome.Warnings {
			fmt.Fprintln(opt.Err, w)
		}
	}

	go sendEvents(scr, enc, rwc)

	return receive(scr, dec)
}

// sendEvents forwards what the terminal did until the terminal is gone.
func sendEvents(scr tcell.Screen, enc *proto.Encoder, rwc io.Closer) {
	for {
		ev := scr.PollEvent()
		if ev == nil {
			// The screen has been finalised: this terminal is over. Saying so
			// lets the server drop the client now rather than when its next
			// write fails.
			_ = enc.ToServer(proto.ToServer{Kind: proto.KindDetach})
			_ = rwc.Close()
			return
		}

		var m proto.ToServer
		switch e := ev.(type) {
		case *tcell.EventKey:
			m = proto.ToServer{Kind: proto.KindKey, Key: &proto.Key{
				Key: e.Key(), Rune: e.Rune(), Mods: e.Modifiers(),
			}}
		case *tcell.EventMouse:
			x, y := e.Position()
			m = proto.ToServer{Kind: proto.KindMouse, Mouse: &proto.Mouse{
				X: x, Y: y, Buttons: e.Buttons(), Mods: e.Modifiers(),
			}}
		case *tcell.EventResize:
			w, h := e.Size()
			m = proto.ToServer{Kind: proto.KindResize, Resize: &proto.Size{Width: w, Height: h}}
		case *tcell.EventPaste:
			m = proto.ToServer{Kind: proto.KindPaste, Paste: &proto.Paste{Start: e.Start()}}
		case *tcell.EventFocus:
			m = proto.ToServer{Kind: proto.KindFocus, Focus: &proto.Focus{Focused: e.Focused}}
		case *tcell.EventClipboard:
			m = proto.ToServer{Kind: proto.KindClipboardData, Clipboard: e.Data()}
		default:
			continue
		}

		if err := enc.ToServer(m); err != nil {
			return
		}
	}
}

// receive draws what the server sends until it stops sending.
func receive(scr tcell.Screen, dec *proto.Decoder) error {
	for {
		m, err := dec.ToClient()
		if err != nil {
			// The server is gone, or this client was detached. Neither is a
			// failure of this process, and neither has anything more to say.
			return nil
		}

		switch m.Kind {
		case proto.KindFrame:
			if m.Frame != nil {
				draw(scr, *m.Frame)
			}
		case proto.KindTitle:
			scr.SetTitle(m.Title)
		case proto.KindSetClipboard:
			scr.SetClipboard(m.Clipboard)
		case proto.KindRequestClipboard:
			scr.GetClipboard()
		case proto.KindBell:
			_ = scr.Beep()
		case proto.KindBye:
			return nil
		}
	}
}

func draw(scr tcell.Screen, f screen.Frame) {
	for _, c := range f.Cells {
		scr.SetContent(c.X, c.Y, c.Main, c.Comb, c.Style.Tcell())
	}
	if f.Cursor.Visible {
		scr.ShowCursor(f.Cursor.X, f.Cursor.Y)
	} else {
		scr.HideCursor()
	}
	scr.Show()
}
```

`pid()` is `os.Getpid()`; add the `os` import and call it inline if you
prefer. `EventFocus.Focused` is a field, `EventPaste.Start()` and
`EventClipboard.Data()` are methods — that is tcell v2.13.10 as written
above.

- [ ] **Step 4: Run the tests and watch them pass**

Run: `go test ./internal/attach/ -v -race`
Expected: all three PASS.

- [ ] **Step 5: Prove the round trip bites**

In `draw`, change `c.Style.Tcell()` to `tcell.StyleDefault` — the test still
passes, because it reads runes. That is the test being honest about its
scope: styles are pinned in Part 1, on both sides of the wire, and this test
is about the trip. Restore it, then instead delete the `scr.Show()` at the end
of `draw` and confirm `TestWhatTheSessionDrewIsWhatTheTerminalShows` FAILS
with `top row is "  " after five seconds`.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test -race ./internal/...
git add internal/attach/
git commit -m "Draw what arrives, send what the terminal did

The client does not know that dv talks to a database, what a datasource
is, or what any of the cells mean. That is the boundary worth keeping:
as long as it holds, nothing the session decides to draw can require a
change here.

The round-trip test is the one that says the split works — a keystroke
injected into a simulation screen on one side comes back as a cell on
the other, with a stand-in session in between and no database anywhere.

A screen that has been finalised sends a detach before closing, so the
server drops the client at once instead of finding out when its next
write fails."
```

---

### Task 4: A socket on disk that tells a live server from a dead one

**Files:**
- Create: `internal/daemon/socket.go`
- Test: `internal/daemon/socket_test.go`

**Interfaces:**
- Consumes: `*Server` from Task 2.
- Produces:
  - `func SocketPath() (string, error)`, `func LogPath() (string, error)`
  - `func Dial(path string) (net.Conn, error)`
  - `func Listen(path string) (net.Listener, error)`, `var ErrAlreadyRunning error`
  - `func (s *Server) Accept(ln net.Listener)`

- [ ] **Step 1: Write the failing test**

Create `internal/daemon/socket_test.go`:

```go
package daemon_test

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ahngbeom/datavase/internal/daemon"
)

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
	path := filepath.Join(t.TempDir(), "dv.sock")

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
	path := filepath.Join(t.TempDir(), "dv.sock")

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
	path := filepath.Join(t.TempDir(), "dv.sock")

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
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/daemon/ -run 'Socket|Stale|Listen'`
Expected: build failure — `daemon.SocketPath undefined`.

- [ ] **Step 3: Write the implementation**

Create `internal/daemon/socket.go`:

```go
package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
)

// ErrAlreadyRunning reports that something answered on the socket already.
var ErrAlreadyRunning = errors.New("a dv server is already listening")

// SocketPath is where a client looks for a server.
//
// Beside the schema cache and the query history rather than beside the
// configuration file: this is runtime state, and it means nothing once the
// process behind it is gone.
func SocketPath() (string, error) { return statePath("dv.sock") }

// LogPath is where a spawned server writes what it could not say to anyone.
//
// A server that dies while starting has no client to tell, and without this
// the failure is invisible.
func LogPath() (string, error) { return statePath("server.log") }

func statePath(name string) (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "datavase", name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "datavase", name), nil
}

// Dial connects to a running server.
func Dial(path string) (net.Conn, error) { return net.Dial("unix", path) }

// Listen takes the socket, replacing one that nothing answers on.
func Listen(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	// A socket file is not a running server. A crash leaves one behind, and
	// connecting is the only thing that can tell the difference — so connect,
	// and take a refusal as permission to replace it. Testing for the file
	// instead would mean one crash costs every later session.
	if c, err := Dial(path); err == nil {
		_ = c.Close()
		return nil, ErrAlreadyRunning
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	// The umask decides the mode a socket is created with, so it is set
	// afterwards rather than hoped for. This is a way into a session that can
	// read production databases.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	return ln, nil
}

// Accept serves clients until the listener closes.
func (s *Server) Accept(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go s.Serve(c)
	}
}
```

- [ ] **Step 4: Run the tests and watch them pass**

Run: `go test ./internal/daemon/ -v -race`
Expected: all PASS.

- [ ] **Step 5: Prove the stale-socket test bites**

Replace the `Dial` probe in `Listen` with a `os.Stat` existence check
returning `ErrAlreadyRunning`. Run `go test ./internal/daemon/ -run
TestAStaleSocketFileIsReplaced` and confirm it FAILS. Restore it. A file left
by a crash must never be able to lock the user out of their own tool.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test -race ./internal/...
git add internal/daemon/
git commit -m "Tell a live server from the file a dead one left

A socket file is not a running server. Testing for the file would mean
one crash costs every session after it, until somebody works out what to
delete. Connecting is the only thing that can tell the difference, so
Listen connects and takes a refusal as permission to replace what it
found.

The mode is set after listening rather than assumed, because the umask
decides what a socket is created with. This is a way into a session that
can read production databases.

There is a log path here for a reason that only shows up when things go
wrong: a spawned server that dies while starting has no client to tell."
```

---

### Task 5: dv spawns, attaches, or runs the way it always did

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/proto/proto.go` (add `KindStop`), `internal/daemon/serve.go` (handle it in `Serve` before the hello check)
- Create: `cmd/dv/runtime.go`, `cmd/dv/spawn_unix.go`, `cmd/dv/spawn_windows.go`
- Modify: `cmd/dv/main.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `daemon.*` (Tasks 2 and 4), `attach.Run` (Task 3).
- Produces:
  - `cli.App.Attach func(ctx context.Context, ds *config.DataSource, cfg *config.Config, opt UIOptions) error` — set when a runtime is available; nil falls back to `OpenUI`
  - `cli.App.RunServer func() error`, `cli.App.StopServer func() error`, `cli.App.ServerStatus func() (string, error)`
  - `dv server`, `dv server stop`, `dv status`, `dv --no-session`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/cli_test.go`:

```go
// Attaching must not ask for a password. The prompt needs a terminal and the
// connection is opened in another process, which is why a switch mid-session
// has never prompted either.
func TestOpenPrefersAttachAndDoesNotPrompt(t *testing.T) {
	var attached string
	app := &App{
		Config: &config.Config{DataSources: []config.DataSource{{Name: "local", Env: config.EnvDev}}},
		Out:    io.Discard,
		Err:    io.Discard,
		ReadPassword: func(string) (string, error) {
			t.Error("attaching asked for a password")
			return "", nil
		},
		Attach: func(_ context.Context, ds *config.DataSource, _ *config.Config, _ UIOptions) error {
			attached = ds.Name
			return nil
		},
		OpenUI: func(context.Context, *config.DataSource, string, *config.Config, UIOptions) error {
			t.Error("OpenUI was used while a runtime was available")
			return nil
		},
	}

	if code := app.Run(nil); code != exitOK {
		t.Fatalf("Run = %d, want %d", code, exitOK)
	}
	if attached != "local" {
		t.Errorf("attached to %q, want \"local\"", attached)
	}
}

// --no-session is the escape hatch, and it has to keep the behaviour that
// existed before there was anything to escape from.
func TestWithoutARuntimeOpenUIStillRuns(t *testing.T) {
	var opened string
	app := &App{
		Config:       &config.Config{DataSources: []config.DataSource{{Name: "local", Env: config.EnvDev}}},
		Out:          io.Discard,
		Err:          io.Discard,
		Secrets:      stubSecrets{"local": "hunter2"},
		ReadPassword: func(string) (string, error) { return "hunter2", nil },
		OpenUI: func(_ context.Context, ds *config.DataSource, pw string, _ *config.Config, _ UIOptions) error {
			opened = ds.Name + ":" + pw
			return nil
		},
	}

	if code := app.Run(nil); code != exitOK {
		t.Fatalf("Run = %d, want %d", code, exitOK)
	}
	if opened != "local:hunter2" {
		t.Errorf("opened %q, want \"local:hunter2\"", opened)
	}
}

// dv status has to answer even when there is no server, because "is one
// running" is the question it exists for.
func TestStatusReportsWithNoServer(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Config:       &config.Config{},
		Out:          &out,
		Err:          io.Discard,
		ServerStatus: func() (string, error) { return "no dv server is running", nil },
	}

	if code := app.Run([]string{"status"}); code != exitOK {
		t.Fatalf("Run = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out.String(), "no dv server") {
		t.Errorf("status printed %q", out.String())
	}
}
```

`stubSecrets` may already exist in `cli_test.go`; reuse it rather than adding
a second. If it does not, the smallest one that satisfies `secret.Store` is a
`map[string]string` with `Get`, `Set` and `Delete`.

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/cli/`
Expected: FAIL — `unknown field Attach in struct literal`, and `unknown
command "status"`.

- [ ] **Step 3: Add the fields and the commands**

In `internal/cli/cli.go`, add to `App`:

```go
	// Attach hands the datasource to a session running elsewhere. It is nil
	// for a monolithic run, which is when OpenUI is used instead.
	//
	// It takes no password: the process that opens the connection reads the
	// keychain itself, the way a mid-session switch already does.
	Attach func(ctx context.Context, ds *config.DataSource, cfg *config.Config, opt UIOptions) error

	// RunServer runs the headless server in the foreground.
	RunServer func() error
	// StopServer ends a running server.
	StopServer func() error
	// ServerStatus describes what is running, in one paragraph for a person.
	ServerStatus func() (string, error)
```

In `Run`'s switch, before `default`:

```go
	case "server":
		return a.server(args[1:])
	case "status":
		return a.serverStatus()
```

And the two commands:

```go
func (a *App) server(args []string) int {
	if len(args) > 0 && args[0] == "stop" {
		if a.StopServer == nil {
			fmt.Fprintln(a.Err, "this build cannot stop a server")
			return exitUsage
		}
		if err := a.StopServer(); err != nil {
			fmt.Fprintf(a.Err, "%v\n", err)
			return exitFailure
		}
		return exitOK
	}
	if len(args) > 0 {
		fmt.Fprintf(a.Err, "unknown command %q\n", "server "+args[0])
		return exitUsage
	}
	if a.RunServer == nil {
		fmt.Fprintln(a.Err, "this build has no server")
		return exitUsage
	}
	if err := a.RunServer(); err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitFailure
	}
	return exitOK
}

func (a *App) serverStatus() int {
	if a.ServerStatus == nil {
		fmt.Fprintln(a.Out, "this build has no server")
		return exitOK
	}
	report, err := a.ServerStatus()
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitFailure
	}
	fmt.Fprintln(a.Out, report)
	return exitOK
}
```

If `exitFailure` is spelled differently in this file, use the existing
constant.

In `open`, prefer `Attach` and skip the password when it is set. Find where
the password is resolved before `a.OpenUI(...)` and put this in front of it:

```go
	if a.Attach != nil {
		if err := a.Attach(ctx, ds, a.Config, opt); err != nil {
			fmt.Fprintf(a.Err, "%v\n", err)
			return exitFailure
		}
		return exitOK
	}
```

Add to `usage`, after the `dv keys` block:

```
  dv status             say whether a session is running, and on what
  dv --no-session       run without a session server

advanced:
  dv server             run the session server in the foreground
  dv server stop        end a running session
```

- [ ] **Step 4: Run the tests and watch them pass**

Run: `go test ./internal/cli/ -v`
Expected: the three new tests PASS along with the existing ones.

- [ ] **Step 5: Wire the runtime in cmd/dv**

Create `cmd/dv/spawn_unix.go`:

```go
//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detach puts the server in a session of its own, so that Ctrl+C in the
// terminal that happened to start it does not take the session down with the
// client.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
```

Create `cmd/dv/spawn_windows.go`:

```go
//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// detach puts the server in a process group of its own, so a console Ctrl+C
// does not take the session down with the client.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
```

Create `cmd/dv/runtime.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/Ahngbeom/datavase/internal/attach"
	"github.com/Ahngbeom/datavase/internal/cli"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/version"
)

// spawnWait bounds how long a freshly started server has to take its socket
// before the client gives up and runs on its own.
const spawnWait = 3 * time.Second

// attachSession reaches a running session, starting a server if there is
// none.
//
// Nothing here is allowed to cost the session. A socket that cannot be made,
// a server that will not start, a machine where none of this works at all —
// each of those costs persistence, says one line about it, and runs the way
// dv always did.
func attachSession(_ context.Context, ds *config.DataSource, cfg *config.Config, opt cli.UIOptions) error {
	path, err := daemon.SocketPath()
	if err != nil {
		return fallback(ds, cfg, opt, err)
	}

	conn, err := daemon.Dial(path)
	if err != nil {
		if err := spawnServer(); err != nil {
			return fallback(ds, cfg, opt, err)
		}
		conn, err = waitForSocket(path)
		if err != nil {
			return fallback(ds, cfg, opt, err)
		}
	}

	return attach.Run(conn, attach.Options{
		Version:    version.Version(),
		WorkDir:    opt.WorkDir,
		DataSource: ds.Name,
		Err:        os.Stderr,
	})
}

// fallback runs monolithically and says why, once.
func fallback(ds *config.DataSource, cfg *config.Config, opt cli.UIOptions, cause error) error {
	fmt.Fprintf(os.Stderr, "no session server (%v); this session ends with the terminal\n", cause)

	password, err := secrets().Get(ds.Name)
	if err != nil {
		return fmt.Errorf("no password for %q; run: dv auth %s", ds.Name, ds.Name)
	}
	return openUI(context.Background(), ds, password, cfg, opt)
}

func waitForSocket(path string) (net.Conn, error) {
	deadline := time.Now().Add(spawnWait)
	for {
		conn, err := daemon.Dial(path)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			logPath, _ := daemon.LogPath()
			return nil, fmt.Errorf("the server did not start; see %s", logPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func spawnServer() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	logPath, err := daemon.LogPath()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(exe, "server")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	detach(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// runServer is `dv server`: it holds the session until the session ends.
func runServer(cfg *config.Config) error {
	path, err := daemon.SocketPath()
	if err != nil {
		return err
	}

	ln, err := daemon.Listen(path)
	if err != nil {
		if errors.Is(err, daemon.ErrAlreadyRunning) {
			return errors.New("a dv server is already running")
		}
		return err
	}
	defer os.Remove(path)
	defer ln.Close()

	var srv *daemon.Server
	srv = daemon.New(daemon.Options{
		Version: version.Version(),
		Start: func(h proto.Hello) (daemon.Session, []string, error) {
			return buildSession(h, cfg, srv)
		},
	})

	go srv.Accept(ln)
	return srv.Wait()
}

// stopServer is `dv server stop`.
func stopServer() error {
	path, err := daemon.SocketPath()
	if err != nil {
		return err
	}
	conn, err := daemon.Dial(path)
	if err != nil {
		return errors.New("no dv server is running")
	}
	defer conn.Close()

	return proto.NewEncoder(conn).ToServer(proto.ToServer{Kind: proto.KindStop})
}

// serverStatus is `dv status`.
func serverStatus() (string, error) {
	path, err := daemon.SocketPath()
	if err != nil {
		return "", err
	}
	conn, err := daemon.Dial(path)
	if err != nil {
		return "no dv server is running", nil
	}
	conn.Close()
	return fmt.Sprintf("a dv server is running (%s)", path), nil
}
```

`proto.KindStop` is new: add it to the client-to-server constants in
`internal/proto/proto.go` and handle it in `daemon.Serve` before the hello
check — a stop request is not a client attaching, and answering it with
"the first message was not a hello" would be wrong:

```go
	if first.Kind == proto.KindStop {
		_ = rwc.Close()
		s.Stop()
		return
	}
```

`dv status` reports what it can see from outside. Part 3 replaces this
one-liner with the session's own answer over the observation socket; until
then it says whether a server is there, which is what the command is for.

- [ ] **Step 6: Move the session wiring out of openUI**

In `cmd/dv/main.go`, `openUI`'s body becomes two functions. Keep `openUI`
exactly as it is for the monolithic path, and add beside it:

```go
// sessionAdapter makes *ui.App satisfy daemon.Stateful.
//
// ui.App.State returns ui.RuntimeState, a type internal/ui owns so that
// package never has to import internal/daemon — the two are independent on
// purpose, App is built and tested in Task 1 before internal/daemon exists.
// daemon.Stateful requires daemon.State by name, and Go's interface
// satisfaction is exact on return types: a *ui.App handed to the daemon
// directly does not satisfy Stateful, and session.(Stateful) fails silently
// at runtime. The daemon then treats every session as busy — indistinguishable
// from a broken build, since nothing refuses to compile. This adapter is the
// one place that conversion belongs: cmd/dv already imports both packages to
// wire them together.
//
// SetScreen, Run, Stop and SwitchTo are unaffected — SwitchTo's signature
// matches daemon.Switcher exactly, so embedding satisfies it without a method
// here.
type sessionAdapter struct{ *ui.App }

func (a sessionAdapter) State(ctx context.Context) (daemon.State, error) {
	s, err := a.App.State(ctx)
	return daemon.State{DataSource: s.DataSource, Busy: s.Busy}, err
}

// buildSession is what the server calls when the first client arrives.
//
// It is openUI's wiring with two differences. The warnings go back to the
// caller instead of to stderr, because in this process stderr is a log file
// nobody reads. And the password comes from the keychain and nowhere else:
// the terminal that could answer a prompt is in another process, which is the
// same reason a mid-session switch has never prompted.
func buildSession(h proto.Hello, cfg *config.Config, srv *daemon.Server) (daemon.Session, []string, error) {
	ds, err := lookupDataSource(cfg, h.DataSource)
	if err != nil {
		return nil, nil, err
	}

	password, err := secrets().Get(ds.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("no password for %q; run: dv auth %s — or set %s",
			ds.Name, ds.Name, secret.EnvVarName(ds.Name))
	}

	keys, err := keymap.FromConfig(cfg.Keymap.Preset, cfg.Keymap.Actions)
	if err != nil {
		return nil, nil, fmt.Errorf("keymap: %w", err)
	}

	var warnings []string
	// … the cache, history, recent, intro and worktree blocks from openUI,
	// each appending to warnings instead of writing to os.Stderr …

	sess, err := session.Open(context.Background(), ds, password)
	if err != nil {
		return nil, nil, err
	}

	return sessionAdapter{ui.New(sess, cfg, ui.Deps{
		Keys:      keys,
		Cache:     cache,
		History:   hist,
		Worktree:  wt,
		Recent:    recents,
		IntroPath: introPath,
		Connect:   connectTo,
		Detach:    srv.Detach,
	})}, warnings, nil
}

// lookupDataSource resolves the name a client asked for, defaulting to the
// first configured datasource when it asked for none.
func lookupDataSource(cfg *config.Config, name string) (*config.DataSource, error) {
	if len(cfg.DataSources) == 0 {
		return nil, errors.New("no datasources are configured; run: dv init")
	}
	if name == "" {
		return &cfg.DataSources[0], nil
	}
	for i := range cfg.DataSources {
		if cfg.DataSources[i].Name == name {
			return &cfg.DataSources[i], nil
		}
	}
	return nil, fmt.Errorf("no datasource named %q", name)
}
```

The `defer cache.Close()` calls in `openUI` have no equivalent here: the
session outlives this function. They move onto the session's own teardown, or
are dropped — the process exits when the session ends, and closing a SQLite
handle on the way out of a process is not what keeps that file intact.

Then in `run()`, add the flag and the wiring:

```go
	noSession := flag.Bool("no-session", false, "run without a session server")
```

and, when building `cli.App`:

```go
	app := &cli.App{
		Config:       cfg,
		Secrets:      secrets(),
		Out:          os.Stdout,
		Err:          os.Stderr,
		ReadPassword: readPassword,
		Probe:        probe,
		OpenUI:       openUI,
		RunServer:    func() error { return runServer(cfg) },
		StopServer:   stopServer,
		ServerStatus: serverStatus,
	}
	if !*noSession {
		app.Attach = attachSession
	}
```

- [ ] **Step 7: Run everything**

```bash
gofmt -l . && go vet ./... && go test -race ./internal/... && make build
```

Then try it by hand, which is the only way to see the part no test covers:

```bash
./dv                    # opens; a server starts behind it
./dv status             # says one is running
# press Shift+F10 to detach
./dv status             # still running
./dv                    # the editor still has what you typed
# press Ctrl+Q to quit
./dv status             # says none is running
```

- [ ] **Step 8: Commit**

```bash
git add internal/cli/ cmd/dv/
git commit -m "Attach to a session, or start one, or do without

dv is spelled the same as before. What changed is behind it: the client
looks for a server, starts one if there is none, and runs the way it
always did when it cannot do either. That last branch is the one to keep
honest — a home directory that cannot hold a socket costs persistence
and one line on stderr, never the session, which is the bargain the
schema cache and the query history already make.

Attaching does not ask for a password. The prompt needs a terminal and
the connection is opened in another process, so the server reads the
keychain itself and says what to run when there is nothing there. That
is the rule a mid-session datasource switch has followed since it was
written, for the same reason.

The spawned server gets a session of its own so that Ctrl+C in whichever
terminal happened to start it does not take the session down with the
client, and a log file so that a server dying at startup is not silent."
```

---

### Task 6: It survives the terminal, against a real database

The property the whole of Parts 1 and 2 exists for. Everything until now has
been tested against a stand-in; this is the real interface, a real connection
and a real statement.

**Files:**
- Create: `internal/daemon/runtime_integration_test.go`
- Test: itself

**Interfaces:**
- Consumes: everything above, plus `internal/session`, `internal/ui`,
  `internal/keymap`, `internal/testmysql`.
- Produces: nothing.

- [ ] **Step 1: Start the database**

Run: `make db-up`
Expected: a MariaDB container on port 13306. This test does not run without
it, and does not run at all without the build tag.

- [ ] **Step 2: Write the failing test**

Create `internal/daemon/runtime_integration_test.go`:

```go
//go:build integration

package daemon_test

import (
	"context"
	"net"
	"strings"
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

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = attach.Run(client, attach.Options{Version: testVersion, Screen: sim})
	}()

	return sim, func() {
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
		Start: func(proto.Hello) (daemon.Session, []string, error) {
			return realSession(t, cfg), nil, nil
		},
	})
	t.Cleanup(srv.Stop)

	sim, leave := attachTo(t, srv, 100, 30)
	typeInto(sim, "SELECT SLEEP(3), 42 AS answer")
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
		Start: func(proto.Hello) (daemon.Session, []string, error) {
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
```

`screenCaps(w, h)` is a small helper returning `screen.Caps{Width: w, Height:
h, Colors: 256, CharacterSet: "UTF-8"}`; put it beside `testCaps` in
`daemon_test.go` and use one of the two everywhere.

- [ ] **Step 3: Run it and watch it pass**

Run: `go test -tags integration ./internal/daemon/ -run Integration -v`
(Or by name: `-run 'TestAStatementSurvives|TestAnotherDataSource'`.)
Expected: both PASS. They take about fifteen seconds between them, which is
what waiting for real sleeps costs.

- [ ] **Step 4: Prove the survival test bites**

In `Server.read`'s deferred function, add `s.Stop()`. Run
`TestAStatementSurvivesTheTerminal` and confirm it FAILS — the screen after
re-attaching never shows `42`, because the session died with the first
client. Remove it.

This is the same mutation as Task 2 Step 6, and it is worth doing twice: the
unit test says the session object was not stopped, and this one says a real
statement against a real server actually finished.

- [ ] **Step 5: Run everything and commit**

```bash
make lint && make test && make test-integration
git add internal/daemon/
git commit -m "Prove a statement outlives the terminal that started it

Everything until now was tested against a stand-in that echoed
keystrokes. This is the real interface on a real connection: type a
statement that sleeps, run it, close the terminal while it is still
going, come back, and find the answer on screen.

The second test is the safety rule the design put in front of it.
Switching datasource closes the session it leaves, and closing the
session kills the statement — so a second dv naming somewhere else is
refused while one is running, and told what it can do instead.

Both are behind the integration tag and need make db-up, because the
behaviour they pin down — streaming, cancellation, a connection that
outlives its terminal — does not exist against a stub."
```

---

## What this leaves

`dv` survives its terminal. A statement that is streaming keeps streaming, the
editor keeps its text, and running `dv` again lands back in the session that
was there. `dv --no-session` is the way back to the old behaviour, and a
machine where none of it works gets the old behaviour anyway with one line of
explanation.

`dv status` answers from outside — whether a socket answers, and nothing more.
Part 3 replaces that with the session's own account of itself over a
read-only observation socket, and adds `dv api snapshot`.

## Risk carried into Part 3

**The keychain is now read by a detached process.** With passwords kept off
the socket, `secret.NewKeychain()` in the server is the only path to a
connection. macOS keychain behaviour for a background process needs measuring
against a real login session before Task 5 is called done; `secret.WithEnv`
is the fallback, and the server's refusal names the environment variable, so
this degrades rather than breaks. It is the one thing in this plan that no
test in it can settle.
