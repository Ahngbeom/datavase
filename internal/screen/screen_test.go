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
