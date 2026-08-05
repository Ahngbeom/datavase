package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// The card closes on any key and then does what that key names, and it offers
// Enter and Escape as the way out. Those are only the same thing while neither
// is bound to anything: a preset that bound Enter would make the card's own
// closing instruction run something else, on the one screen whose whole purpose
// is telling a new user what the keys do.
func TestTheCardsWayOutIsBoundToNothing(t *testing.T) {
	wayOut := []struct {
		name string
		ev   *tcell.EventKey
	}{
		{"Enter", tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)},
		{"Escape", tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)},
	}

	for _, preset := range keymap.Presets() {
		m, err := keymap.ForPreset(preset)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}

		for _, key := range wayOut {
			if action := m.Lookup(key.ev); action != keymap.ActionNone {
				t.Errorf("the %s preset binds %s to %s, and the welcome card offers it as the way out",
					preset, key.name, action)
			}
		}
	}
}
