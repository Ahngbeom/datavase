package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
)

// Mouse reporting disables the terminal's own text selection. Someone who
// copies by dragging must be able to turn this off — and must lose only the
// ways in, never a capability.
func TestTurningTheMouseOffLosesNoCommand(t *testing.T) {
	for _, ctx := range allMenuContexts() {
		entries := menuEntries(paletteCommands(), ctx, func(keymap.Action) string { return "" })
		for _, e := range entries {
			if !namedInPalette(paletteCommands(), e.name) {
				t.Errorf("%q is offered in context %v and nowhere a keyboard can reach", e.name, ctx)
			}
		}
	}
}

func namedInPalette(cmds []command, name string) bool {
	for _, c := range cmds {
		if c.name == name {
			return true
		}
	}
	return false
}
