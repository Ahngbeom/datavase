package keymap

import (
	"testing"
)

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
