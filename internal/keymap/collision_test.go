package keymap

import (
	"testing"
)

// Two actions sharing one binding is silent: the map is keyed by binding, so
// the second registration simply wins and the first action becomes
// unreachable. Nothing else in the suite would notice.
func TestNoTwoActionsShareABinding(t *testing.T) {
	m := Default()

	owner := make(map[Binding]Action)
	for _, action := range AllActions() {
		for _, b := range m.Bindings(action) {
			if previous, taken := owner[b]; taken && previous != action {
				t.Errorf("%v and %v are both bound to %s", previous, action, b.Label(false))
			}
			owner[b] = action
		}
	}
}
