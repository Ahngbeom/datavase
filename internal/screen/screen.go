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
