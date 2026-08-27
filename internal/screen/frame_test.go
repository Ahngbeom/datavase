package screen_test

import (
	"sync"
	"testing"
	"time"

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

func (r *recorder) Frame(f screen.Frame)  { r.frames = append(r.frames, f) }
func (r *recorder) SetTitle(t string)     { r.titles = append(r.titles, t) }
func (r *recorder) SetClipboard(b []byte) { r.clipboard = append(r.clipboard, b) }
func (r *recorder) RequestClipboard()     { r.requests++ }
func (r *recorder) Bell()                 { r.bells++ }

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

// A frame already handed to a reader must not be corrupted by a later one:
// the zero-cell fast path used to return a frame sharing the receiver's
// backing array, so mutating what Merge returned changed what an earlier
// reader was still holding.
func TestMergeZeroCellFastPathDoesNotAliasTheReceiver(t *testing.T) {
	waiting := screen.Frame{Cells: []screen.Cell{{X: 5, Y: 2, Main: 'b', Width: 1}}}

	got := waiting.Merge(screen.Frame{})
	got.Cells[0].Main = 'z'

	if waiting.Cells[0].Main != 'b' {
		t.Errorf("mutating the merged frame changed the receiver's cell to %q", waiting.Cells[0].Main)
	}
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

// The interface relayouts when tcell's resize event arrives and asks the
// screen how big it is while doing so. An event that can be seen before the
// buffer has been resized lays every widget out at the size the terminal has
// just stopped being.
func TestSetSizeResizesBeforeTheEventCanBeSeen(t *testing.T) {
	s, _ := attached(t, 20, 5)

	// A full event queue pins SetSize inside PostEventWait, which is the only
	// moment the order is observable at all: the new size has to be readable
	// while the event is still stuck behind the backlog.
	for s.PostEvent(tcell.NewEventInterrupt(nil)) == nil {
	}

	resized := make(chan struct{})
	go func() {
		s.SetSize(40, 10)
		close(resized)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if w, h := s.Size(); w == 40 && h == 10 {
			break
		}
		if time.Now().After(deadline) {
			w, h := s.Size()
			t.Fatalf("screen size = %dx%d while the resize event was already queued; tview would relayout at the old geometry", w, h)
		}
		time.Sleep(time.Millisecond)
	}

	// PollEvent blocks once the backlog is drained, so a SetSize that stopped
	// posting the resize event at all must not hang the test: it has to fail
	// by name instead of surfacing as a timeout panic.
	found := make(chan *tcell.EventResize, 1)
	go func() {
		for {
			ev, ok := s.PollEvent().(*tcell.EventResize)
			if !ok {
				continue // the backlog this test queued up
			}
			found <- ev
			return
		}
	}()

	select {
	case ev := <-found:
		if w, h := ev.Size(); w != 40 || h != 10 {
			t.Errorf("event size = %dx%d, want 40x10", w, h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no resize event arrived within 2s; SetSize did not post one")
	}
	<-resized
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

// blockingSink holds the first frame it is handed until it is released, which
// is what makes the delivery of a second frame observable.
type blockingSink struct {
	mu      sync.Mutex
	frames  []screen.Frame
	started bool
	entered chan struct{}
	release chan struct{}
}

func (b *blockingSink) Frame(f screen.Frame) {
	b.mu.Lock()
	first := !b.started
	b.started = true
	b.mu.Unlock()

	if first {
		close(b.entered)
		<-b.release
	}

	b.mu.Lock()
	b.frames = append(b.frames, f)
	b.mu.Unlock()
}

func (b *blockingSink) SetTitle(string)     {}
func (b *blockingSink) SetClipboard([]byte) {}
func (b *blockingSink) RequestClipboard()   {}
func (b *blockingSink) Bell()               {}

// Attaching sends the whole screen from whichever goroutine took the client;
// the interface goes on drawing on its own. A draw that finishes while that
// repaint is still in the sink must not pass it, or the terminal applies the
// newer cells and then has the older full frame paint them back to what they
// were — stale until something happens to touch them again.
func TestADrawCannotOvertakeTheRepaintBeforeIt(t *testing.T) {
	s := screen.New(screen.Caps{Width: 20, Height: 5, Colors: 256, CharacterSet: "UTF-8"})
	b := &blockingSink{entered: make(chan struct{}), release: make(chan struct{})}

	attaching := make(chan struct{})
	go func() {
		s.Attach(b)
		close(attaching)
	}()
	<-b.entered // the repaint is inside the sink and has not returned

	s.SetContent(2, 1, 'x', nil, tcell.StyleDefault)
	drawn := make(chan struct{})
	go func() {
		s.Show()
		close(drawn)
	}()

	select {
	case <-drawn:
		t.Fatal("a draw reached the sink while the repaint before it was still being delivered")
	case <-time.After(250 * time.Millisecond):
	}

	close(b.release)
	<-attaching
	<-drawn

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.frames) != 2 {
		t.Fatalf("sink got %d frames, want the repaint and then the draw", len(b.frames))
	}
	if n := len(b.frames[0].Cells); n != 20*5 {
		t.Errorf("first frame carried %d cells, want the whole screen (%d)", n, 20*5)
	}
	if n := len(b.frames[1].Cells); n != 1 {
		t.Errorf("second frame carried %d cells, want only the one that changed", n)
	}
}
