package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// The palette is the way to every command that has no key of its own, so it
// is the one thing that must not be reachable only by a chord. A host
// application can claim ⌘⇧A, and Ctrl+Shift+A needs the extended keyboard
// protocol — between them that leaves terminals where the escape hatch itself
// is locked.
func TestThePaletteHasAKeyNoHostCanClaim(t *testing.T) {
	if got := Default().Lookup(key(tcell.KeyF3, tcell.ModNone)); got != ActionCommandPalette {
		t.Errorf("F3 = %v, want ActionCommandPalette", got)
	}

	// And the chord spellings still work where they arrive.
	for _, ev := range []*tcell.EventKey{
		runeKey('a', tcell.ModCtrl|tcell.ModShift),
		runeKey('a', tcell.ModMeta|tcell.ModShift),
	} {
		if got := Default().Lookup(ev); got != ActionCommandPalette {
			t.Errorf("%s = %v, want ActionCommandPalette", ev.Name(), got)
		}
	}
}

// Plain F2 must not have been taken from anything: cancelling is ⌘F2, and a
// stolen key is only noticed when the feature it belonged to stops working.
func TestPlainF2DoesNotDisturbCancel(t *testing.T) {
	if got := Default().Lookup(key(tcell.KeyF2, tcell.ModCtrl)); got != ActionCancel {
		t.Errorf("Ctrl+F2 = %v, want ActionCancel", got)
	}
}

// Two actions sharing one binding is silent: the map is keyed by binding, so
// the second registration simply wins and the first action becomes
// unreachable. Nothing else in the suite would notice.
func TestNoTwoActionsShareABinding(t *testing.T) {
	for _, preset := range Presets() {
		m, err := ForPreset(preset)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}

		owner := make(map[Binding]Action)
		for _, action := range AllActions() {
			for _, b := range m.Bindings(action) {
				if previous, taken := owner[b]; taken && previous != action {
					t.Errorf("%s preset: %v and %v are both bound to %s",
						preset, previous, action, b.Label(false))
				}
				owner[b] = action
			}
		}
	}
}
