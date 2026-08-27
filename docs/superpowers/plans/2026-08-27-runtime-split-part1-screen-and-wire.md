# Runtime Split, Part 1: The Screen and the Wire — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the two pure packages the runtime split rests on — a
`tcell.Screen` that turns what was drawn into frames, and the wire format that
carries them — with nothing yet using either.

**Architecture:** `internal/screen` implements `tcell.Screen` over a
`tcell.CellBuffer`, reporting the real terminal's capabilities from a `Caps`
value it is handed, and emitting only the cells that changed. `internal/proto`
encodes those frames and the events coming back, and holds the one-deep queue
that keeps a slow reader from stalling the drawing goroutine. Neither package
knows about sockets, tview, or that `dv` talks to a database.

**Tech Stack:** Go 1.26.4, `github.com/gdamore/tcell/v2` (already a
dependency), `encoding/gob` and `net.Pipe` from the standard library. No new
third-party dependency.

**Spec:** `docs/superpowers/specs/2026-08-27-dv-runtime-design.md`

## Global Constraints

- Repository `Ahngbeom/datavase`, default branch `main`. Work on a branch and
  open a PR; never push to `main`.
- `CGO_ENABLED=0` must stay viable. Add no dependency that needs cgo. This
  plan adds no third-party dependency at all.
- **tcell is Apache-2.0; datavase is MIT. Do not copy tcell source into this
  repository.** `screenImpl`/`baseScreen` in `screen.go` show how tcell splits
  the interface into primitives and derived methods; follow that shape,
  written fresh.
- `make lint` (`go vet ./...` and `gofmt -l .`) must be clean at every commit.
- `make test` must pass at every commit. No test in this plan needs a database
  or a terminal.
- Every package gets a doc comment that states what it deliberately does not
  know. That is the convention every package in `internal/` follows, and it is
  load-bearing — see `CLAUDE.md`.
- **Comments say why, not what.** Explain the failure a piece of code exists
  to prevent. Do not narrate.
- **Test names and comments state the user-visible consequence** being pinned
  down, not the function being called.
- **TDD.** Write the failing test first and watch it fail for the right
  reason. If a test goes from build-failure straight to green, mutate the
  implementation to confirm the test actually bites.
- Nothing exists in production code for a test's sake.

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/screen/screen.go` | `Caps`, `Screen`, and the whole of `tcell.Screen`: cell buffer, capabilities, cursor, events, no-ops. |
| `internal/screen/frame.go` | `Cell`, `Cursor`, `Frame`, `Sink`; `Attach`/`Detach`; `Show`/`Sync`; `Frame.Merge`. |
| `internal/screen/screen_test.go` | Capabilities, cell round trip, events, `Fini`. |
| `internal/screen/frame_test.go` | Diffing, wide runes, sync, detach and re-attach, merge. |
| `internal/proto/proto.go` | `ToServer`, `ToClient`, payload types, `Kind`. |
| `internal/proto/codec.go` | `Encoder`/`Decoder` over `io.Writer`/`io.Reader`. |
| `internal/proto/queue.go` | The one-deep frame queue. |
| `internal/proto/proto_test.go` | Round trips over `net.Pipe`. |
| `internal/proto/queue_test.go` | Coalescing, and that the producer never blocks. |

`screen.go` and `frame.go` split by responsibility rather than by interface
method: everything about *holding* cells is in one, everything about *shipping*
them in the other.

---

### Task 1: A tcell.Screen that holds cells locally

The screen with no output yet: it satisfies `tcell.Screen`, keeps what was
drawn, reports the capabilities it was handed, and delivers events. Frames come
in Task 2.

**Files:**
- Create: `internal/screen/screen.go`
- Test: `internal/screen/screen_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Caps struct { Width, Height int; Colors int; CharacterSet string; HasMouse bool }`
  - `func New(caps Caps) *Screen`
  - `*Screen` satisfies `tcell.Screen`.

- [ ] **Step 1: Write the failing test**

Create `internal/screen/screen_test.go`:

```go
package screen_test

import (
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// A screen that understates what the terminal can do makes the interface
// degrade itself, and nothing past this point can undo that.
func TestCapabilitiesAreTheOnesSupplied(t *testing.T) {
	s := screen.New(screen.Caps{
		Width: 80, Height: 24,
		Colors:       16777216,
		CharacterSet: "UTF-8",
		HasMouse:     true,
	})

	if got := s.Colors(); got != 16777216 {
		t.Errorf("Colors() = %d, want 16777216", got)
	}
	if got := s.CharacterSet(); got != "UTF-8" {
		t.Errorf("CharacterSet() = %q, want %q", got, "UTF-8")
	}
	if !s.HasMouse() {
		t.Error("HasMouse() = false; the terminal said it has one")
	}
	if w, h := s.Size(); w != 80 || h != 24 {
		t.Errorf("Size() = %dx%d, want 80x24", w, h)
	}
}

// A cell that loses its style or its width is a cell that will be redrawn
// wrongly on the terminal.
func TestDrawnCellReadsBack(t *testing.T) {
	s := screen.New(screen.Caps{Width: 80, Height: 24, Colors: 256, CharacterSet: "UTF-8"})
	style := tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)

	s.SetContent(3, 4, '가', nil, style)

	main, comb, got, width := s.GetContent(3, 4)
	if main != '가' {
		t.Errorf("primary rune = %q, want %q", main, '가')
	}
	if len(comb) != 0 {
		t.Errorf("combining = %q, want none", comb)
	}
	if got != style {
		t.Error("style did not survive the round trip")
	}
	if width != 2 {
		t.Errorf("width = %d, want 2; a wide rune that reports 1 corrupts the row", width)
	}
}

// A key pressed while the interface is busy has to wait, not vanish.
func TestPostedEventIsPolled(t *testing.T) {
	s := screen.New(screen.Caps{Width: 80, Height: 24, Colors: 256, CharacterSet: "UTF-8"})

	want := tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	if err := s.PostEvent(want); err != nil {
		t.Fatalf("PostEvent: %v", err)
	}

	got, ok := s.PollEvent().(*tcell.EventKey)
	if !ok {
		t.Fatalf("PollEvent returned %T, want *tcell.EventKey", got)
	}
	if got.Rune() != 'q' {
		t.Errorf("rune = %q, want 'q'", got.Rune())
	}
}

// tview's event loop calls PollEvent forever; a screen that never releases it
// leaves the goroutine alive after the interface has stopped.
func TestFiniReleasesPollEvent(t *testing.T) {
	s := screen.New(screen.Caps{Width: 80, Height: 24, Colors: 256, CharacterSet: "UTF-8"})

	done := make(chan tcell.Event, 1)
	go func() { done <- s.PollEvent() }()

	s.Fini()

	select {
	case ev := <-done:
		if ev != nil {
			t.Errorf("PollEvent returned %T after Fini, want nil", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PollEvent still blocked two seconds after Fini")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/screen/`
Expected: build failure — `no required module provides package
github.com/Ahngbeom/datavase/internal/screen`.

- [ ] **Step 3: Write the implementation**

Create `internal/screen/screen.go`:

```go
// Package screen is a tcell.Screen whose terminal is somewhere else.
//
// It holds cells, turns what was drawn into frames, and accepts the events a
// terminal would have produced. It does not know how a frame reaches a
// terminal, and it does not know that the interface drawing on it is a
// database client — which is what lets all of it be tested without either.
//
// Capabilities are reported from the Caps it was built with rather than
// guessed. Whatever is holding the real terminal is the only thing that can
// answer how many colours it has, and a screen that understates that makes
// the interface degrade itself somewhere no later stage can repair.
package screen

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

// eventQueue bounds how many unread events may pile up. tcell's own screens
// bound theirs for the same reason: an unbounded queue turns a wedged
// interface into exhausted memory instead of a dropped keystroke.
const eventQueue = 128

// Caps is what the real terminal can do.
//
// It is supplied rather than probed because the terminal is elsewhere. Every
// field here is a question only the process holding it can answer.
type Caps struct {
	Width, Height int
	Colors        int
	CharacterSet  string
	HasMouse      bool
}

// Screen implements tcell.Screen against a cell buffer instead of a terminal.
type Screen struct {
	mu   sync.Mutex
	buf  tcell.CellBuffer
	caps Caps

	// style is the default set by SetStyle, used by Clear and PutStr.
	style tcell.Style

	cursor      Cursor
	cursorStyle tcell.CursorStyle
	cursorColor tcell.Color

	sink Sink

	events chan tcell.Event
	quit   chan struct{}
	once   sync.Once
}

var _ tcell.Screen = (*Screen)(nil)

// New returns a screen sized and capability-reported per caps.
func New(caps Caps) *Screen {
	s := &Screen{
		caps:   caps,
		style:  tcell.StyleDefault,
		events: make(chan tcell.Event, eventQueue),
		quit:   make(chan struct{}),
	}
	s.buf.Resize(caps.Width, caps.Height)
	return s
}

func (s *Screen) Init() error { return nil }

// Fini releases anything blocked in PollEvent. It is safe to call twice: the
// interface calls it on the way out, and so does whatever owns the screen.
func (s *Screen) Fini() {
	s.once.Do(func() { close(s.quit) })
}

// --- capabilities: the client's answers, never invented ---

func (s *Screen) Colors() int           { return s.caps.Colors }
func (s *Screen) CharacterSet() string  { return s.caps.CharacterSet }
func (s *Screen) HasMouse() bool        { return s.caps.HasMouse }
func (s *Screen) HasKey(tcell.Key) bool { return true }

func (s *Screen) Size() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Size()
}

// CanDisplay answers for the client's character set, because that is where
// the encoder actually lives. Under UTF-8 everything goes; under anything
// else only ASCII is safe to promise, and the client's own screen substitutes
// for the rest.
func (s *Screen) CanDisplay(r rune, _ bool) bool {
	if s.caps.CharacterSet == "UTF-8" {
		return true
	}
	return r < 0x80
}

// Rune fallbacks are the client's business: substitution happens where the
// encoder is, on the real screen. Recording them here would mean two
// substitution tables that can disagree.
func (s *Screen) RegisterRuneFallback(rune, string) {}
func (s *Screen) UnregisterRuneFallback(rune)       {}

// --- cell buffer ---

func (s *Screen) SetStyle(style tcell.Style) {
	s.mu.Lock()
	s.style = style
	s.mu.Unlock()
}

func (s *Screen) Put(x, y int, str string, style tcell.Style) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Put(x, y, str, style)
}

func (s *Screen) PutStrStyled(x, y int, str string, style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cols, rows := s.buf.Size()
	for str != "" && x < cols && y < rows {
		remain, width := s.buf.Put(x, y, str, style)
		if width == 0 {
			return
		}
		str = remain
		x += width
	}
}

func (s *Screen) PutStr(x, y int, str string) {
	s.mu.Lock()
	style := s.style
	s.mu.Unlock()
	s.PutStrStyled(x, y, str, style)
}

func (s *Screen) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.SetContent(x, y, mainc, combc, style)
}

// SetCell is tcell's deprecated spelling of Put, kept because the interface
// still carries it.
func (s *Screen) SetCell(x, y int, style tcell.Style, ch ...rune) {
	if len(ch) == 0 {
		ch = []rune{' '}
	}
	s.Put(x, y, string(ch), style)
}

func (s *Screen) Get(x, y int) (string, tcell.Style, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Get(x, y)
}

func (s *Screen) GetContent(x, y int) (rune, []rune, tcell.Style, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.GetContent(x, y)
}

// Clear fills with the default style rather than the one SetStyle last set,
// which is what tcell's own screens do.
func (s *Screen) Clear() { s.Fill(' ', tcell.StyleDefault) }

func (s *Screen) Fill(r rune, style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Fill(r, style)
}

// --- cursor ---

func (s *Screen) ShowCursor(x, y int) {
	s.mu.Lock()
	s.cursor = Cursor{X: x, Y: y, Visible: true}
	s.mu.Unlock()
}

func (s *Screen) HideCursor() {
	s.mu.Lock()
	s.cursor.Visible = false
	s.mu.Unlock()
}

func (s *Screen) SetCursorStyle(cs tcell.CursorStyle, cc ...tcell.Color) {
	s.mu.Lock()
	s.cursorStyle = cs
	if len(cc) > 0 {
		s.cursorColor = cc[0]
	}
	s.mu.Unlock()
}

// --- events ---

func (s *Screen) PollEvent() tcell.Event {
	select {
	case ev := <-s.events:
		return ev
	case <-s.quit:
		return nil
	}
}

func (s *Screen) HasPendingEvent() bool { return len(s.events) > 0 }

func (s *Screen) PostEvent(ev tcell.Event) error {
	select {
	case s.events <- ev:
		return nil
	case <-s.quit:
		return nil
	default:
		return tcell.ErrEventQFull
	}
}

// PostEventWait is what whatever reads the transport should use: a key that
// arrived while the interface was busy should wait its turn rather than be
// refused.
func (s *Screen) PostEventWait(ev tcell.Event) {
	select {
	case s.events <- ev:
	case <-s.quit:
	}
}

func (s *Screen) ChannelEvents(ch chan<- tcell.Event, quit <-chan struct{}) {
	defer close(ch)
	for {
		select {
		case <-quit:
			return
		case <-s.quit:
			return
		case ev := <-s.events:
			select {
			case ch <- ev:
			case <-quit:
				return
			case <-s.quit:
				return
			}
		}
	}
}

// --- things a screen without a terminal cannot do ---

// EnableMouse and friends record nothing: the client turned these on for its
// own screen before it ever connected, and the interface asking again here
// changes nothing about what the terminal will send.
func (s *Screen) EnableMouse(...tcell.MouseFlags) {}
func (s *Screen) DisableMouse()                   {}
func (s *Screen) EnablePaste()                    {}
func (s *Screen) DisablePaste()                   {}
func (s *Screen) EnableFocus()                    {}
func (s *Screen) DisableFocus()                   {}

// Resize is tcell's vestigial window-resize request; no backend implements it.
func (s *Screen) Resize(int, int, int, int) {}

// LockRegion marks cells the terminal must not touch during a redraw. There
// is no terminal here to hold off.
func (s *Screen) LockRegion(int, int, int, int, bool) {}

// Suspend and Resume put the terminal back in its original mode for a shell.
// The terminal is in another process, which suspends itself if it wants to.
func (s *Screen) Suspend() error { return nil }
func (s *Screen) Resume() error  { return nil }

// Tty is the file descriptor a terminal screen was built on. There is none.
func (s *Screen) Tty() (tcell.Tty, bool) { return nil, false }
```

`Cursor`, `Sink`, `Show` and `Sync` are named here but defined in Task 2. To
keep this task compiling on its own, add a temporary `frame.go` containing
only:

```go
package screen

// Cursor is where the terminal should put the caret.
type Cursor struct {
	X, Y    int
	Visible bool
}

// Sink is where a drawn screen goes. Task 2 gives it methods.
type Sink interface{}

// Show reconciles the buffer after a draw. Task 2 makes it emit a frame.
func (s *Screen) Show() {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, h := s.buf.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s.buf.SetDirty(x, y, false)
		}
	}
}

// Sync redraws everything. Task 2 makes it emit a whole frame.
func (s *Screen) Sync() {
	s.mu.Lock()
	s.buf.Invalidate()
	s.mu.Unlock()
	s.Show()
}

// SetSize is what a client resize arrives as. Task 4 makes it post the event
// the interface needs to relayout.
func (s *Screen) SetSize(w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Resize(w, h)
}

func (s *Screen) SetTitle(string)      {}
func (s *Screen) SetClipboard([]byte)  {}
func (s *Screen) GetClipboard()        {}
func (s *Screen) Beep() error          { return nil }
```

- [ ] **Step 4: Run the test and watch it pass**

Run: `go test ./internal/screen/ -v`
Expected: all four tests PASS.

- [ ] **Step 5: Prove the capability test bites**

Change `Colors()` to `return 256`, run `go test ./internal/screen/ -run
TestCapabilitiesAreTheOnesSupplied`, and confirm it FAILS with `Colors() = 256,
want 16777216`. This is the exact bug `tcell.SimulationScreen` has, and the
test exists to keep it out. Restore the correct implementation.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/screen/
git add internal/screen/
git commit -m "Hold the cells for a terminal that is somewhere else

The first half of a tcell.Screen with no terminal under it: it keeps
what was drawn, delivers the events a terminal would have produced, and
reports capabilities from the Caps it was handed.

Reporting rather than guessing is the part worth the type. tcell's own
SimulationScreen hardcodes Colors() to 256 and HasMouse() to false, which
would make the palette degrade itself against a truecolor terminal with
nothing downstream able to tell. A test pins it.

Frames come next; Show only reconciles the buffer for now."
```

---

### Task 2: Turn what was drawn into frames

**Files:**
- Modify: `internal/screen/frame.go` (replaces the placeholder from Task 1)
- Modify: `internal/screen/screen.go:` the `sink` field is already there
- Test: `internal/screen/frame_test.go`

**Interfaces:**
- Consumes: `screen.New`, `*Screen` from Task 1.
- Produces:
  - `type Style struct { Fg, Bg tcell.Color; Attrs tcell.AttrMask; UnderlineStyle tcell.UnderlineStyle; UnderlineColor tcell.Color }`
  - `func StyleFrom(tcell.Style) Style` and `func (Style) Tcell() tcell.Style`
  - `type Cell struct { X, Y int; Main rune; Comb []rune; Style Style; Width int }`
  - `type Cursor struct { X, Y int; Visible bool }`
  - `type Frame struct { Cells []Cell; Cursor Cursor }`
  - `type Sink interface { Frame(Frame); SetTitle(string); SetClipboard([]byte); RequestClipboard(); Bell() }`
  - `func (s *Screen) Attach(sink Sink)` and `func (s *Screen) Detach()`

- [ ] **Step 1: Write the failing test**

Create `internal/screen/frame_test.go`:

```go
package screen_test

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// recorder is a Sink that keeps what it was given.
type recorder struct {
	frames    []screen.Frame
	titles    []string
	clipboard [][]byte
	requests  int
	bells     int
}

func (r *recorder) Frame(f screen.Frame)      { r.frames = append(r.frames, f) }
func (r *recorder) SetTitle(t string)         { r.titles = append(r.titles, t) }
func (r *recorder) SetClipboard(b []byte)     { r.clipboard = append(r.clipboard, b) }
func (r *recorder) RequestClipboard()         { r.requests++ }
func (r *recorder) Bell()                     { r.bells++ }

func attached(t *testing.T, w, h int) (*screen.Screen, *recorder) {
	t.Helper()
	s := screen.New(screen.Caps{Width: w, Height: h, Colors: 256, CharacterSet: "UTF-8"})
	r := &recorder{}
	s.Attach(r)
	return s, r
}

// last returns the most recent frame, failing if there is none.
func last(t *testing.T, r *recorder) screen.Frame {
	t.Helper()
	if len(r.frames) == 0 {
		t.Fatal("no frame was emitted")
	}
	return r.frames[len(r.frames)-1]
}

// find returns the cell at x,y, or fails.
func find(t *testing.T, f screen.Frame, x, y int) screen.Cell {
	t.Helper()
	for _, c := range f.Cells {
		if c.X == x && c.Y == y {
			return c
		}
	}
	t.Fatalf("no cell at %d,%d in a frame of %d cells", x, y, len(f.Cells))
	return screen.Cell{}
}

// Sending the whole screen on every draw would put a slow terminal a whole
// screen behind on every keystroke.
func TestOnlyChangedCellsAreSent(t *testing.T) {
	s, r := attached(t, 20, 5)
	s.Sync() // the first frame is the whole screen; discard it
	r.frames = nil

	s.SetContent(2, 1, 'x', nil, tcell.StyleDefault)
	s.Show()

	f := last(t, r)
	if len(f.Cells) != 1 {
		t.Fatalf("changing one cell produced %d cells", len(f.Cells))
	}
	if f.Cells[0].Main != 'x' {
		t.Errorf("rune = %q, want 'x'", f.Cells[0].Main)
	}

	r.frames = nil
	s.Show()

	if got := last(t, r); len(got.Cells) != 0 {
		t.Errorf("redrawing unchanged content produced %d cells, want 0", len(got.Cells))
	}
}

// A wide rune occupies two columns but is one cell. Sending the column behind
// it would draw whatever the buffer never wrote there.
func TestWideRuneIsOneCell(t *testing.T) {
	s, r := attached(t, 20, 5)
	s.Sync()
	r.frames = nil

	s.SetContent(4, 2, '한', nil, tcell.StyleDefault)
	s.Show()

	f := last(t, r)
	if len(f.Cells) != 1 {
		t.Fatalf("one wide rune produced %d cells, want 1", len(f.Cells))
	}
	c := find(t, f, 4, 2)
	if c.Width != 2 {
		t.Errorf("Width = %d, want 2", c.Width)
	}
	for _, other := range f.Cells {
		if other.X == 5 && other.Y == 2 {
			t.Error("the column behind the wide rune was sent; it holds nothing the buffer wrote")
		}
	}
}

// Re-attaching has to repaint everything: the terminal on the other side is
// showing whatever it had before, or nothing at all.
func TestSyncSendsTheWholeScreen(t *testing.T) {
	s, r := attached(t, 20, 5)
	s.Show()
	r.frames = nil

	s.Sync()

	if got := len(last(t, r).Cells); got != 20*5 {
		t.Errorf("Sync sent %d cells, want %d", got, 20*5)
	}
}

// The caret has to arrive with the cells it belongs to; a frame that moves
// text without moving the caret leaves it over the wrong character.
func TestCursorTravelsWithTheFrame(t *testing.T) {
	s, r := attached(t, 20, 5)

	s.ShowCursor(7, 3)
	s.Show()

	f := last(t, r)
	if !f.Cursor.Visible || f.Cursor.X != 7 || f.Cursor.Y != 3 {
		t.Errorf("Cursor = %+v, want {7 3 true}", f.Cursor)
	}

	s.HideCursor()
	s.Show()

	if last(t, r).Cursor.Visible {
		t.Error("cursor still reported visible after HideCursor")
	}
}

// The interface reaches the terminal's clipboard over OSC 52, and holding the
// screen is the only reason it keeps a reference to one.
func TestClipboardAndTitleReachTheSink(t *testing.T) {
	s, r := attached(t, 20, 5)

	s.SetTitle("dv — prod")
	s.SetClipboard([]byte("select 1"))
	s.GetClipboard()
	if err := s.Beep(); err != nil {
		t.Fatalf("Beep: %v", err)
	}

	if len(r.titles) != 1 || r.titles[0] != "dv — prod" {
		t.Errorf("titles = %q", r.titles)
	}
	if len(r.clipboard) != 1 || string(r.clipboard[0]) != "select 1" {
		t.Errorf("clipboard = %q", r.clipboard)
	}
	if r.requests != 1 {
		t.Errorf("clipboard requests = %d, want 1", r.requests)
	}
	if r.bells != 1 {
		t.Errorf("bells = %d, want 1", r.bells)
	}
}

// A colour that does not survive being flattened for the wire is a colour
// lost for the whole session, and nothing later can tell it was ever there.
func TestStyleSurvivesFlattening(t *testing.T) {
	want := tcell.StyleDefault.
		Foreground(tcell.ColorRed).
		Background(tcell.ColorNavy).
		Bold(true).
		Underline(tcell.UnderlineStyleCurly, tcell.ColorYellow)

	if got := screen.StyleFrom(want).Tcell(); got != want {
		t.Error("StyleFrom/Tcell did not give back the style that went in")
	}
}

// Drawing with nobody watching must not panic, and must not queue: the
// interface keeps drawing while a statement streams whether or not anyone is
// attached.
func TestDrawingWhileDetachedIsSafe(t *testing.T) {
	s := screen.New(screen.Caps{Width: 20, Height: 5, Colors: 256, CharacterSet: "UTF-8"})

	s.SetContent(1, 1, 'a', nil, tcell.StyleDefault)
	s.Show()
	s.Sync()

	s.Detach() // detaching when never attached is not an error either
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/screen/`
Expected: build failure — `s.Attach undefined`, `screen.Frame undefined`,
`screen.Cell undefined`.

- [ ] **Step 3: Write the implementation**

Replace `internal/screen/frame.go` entirely:

```go
package screen

import "github.com/gdamore/tcell/v2"

// Style is tcell.Style in a form an encoder can carry.
//
// tcell.Style has no exported fields, so encoding/gob writes it as a zero
// value and every cell arrives colourless. Nothing catches that at compile
// time, which is why this type exists and why a test pins the round trip.
//
// The hyperlink a style can carry (OSC 8) is not here: tcell offers Url and
// UrlId as setters and no getters, so it cannot be read back out. dv writes
// no hyperlink markup — see the risk note in the spec — and a style that
// carried one would arrive without it.
type Style struct {
	Fg, Bg         tcell.Color
	Attrs          tcell.AttrMask
	UnderlineStyle tcell.UnderlineStyle
	UnderlineColor tcell.Color
}

// StyleFrom flattens a tcell.Style.
func StyleFrom(s tcell.Style) Style {
	fg, bg, attrs := s.Decompose()
	return Style{
		Fg: fg, Bg: bg, Attrs: attrs,
		UnderlineStyle: s.GetUnderlineStyle(),
		UnderlineColor: s.GetUnderlineColor(),
	}
}

// Tcell rebuilds the style for a screen to draw with.
//
// Attributes goes last because Underline sets and clears AttrUnderline as a
// side effect; applying the recorded mask afterwards restores exactly what
// was decomposed.
func (s Style) Tcell() tcell.Style {
	return tcell.StyleDefault.
		Foreground(s.Fg).
		Background(s.Bg).
		Underline(s.UnderlineStyle, s.UnderlineColor).
		Attributes(s.Attrs)
}

// Cell is one position on the screen and what belongs there.
//
// Width travels with it so a wrong one can be seen rather than inferred. The
// client does not need it to place the next cell — every cell carries its own
// coordinates.
type Cell struct {
	X, Y  int
	Main  rune
	Comb  []rune
	Style Style
	Width int
}

// Cursor is where the terminal should put the caret.
type Cursor struct {
	X, Y    int
	Visible bool
}

// Frame is what changed since the last one, plus where the caret ended up.
type Frame struct {
	Cells  []Cell
	Cursor Cursor
}

// Sink is where a drawn screen goes.
//
// It is an interface rather than a set of callbacks so that "attached" is one
// thing to hold and one thing to drop, and so a test can record every kind of
// output in one place.
type Sink interface {
	Frame(Frame)
	SetTitle(string)
	SetClipboard([]byte)
	// RequestClipboard asks the terminal for its contents. Most terminals
	// refuse, so no reply is the ordinary case.
	RequestClipboard()
	Bell()
}

// Attach makes sink the destination and repaints everything into it.
//
// The repaint is not politeness: Show clears dirty flags whether or not
// anyone was listening, so a screen drawn while detached has no record of
// what changed. Without the full frame, the first one after re-attaching is
// empty and the terminal keeps showing whatever it had.
func (s *Screen) Attach(sink Sink) {
	s.mu.Lock()
	s.sink = sink
	s.mu.Unlock()

	s.Sync()
}

// Detach stops emitting. The interface goes on drawing and does not learn
// that nobody is watching.
func (s *Screen) Detach() {
	s.mu.Lock()
	s.sink = nil
	s.mu.Unlock()
}

// Show emits the cells that changed since the last call and clears their
// flags.
func (s *Screen) Show() { s.emit(false) }

// Sync emits every cell.
//
// It does not go through the buffer's dirty flags. tcell marks a cell dirty
// by clearing the record of what was last drawn there, which leaves a cell
// nothing was ever written to indistinguishable from a clean one — so an
// invalidated blank screen reports nothing to send. A repaint has to mean
// every cell, because the terminal being repainted for may be showing
// anything at all.
func (s *Screen) Sync() { s.emit(true) }

func (s *Screen) emit(all bool) {
	s.mu.Lock()

	var cells []Cell
	w, h := s.buf.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; {
			main, comb, style, width := s.buf.GetContent(x, y)
			if all || s.buf.Dirty(x, y) {
				s.buf.SetDirty(x, y, false)
				cells = append(cells, Cell{
					X: x, Y: y,
					Main: main, Comb: comb,
					Style: StyleFrom(style), Width: width,
				})
			}
			// The column behind a wide rune holds nothing the buffer was
			// asked to write. Sending it would paint whatever was there
			// before over the second half of the character.
			if width > 1 {
				x += width
			} else {
				x++
			}
		}
	}

	frame := Frame{Cells: cells, Cursor: s.cursor}
	sink := s.sink
	s.mu.Unlock()

	if sink != nil {
		sink.Frame(frame)
	}
}

// SetSize is what a client resize arrives as. Task 4 posts the event the
// interface needs to relayout.
func (s *Screen) SetSize(w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Resize(w, h)
}

func (s *Screen) SetTitle(title string) {
	if sink := s.currentSink(); sink != nil {
		sink.SetTitle(title)
	}
}

func (s *Screen) SetClipboard(data []byte) {
	if sink := s.currentSink(); sink != nil {
		sink.SetClipboard(data)
	}
}

func (s *Screen) GetClipboard() {
	if sink := s.currentSink(); sink != nil {
		sink.RequestClipboard()
	}
}

func (s *Screen) Beep() error {
	if sink := s.currentSink(); sink != nil {
		sink.Bell()
	}
	return nil
}

func (s *Screen) currentSink() Sink {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sink
}
```

- [ ] **Step 4: Run the tests and watch them pass**

Run: `go test ./internal/screen/ -v`
Expected: every test PASSES, Task 1's included.

- [ ] **Step 5: Prove the wide-rune test bites**

In `Show`, change the advance to `x++` unconditionally. Run `go test
./internal/screen/ -run TestWideRuneIsOneCell` and confirm it FAILS with the
column-behind message. Restore it.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/screen/
git add internal/screen/
git commit -m "Send only what changed, and never the half of a wide rune

Show walks the buffer's own dirty flags — the ones tcell's terminal
backend uses — so a keystroke costs the cells it touched rather than a
screen. Sync invalidates first, which is what a resize and a fresh
terminal both need.

The advance past a wide rune is the part that matters here. The column
behind one holds nothing the buffer was ever asked to write, so sending
it would paint stale content over the second half of the character. A
grid of Korean text finds that immediately, which is why the test is
written in it.

Attach takes the full-repaint path for a reason worth stating: Show
clears dirty flags whether or not a sink is listening, so without it the
first frame after re-attaching is empty and the terminal keeps showing
what it had. Sync does not go through the dirty flags at all — tcell
marks a cell dirty by forgetting what was last drawn there, which leaves
a never-written cell indistinguishable from a clean one, so an
invalidated blank screen would report nothing to send.

Cells carry a flattened style rather than a tcell.Style because that
type has no exported fields: gob would write it as a zero value and
every cell would arrive colourless, with nothing failing to build."
```

---

### Task 3: Merge two frames into one

**Files:**
- Modify: `internal/screen/frame.go`
- Test: `internal/screen/frame_test.go`

**Interfaces:**
- Consumes: `Frame`, `Cell`, `Cursor` from Task 2.
- Produces: `func (f Frame) Merge(next Frame) Frame`

- [ ] **Step 1: Write the failing test**

Append to `internal/screen/frame_test.go`:

```go
// apply plays a frame onto a grid, the way a client would.
func apply(grid map[[2]int]rune, f screen.Frame) {
	for _, c := range f.Cells {
		grid[[2]int{c.X, c.Y}] = c.Main
	}
}

// Merging is what lets a slow reader be given one frame instead of a queue.
// It is only allowed if the result draws the same screen as playing both.
func TestMergeDrawsTheSameScreenAsPlayingBoth(t *testing.T) {
	first := screen.Frame{
		Cells: []screen.Cell{
			{X: 1, Y: 0, Main: 'a', Width: 1},
			{X: 5, Y: 2, Main: 'b', Width: 1},
		},
		Cursor: screen.Cursor{X: 1, Y: 0, Visible: true},
	}
	second := screen.Frame{
		Cells: []screen.Cell{
			{X: 1, Y: 0, Main: 'z', Width: 1},
			{X: 9, Y: 4, Main: 'c', Width: 1},
		},
		Cursor: screen.Cursor{X: 9, Y: 4, Visible: true},
	}

	inOrder := map[[2]int]rune{}
	apply(inOrder, first)
	apply(inOrder, second)

	merged := map[[2]int]rune{}
	apply(merged, first.Merge(second))

	if len(merged) != len(inOrder) {
		t.Fatalf("merged screen has %d cells, playing both gives %d", len(merged), len(inOrder))
	}
	for pos, want := range inOrder {
		if merged[pos] != want {
			t.Errorf("at %v: merged has %q, playing both gives %q", pos, merged[pos], want)
		}
	}
}

// A cell the waiting frame carried and the new one does not must survive.
// Dropping it instead of merging leaves stale content on screen forever.
func TestMergeKeepsCellsTheNewFrameDoesNotTouch(t *testing.T) {
	waiting := screen.Frame{Cells: []screen.Cell{{X: 5, Y: 2, Main: 'b', Width: 1}}}
	arriving := screen.Frame{Cells: []screen.Cell{{X: 1, Y: 0, Main: 'z', Width: 1}}}

	got := waiting.Merge(arriving)

	if len(got.Cells) != 2 {
		t.Fatalf("merge produced %d cells, want 2", len(got.Cells))
	}
	find(t, got, 5, 2)
	find(t, got, 1, 0)
}

// The caret is not a cell: the newest position is the only correct one.
func TestMergeTakesTheNewerCursor(t *testing.T) {
	waiting := screen.Frame{Cursor: screen.Cursor{X: 1, Y: 1, Visible: true}}
	arriving := screen.Frame{Cursor: screen.Cursor{X: 8, Y: 3, Visible: false}}

	got := waiting.Merge(arriving)

	if got.Cursor != (screen.Cursor{X: 8, Y: 3, Visible: false}) {
		t.Errorf("Cursor = %+v, want the arriving one", got.Cursor)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/screen/ -run TestMerge`
Expected: build failure — `first.Merge undefined`.

- [ ] **Step 3: Write the implementation**

Append to `internal/screen/frame.go`:

```go
// Merge folds next into f so one frame can be delivered where two were
// waiting.
//
// It is sound because both are differences against the same picture — the one
// the reader is still showing, having applied neither. For any position the
// later cell wins; a position only f touched survives, which is the whole
// difference between merging and discarding. Discarding would leave whatever f
// was carrying stale on the screen for good.
//
// The cursor is not a position on the screen but a single value, so the newer
// one is simply correct.
func (f Frame) Merge(next Frame) Frame {
	if len(next.Cells) == 0 {
		return Frame{Cells: f.Cells, Cursor: next.Cursor}
	}

	at := make(map[[2]int]int, len(f.Cells)+len(next.Cells))
	cells := make([]Cell, 0, len(f.Cells)+len(next.Cells))

	for _, c := range append(append([]Cell{}, f.Cells...), next.Cells...) {
		key := [2]int{c.X, c.Y}
		if i, seen := at[key]; seen {
			cells[i] = c
			continue
		}
		at[key] = len(cells)
		cells = append(cells, c)
	}

	return Frame{Cells: cells, Cursor: next.Cursor}
}
```

- [ ] **Step 4: Run the tests and watch them pass**

Run: `go test ./internal/screen/ -v`
Expected: all PASS.

- [ ] **Step 5: Prove the survival test bites**

Change `Merge` to `return next`. Run `go test ./internal/screen/ -run
TestMergeKeepsCells` and confirm it FAILS with `merge produced 1 cells, want
2`. Restore it.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/screen/
git add internal/screen/
git commit -m "Fold two waiting frames into one without losing a cell

A reader that falls behind gets one frame rather than a queue, and this
is the operation that makes that safe. Both frames are differences
against the same picture — the one the reader is still showing, having
applied neither — so taking the later cell at each position draws the
same screen as playing them in order.

The test says exactly that: it plays both onto a grid, plays the merge
onto another, and compares the grids. Comparing cell lists would pass
for an implementation that reordered them and fail for one that was
right.

Discarding the waiting frame instead would be the bug. A cell it carried
that the new frame does not touch would stay stale on screen for good."
```

---

### Task 4: Resize, and coming back to a terminal that has moved on

**Files:**
- Modify: `internal/screen/frame.go` (`SetSize`)
- Test: `internal/screen/frame_test.go`

**Interfaces:**
- Consumes: `Attach`, `Show`, `Sync`, `SetSize` from Tasks 1–2.
- Produces: no new names. `SetSize` gains its event.

- [ ] **Step 1: Write the failing test**

Append to `internal/screen/frame_test.go`:

```go
// The interface relayouts on tcell's resize event. Resizing the buffer
// without posting one leaves every widget at the old geometry.
func TestSetSizePostsResizeAfterResizing(t *testing.T) {
	s, _ := attached(t, 20, 5)

	s.SetSize(40, 10)

	ev, ok := s.PollEvent().(*tcell.EventResize)
	if !ok {
		t.Fatalf("PollEvent returned %T, want *tcell.EventResize", ev)
	}
	if w, h := ev.Size(); w != 40 || h != 10 {
		t.Errorf("event size = %dx%d, want 40x10", w, h)
	}
	if w, h := s.Size(); w != 40 || h != 10 {
		t.Errorf("screen size = %dx%d, want 40x10; the event must not arrive before the size", w, h)
	}
}

// Drawing goes on while detached and clears the dirty flags as it goes. A
// client that comes back must be sent everything, not the nothing that is
// left over.
func TestReattachingRepaintsEverything(t *testing.T) {
	s, r := attached(t, 20, 5)
	s.Sync()

	s.Detach()
	s.SetContent(3, 3, 'q', nil, tcell.StyleDefault)
	s.Show() // nobody listening; the dirty flag is spent here

	r.frames = nil
	s.Attach(r)

	f := last(t, r)
	if len(f.Cells) != 20*5 {
		t.Fatalf("re-attaching sent %d cells, want the whole screen (%d)", len(f.Cells), 20*5)
	}
	if c := find(t, f, 3, 3); c.Main != 'q' {
		t.Errorf("cell drawn while detached came back as %q, want 'q'", c.Main)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/screen/ -run TestSetSizePostsResize`
Expected: FAIL — `PollEvent returned <nil>, want *tcell.EventResize`. (The
re-attach test already passes, because Task 2 wrote `Attach` with the
invalidate. Keep it: it is the regression guard for that line, and Step 5
proves it bites.)

- [ ] **Step 3: Write the implementation**

Replace `SetSize` in `internal/screen/frame.go`:

```go
// SetSize is what a client resize arrives as.
//
// The order is load-bearing. tview relayouts when it sees the resize event
// and asks the screen how big it is while doing so; posting the event before
// the buffer has been resized lays the interface out at the size it just
// stopped being.
func (s *Screen) SetSize(w, h int) {
	s.mu.Lock()
	current, currentH := s.buf.Size()
	if w == current && h == currentH {
		s.mu.Unlock()
		return
	}
	s.buf.Resize(w, h)
	s.mu.Unlock()

	s.PostEventWait(tcell.NewEventResize(w, h))
}
```

- [ ] **Step 4: Run the tests and watch them pass**

Run: `go test ./internal/screen/ -v`
Expected: all PASS.

- [ ] **Step 5: Prove the re-attach test bites**

Change `s.Sync()` at the end of `Attach` to `s.Show()`. Run `go test
./internal/screen/ -run TestReattachingRepaints` and confirm it FAILS with
`re-attaching sent 1 cells` — the one cell drawn while detached is the only
dirty flag left, and the rest of the screen never arrives. Restore it. This is
the subtlest bug in the design; the test is the only thing standing in front
of it.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/screen/
git add internal/screen/
git commit -m "Resize the buffer before saying it resized

tview relayouts when it sees tcell's resize event and asks the screen
how big it is while doing so. Posting the event first lays the interface
out at the size it has just stopped being, which shows up as a frame
drawn to the old geometry and never corrected.

The other test here guards the line in Attach that was already written:
Show spends dirty flags whether or not anyone is listening, so a screen
drawn while detached has no record of what changed, and re-attaching
without invalidating sends an empty frame to a terminal showing
something else entirely."
```

---

### Task 5: The wire format

**Files:**
- Create: `internal/proto/proto.go`, `internal/proto/codec.go`
- Test: `internal/proto/proto_test.go`

**Interfaces:**
- Consumes: `screen.Frame`, `screen.Caps` from Tasks 1–2.
- Produces:
  - `type Kind uint8` with the constants below
  - `type ToServer struct` / `type ToClient struct` and their payload types
  - `func NewEncoder(w io.Writer) *Encoder`, `func (e *Encoder) ToServer(ToServer) error`, `func (e *Encoder) ToClient(ToClient) error`
  - `func NewDecoder(r io.Reader) *Decoder`, `func (d *Decoder) ToServer() (ToServer, error)`, `func (d *Decoder) ToClient() (ToClient, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/proto/proto_test.go`:

```go
package proto_test

import (
	"net"
	"testing"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// A style or a combining rune lost on the wire is a character drawn wrongly
// with nothing to say it was ever right.
func TestFrameSurvivesTheWire(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	want := proto.ToClient{
		Kind: proto.KindFrame,
		Frame: &screen.Frame{
			Cells: []screen.Cell{
				{
					X: 3, Y: 4, Main: '한',
					Comb:  []rune{0x0301},
					Style: screen.StyleFrom(tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)),
					Width: 2,
				},
			},
			Cursor: screen.Cursor{X: 3, Y: 4, Visible: true},
		},
	}

	errc := make(chan error, 1)
	go func() { errc <- proto.NewEncoder(server).ToClient(want) }()

	got, err := proto.NewDecoder(client).ToClient()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("encode: %v", err)
	}

	if got.Kind != proto.KindFrame {
		t.Fatalf("Kind = %v, want KindFrame", got.Kind)
	}
	if got.Frame == nil {
		t.Fatal("Frame is nil")
	}
	if len(got.Frame.Cells) != 1 {
		t.Fatalf("got %d cells, want 1", len(got.Frame.Cells))
	}
	c := got.Frame.Cells[0]
	if c.Main != '한' || c.Width != 2 {
		t.Errorf("cell = %q width %d, want '한' width 2", c.Main, c.Width)
	}
	if len(c.Comb) != 1 || c.Comb[0] != 0x0301 {
		t.Errorf("combining = %v, want [0x0301]", c.Comb)
	}
	if c.Style != want.Frame.Cells[0].Style {
		t.Error("style did not survive the wire")
	}
	if fg, _, _ := c.Style.Tcell().Decompose(); fg != tcell.ColorRed {
		t.Errorf("foreground came back as %v, want red", fg)
	}
	if got.Frame.Cursor != want.Frame.Cursor {
		t.Errorf("Cursor = %+v, want %+v", got.Frame.Cursor, want.Frame.Cursor)
	}
}

// A key that changes on the way up is a key the user did not press.
func TestKeyEventSurvivesTheWire(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	want := proto.ToServer{
		Kind: proto.KindKey,
		Key:  &proto.Key{Key: tcell.KeyRune, Rune: '가', Mods: tcell.ModAlt},
	}

	errc := make(chan error, 1)
	go func() { errc <- proto.NewEncoder(client).ToServer(want) }()

	got, err := proto.NewDecoder(server).ToServer()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("encode: %v", err)
	}

	if got.Kind != proto.KindKey || got.Key == nil {
		t.Fatalf("got %+v, want a key", got)
	}
	if *got.Key != *want.Key {
		t.Errorf("Key = %+v, want %+v", *got.Key, *want.Key)
	}
}

// The handshake carries what the screen will report as its capabilities. A
// colour depth mangled here is a palette degraded for the whole session.
func TestHelloCarriesCaps(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	want := proto.ToServer{
		Kind: proto.KindHello,
		Hello: &proto.Hello{
			Version: "0.7.0",
			Caps: screen.Caps{
				Width: 162, Height: 45,
				Colors: 16777216, CharacterSet: "UTF-8", HasMouse: true,
			},
			WorkDir:    "/home/x/reports",
			DataSource: "prod-ro",
			PID:        4242,
		},
	}

	errc := make(chan error, 1)
	go func() { errc <- proto.NewEncoder(client).ToServer(want) }()

	got, err := proto.NewDecoder(server).ToServer()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("encode: %v", err)
	}

	if got.Hello == nil {
		t.Fatal("Hello is nil")
	}
	if *got.Hello != *want.Hello {
		t.Errorf("Hello = %+v, want %+v", *got.Hello, *want.Hello)
	}
}

// Several messages share one connection; gob sends its type information once
// and a second message must still arrive whole.
func TestSecondMessageOnTheSameConnection(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	enc := proto.NewEncoder(client)
	go func() {
		_ = enc.ToServer(proto.ToServer{Kind: proto.KindKey, Key: &proto.Key{Key: tcell.KeyRune, Rune: 'a'}})
		_ = enc.ToServer(proto.ToServer{Kind: proto.KindDetach})
	}()

	dec := proto.NewDecoder(server)
	if first, err := dec.ToServer(); err != nil || first.Kind != proto.KindKey {
		t.Fatalf("first = %+v, err = %v", first, err)
	}
	second, err := dec.ToServer()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Kind != proto.KindDetach {
		t.Errorf("second Kind = %v, want KindDetach", second.Kind)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/proto/`
Expected: build failure — the package does not exist.

- [ ] **Step 3: Write the messages**

Create `internal/proto/proto.go`:

```go
// Package proto is what the interface and the terminal say to each other when
// they are in different processes.
//
// It knows nothing about sockets: everything here works over an io.Reader and
// an io.Writer, which is what lets net.Pipe test the whole of it.
//
// The payload types come from internal/screen rather than being redeclared
// here. The screen holds the concept and this package only carries it; a wire
// format that owned the cell type would drag the screen along every time the
// transport changed.
package proto

import (
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// Kind names the payload. Messages are an envelope with a Kind and one
// pointer per payload rather than registered interfaces, so that the whole
// protocol reads in one file and there is no registration to forget.
type Kind uint8

const (
	KindNone Kind = iota

	// Client to server.
	KindHello
	KindKey
	KindMouse
	KindResize
	KindPaste
	KindFocus
	KindClipboardData
	KindDetach

	// Server to client.
	KindWelcome
	KindReject
	KindFrame
	KindSetClipboard
	KindRequestClipboard
	KindTitle
	KindBell
	KindBye
)

// Hello opens a connection.
type Hello struct {
	Version string
	Caps    screen.Caps
	// WorkDir is --dir, empty when the session starts unattached.
	WorkDir string
	// DataSource is the name the client was asked to open, empty when none
	// was named.
	DataSource string
	PID        int
}

// Welcome accepts a Hello.
//
// Warnings are what the server would have written to stderr while starting
// the session — a schema cache it could not open, a worktree that no longer
// exists. They travel here because in the server they would otherwise land in
// a log nobody reads, and completion would simply look broken.
type Welcome struct {
	Version  string
	Warnings []string
}

// Reject refuses a Hello and says why in a sentence meant for a person.
type Reject struct {
	Reason string
}

// Bye ends a connection from the server's side. Replaced means another client
// attached; Quit means the session itself has ended.
type Bye struct {
	Reason string
}

const (
	ByeQuit     = "quit"
	ByeReplaced = "replaced"
)

// Key is tcell.EventKey flattened. The event type carries a timestamp and
// unexported state that mean nothing in another process.
type Key struct {
	Key  tcell.Key
	Rune rune
	Mods tcell.ModMask
}

// Mouse is tcell.EventMouse flattened, for the same reason.
type Mouse struct {
	X, Y    int
	Buttons tcell.ButtonMask
	Mods    tcell.ModMask
}

// Size is a resize from the client.
type Size struct {
	Width, Height int
}

// Paste brackets a paste. Start false is the end of one.
type Paste struct {
	Start bool
}

// Focus reports the terminal gaining or losing focus.
type Focus struct {
	Focused bool
}

// ToServer is anything the client says.
type ToServer struct {
	Kind Kind

	Hello     *Hello
	Key       *Key
	Mouse     *Mouse
	Resize    *Size
	Paste     *Paste
	Focus     *Focus
	Clipboard []byte
}

// ToClient is anything the server says.
type ToClient struct {
	Kind Kind

	Welcome   *Welcome
	Reject    *Reject
	Frame     *screen.Frame
	Clipboard []byte
	Title     string
	Bye       *Bye
}
```

- [ ] **Step 4: Write the codec**

Create `internal/proto/codec.go`:

```go
package proto

import (
	"encoding/gob"
	"io"
)

// Encoder writes messages to a connection.
//
// gob rather than JSON because a frame carries runes, combining runes and a
// tcell.Style, and because its stream encoder sends type information once per
// connection instead of once per frame. Both ends are dv, so gob being a Go
// format costs nothing.
type Encoder struct {
	enc *gob.Encoder
}

func NewEncoder(w io.Writer) *Encoder { return &Encoder{enc: gob.NewEncoder(w)} }

func (e *Encoder) ToServer(m ToServer) error { return e.enc.Encode(m) }
func (e *Encoder) ToClient(m ToClient) error { return e.enc.Encode(m) }

// Decoder reads messages from a connection.
type Decoder struct {
	dec *gob.Decoder
}

func NewDecoder(r io.Reader) *Decoder { return &Decoder{dec: gob.NewDecoder(r)} }

func (d *Decoder) ToServer() (ToServer, error) {
	var m ToServer
	err := d.dec.Decode(&m)
	return m, err
}

func (d *Decoder) ToClient() (ToClient, error) {
	var m ToClient
	err := d.dec.Decode(&m)
	return m, err
}
```

- [ ] **Step 5: Run the tests and watch them pass**

Run: `go test ./internal/proto/ -v`
Expected: all four PASS.

The style survives because `screen.Cell` carries a `screen.Style` and not a
`tcell.Style`. `tcell.Style` has no exported fields, so gob would write it as
a zero value and every cell would arrive colourless with nothing failing to
build. Task 2's `TestStyleSurvivesFlattening` guards the conversion; this test
guards the wire.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/proto/
git add internal/proto/
git commit -m "Say it over an io.Writer, not a socket

The whole protocol works over an io.Reader and an io.Writer, so net.Pipe
tests all of it and nothing here has to know a socket exists.

gob rather than JSON: a frame carries runes, combining runes and a
tcell.Style, and gob's stream encoder sends the type information once
per connection instead of once per frame. Both ends are dv, so its being
a Go format costs nothing.

The messages are an envelope with a Kind and one pointer per payload
rather than registered interfaces. The protocol then reads in one file,
and there is no registration to forget at the moment a new message type
first goes out."
```

---

### Task 6: The one-deep frame queue

**Files:**
- Create: `internal/proto/queue.go`
- Test: `internal/proto/queue_test.go`

**Interfaces:**
- Consumes: `screen.Frame` and `Frame.Merge` from Tasks 2–3.
- Produces:
  - `func NewFrameQueue() *FrameQueue`
  - `func (q *FrameQueue) Put(screen.Frame)` — never blocks
  - `func (q *FrameQueue) Take(ctx context.Context) (screen.Frame, bool)`
  - `func (q *FrameQueue) Close()`

- [ ] **Step 1: Write the failing test**

Create `internal/proto/queue_test.go`:

```go
package proto_test

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
)

// The goroutine that draws must not learn how slow the terminal is. If it
// could block here, a wedged client would stop dv reading rows from the
// database — and MySQL will not take another statement until the result set
// is drained, so cancellation and schema browsing would stop with it.
func TestPutNeverBlocks(t *testing.T) {
	q := proto.NewFrameQueue()
	defer q.Close()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			q.Put(screen.Frame{Cells: []screen.Cell{{X: i % 40, Y: 0, Main: 'x', Width: 1}}})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Put was still going after two seconds with nobody taking")
	}
}

// What piles up must be one frame, not a queue of them.
func TestWaitingFramesAreMerged(t *testing.T) {
	q := proto.NewFrameQueue()
	defer q.Close()

	q.Put(screen.Frame{Cells: []screen.Cell{
		{X: 1, Y: 0, Main: 'a', Width: 1},
		{X: 5, Y: 2, Main: 'b', Width: 1},
	}})
	q.Put(screen.Frame{Cells: []screen.Cell{
		{X: 1, Y: 0, Main: 'z', Width: 1},
	}})

	got, ok := q.Take(context.Background())
	if !ok {
		t.Fatal("Take returned nothing")
	}
	if len(got.Cells) != 2 {
		t.Fatalf("took %d cells, want 2: the cell only the first frame touched must survive", len(got.Cells))
	}

	byPos := map[[2]int]rune{}
	for _, c := range got.Cells {
		byPos[[2]int{c.X, c.Y}] = c.Main
	}
	if byPos[[2]int{1, 0}] != 'z' {
		t.Errorf("at 1,0 = %q, want 'z': the later value wins", byPos[[2]int{1, 0}])
	}
	if byPos[[2]int{5, 2}] != 'b' {
		t.Errorf("at 5,2 = %q, want 'b': it was never overwritten", byPos[[2]int{5, 2}])
	}

	if _, ok := q.Take(withTimeout(t, 50*time.Millisecond)); ok {
		t.Error("a second frame was waiting; the two should have been merged into one")
	}
}

// Take must return when the connection ends, or the writer goroutine outlives
// the client it was writing to.
func TestTakeReturnsOnClose(t *testing.T) {
	q := proto.NewFrameQueue()

	done := make(chan bool, 1)
	go func() {
		_, ok := q.Take(context.Background())
		done <- ok
	}()

	q.Close()

	select {
	case ok := <-done:
		if ok {
			t.Error("Take reported a frame after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Take still blocked two seconds after Close")
	}
}

func withTimeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/proto/ -run TestPutNeverBlocks`
Expected: build failure — `proto.NewFrameQueue undefined`.

- [ ] **Step 3: Write the implementation**

Create `internal/proto/queue.go`:

```go
package proto

import (
	"context"
	"sync"

	"github.com/Ahngbeom/datavase/internal/screen"
)

// FrameQueue holds at most one frame for a writer that has fallen behind.
//
// Put never blocks, which is the entire point: it is called from the
// goroutine that draws the interface, and that goroutine is also the one
// reading rows off the connection. A slow, suspended or dead terminal that
// could stall it would stall the statement too — and because MySQL will not
// accept another statement until the result set is drained, cancellation and
// schema browsing would stall with it.
//
// When a frame is already waiting the two are merged rather than one being
// dropped. See screen.Frame.Merge for why that is sound, and for what
// dropping would cost.
type FrameQueue struct {
	mu      sync.Mutex
	frame   screen.Frame
	waiting bool
	ready   chan struct{}
	done    chan struct{}
	once    sync.Once
}

func NewFrameQueue() *FrameQueue {
	return &FrameQueue{
		ready: make(chan struct{}, 1),
		done:  make(chan struct{}),
	}
}

// Put leaves f for the next Take, merging it with anything already waiting.
func (q *FrameQueue) Put(f screen.Frame) {
	q.mu.Lock()
	if q.waiting {
		q.frame = q.frame.Merge(f)
	} else {
		q.frame = f
		q.waiting = true
	}
	q.mu.Unlock()

	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// Take blocks for a frame. It reports false when the queue is closed or ctx
// ends, which is how the writer goroutine learns to stop.
func (q *FrameQueue) Take(ctx context.Context) (screen.Frame, bool) {
	for {
		q.mu.Lock()
		if q.waiting {
			f := q.frame
			q.frame = screen.Frame{}
			q.waiting = false
			q.mu.Unlock()
			return f, true
		}
		q.mu.Unlock()

		select {
		case <-q.ready:
		case <-q.done:
			return screen.Frame{}, false
		case <-ctx.Done():
			return screen.Frame{}, false
		}
	}
}

// Close releases anything blocked in Take. It is safe to call twice.
func (q *FrameQueue) Close() {
	q.once.Do(func() { close(q.done) })
}
```

- [ ] **Step 4: Run the tests and watch them pass**

Run: `go test ./internal/proto/ -v -race`
Expected: all PASS with no race reported. `-race` matters here: `Put` and
`Take` are the only two functions in this plan called from different
goroutines by design.

- [ ] **Step 5: Prove the merge test bites**

In `Put`, replace the merge branch with `q.frame = f`. Run `go test
./internal/proto/ -run TestWaitingFramesAreMerged` and confirm it FAILS with
`took 1 cells, want 2`. Restore it.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test -race ./internal/...
git add internal/proto/
git commit -m "Keep one frame for a writer that has fallen behind

Put is called from the goroutine that draws, and that goroutine is also
the one reading rows off the connection. If it could block on a slow,
suspended or dead terminal it would stall the statement — and MySQL will
not take another statement until the result set is drained, so
cancellation and schema browsing would stall with it. So Put never
blocks, and what piles up is one frame rather than a queue of them.

Merging rather than dropping is what makes that safe. A cell the waiting
frame carried and the new one does not would otherwise stay stale on the
terminal for good."
```

---

## What this leaves

Two packages, tested, used by nothing. `make test` covers them and neither
needs a terminal, a socket or a database.

Part 2 builds `internal/daemon` and `internal/attach` on top: the server that
owns `ui.App` and draws it into a `screen.Screen`, the client that owns the
real terminal, the socket lifecycle, and `ActionDetach`. Part 3 adds
`internal/snapshot` and `dv status`.

Nothing in this plan changes how `dv` behaves. The first commit that does is
in Part 2.
