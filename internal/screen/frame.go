package screen

import "github.com/gdamore/tcell/v2"

// Style is tcell.Style in a form an encoder can carry.
//
// tcell.Style has no exported fields, and encoding/gob refuses a type that
// has none: "gob: type tcell.Style has no exported fields". A cell carrying
// one would not travel at all — the encode fails before a byte is written,
// and the reader waits for a frame that never comes. Nothing catches that at
// compile time, which is why this type exists and why a test pins the round
// trip.
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
//
// No method may block. SetTitle, SetClipboard, GetClipboard and Beep are
// called from the goroutine that draws the interface, which is also the
// goroutine reading rows off the database connection; MySQL will not accept
// another statement until that result set is drained, so a sink that waits on
// a wedged terminal stalls the statement, its cancellation and the schema
// browser with it. Frame is called from that same goroutine for an ordinary
// draw, and from whichever goroutine attached the sink for the repaint that
// Attach sends. Frames have a bounded path for this in proto.FrameQueue — the
// other four methods have nothing behind them, and whatever implements them
// owns that.
//
// An implementation must not call back into the screen — Detach, Show or Sync
// — from inside Frame. Delivery is serialised behind a single lock that Frame
// is called under, and Detach, Show and Sync all take that same lock, so a
// call back in from inside Frame blocks forever. "The write failed, drop the
// client" is the most natural thing to reach for in a transport, and it has
// to be done from somewhere other than inside Frame.
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
	// The sink is set and the lock dropped before Sync, which takes deliver
	// for itself; holding it across the repaint would deadlock on it.
	s.mu.Lock()
	s.sink = sink
	s.mu.Unlock()

	s.Sync()
}

// Detach stops emitting. The interface goes on drawing and does not learn
// that nobody is watching.
//
// It waits on deliver so that a frame emit has already built cannot land in a
// sink this call has dropped.
func (s *Screen) Detach() {
	s.deliver.Lock()
	defer s.deliver.Unlock()

	s.mu.Lock()
	s.sink = nil
	s.mu.Unlock()
}

// Show clears the dirty flags whether or not a sink is listening, because the
// buffer is tview's and the interface draws on it either way. What that costs
// is the record of everything drawn while detached, which is why Attach
// repaints rather than waiting for the next Show.
func (s *Screen) Show() { s.emit(false) }

// Sync emits every cell the buffer holds, skipping only the column behind a
// wide rune as Show does.
//
// It does not go through the buffer's dirty flags. tcell marks a cell dirty
// by clearing the record of what was last drawn there, which leaves a cell
// nothing was ever written to indistinguishable from a clean one — so an
// invalidated blank screen reports nothing to send. A repaint has to mean
// every cell, because the terminal being repainted for may be showing
// anything at all.
func (s *Screen) Sync() { s.emit(true) }

func (s *Screen) emit(all bool) {
	s.deliver.Lock()
	defer s.deliver.Unlock()

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
		return Frame{Cells: append([]Cell{}, f.Cells...), Cursor: next.Cursor}
	}

	at := make(map[[2]int]int, len(f.Cells)+len(next.Cells))
	cells := make([]Cell, 0, len(f.Cells)+len(next.Cells))

	take := func(from []Cell) {
		for _, c := range from {
			key := [2]int{c.X, c.Y}
			if i, seen := at[key]; seen {
				cells[i] = c
				continue
			}
			at[key] = len(cells)
			cells = append(cells, c)
		}
	}
	take(f.Cells)
	take(next.Cells)

	return Frame{Cells: cells, Cursor: next.Cursor}
}
