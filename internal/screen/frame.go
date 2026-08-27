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

func (s *Screen) SetTitle(string)     {}
func (s *Screen) SetClipboard([]byte) {}
func (s *Screen) GetClipboard()       {}
func (s *Screen) Beep() error         { return nil }
