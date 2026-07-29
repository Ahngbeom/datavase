package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
)

// The help screen is built from an explicit list of groups, so an action
// added to the keymap and forgotten here has a key that works and no way to
// discover it. This is the check that stops that happening quietly.
func TestEveryActionAppearsOnTheHelpScreen(t *testing.T) {
	seen := make(map[keymap.Action]int)
	for _, group := range helpGroups {
		for _, action := range group.actions {
			seen[action]++
		}
	}

	for _, action := range keymap.AllActions() {
		switch seen[action] {
		case 1:
		case 0:
			t.Errorf("%s is bindable but is not on the help screen", action)
		default:
			t.Errorf("%s appears on the help screen %d times", action, seen[action])
		}
	}
}
