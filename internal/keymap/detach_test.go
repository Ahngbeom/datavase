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
