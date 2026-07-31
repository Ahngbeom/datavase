package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// chordMods are the modifiers another application can take before the key ever
// reaches a terminal program: a window manager's shortcut, a terminal's own
// menu, a multiplexer's prefix.
//
// Shift is not among them. A shifted key is a distinct terminfo entry rather
// than a chord, and nothing upstream claims it on its own.
const chordMods = tcell.ModCtrl | tcell.ModMeta | tcell.ModAlt

// reachableWithoutAChord reports whether the map binds the action to a key
// that no host application is in a position to intercept.
func reachableWithoutAChord(m *keymap.Map, a keymap.Action) bool {
	for _, b := range m.Bindings(a) {
		if b.Mods&chordMods == 0 {
			return true
		}
	}
	return false
}

func paletteCoverage() map[keymap.Action]bool {
	covered := make(map[keymap.Action]bool)
	for _, c := range paletteCommands() {
		if c.covers != keymap.ActionNone {
			covered[c.covers] = true
		}
	}
	return covered
}

// The invariant this whole arrangement exists for.
//
// A terminal that keeps ⌘ for its own menus, or a multiplexer whose prefix is
// Ctrl+B, takes those keys before datavase sees them. When that happened, the
// schema tree had no way in at all: ⌘B and Ctrl+B were its only two bindings
// and the palette did not offer it. Every action now has a route that survives
// losing its chord.
func TestEveryActionIsReachableWithoutAChord(t *testing.T) {
	covered := paletteCoverage()

	for _, preset := range keymap.Presets() {
		m, err := keymap.ForPreset(preset)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}

		for _, action := range keymap.AllActions() {
			switch {
			case reachableWithoutAChord(m, action):
			case covered[action]:
			case paletteExempt[action]:
			default:
				t.Errorf("%s preset: %s can only be reached by a chord — bind it to a plain key, "+
					"offer it in the palette, or say in paletteExempt why it does not need either",
					preset, action)
			}
		}
	}
}

// An exemption that stopped being needed is a note that has quietly become
// wrong, and the next reader will believe it.
func TestNoExemptionCoversAnActionThatIsAlreadyReachable(t *testing.T) {
	covered := paletteCoverage()
	m := keymap.Default()

	for action := range paletteExempt {
		if covered[action] {
			t.Errorf("%s is exempt from the palette and also offered by it", action)
		}
		if reachableWithoutAChord(m, action) {
			t.Errorf("%s is exempt as unreachable but has a plain key; drop the exemption", action)
		}
	}
}

// Two commands claiming one action means the palette does the same thing twice
// under two names, and neither reads as the real one.
func TestNoTwoCommandsCoverTheSameAction(t *testing.T) {
	seen := make(map[keymap.Action]string)

	for _, c := range paletteCommands() {
		if c.covers == keymap.ActionNone {
			continue
		}
		if first, taken := seen[c.covers]; taken {
			t.Errorf("%q and %q both perform %s", first, c.name, c.covers)
		}
		seen[c.covers] = c.name
	}
}

// The palette is the route to everything that has no key, so it cannot be the
// one thing you need a key to reach.
func TestThePaletteItselfIsReachableWithoutAChord(t *testing.T) {
	for _, preset := range keymap.Presets() {
		m, err := keymap.ForPreset(preset)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}
		if !reachableWithoutAChord(m, keymap.ActionCommandPalette) {
			t.Errorf("%s preset: the palette is only reachable by a chord", preset)
		}
	}
}
