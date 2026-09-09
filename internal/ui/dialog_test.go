package ui

import (
	"strings"
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

// "Start here" repeats keys that appear again further down, which is
// deliberate: the reference has to be complete and the opening has to be
// short. What it must not do is advertise an action that no longer exists.
func TestStartHereOnlyNamesActionsTheReferenceAlsoCarries(t *testing.T) {
	inGroups := make(map[keymap.Action]bool)
	for _, group := range helpGroups {
		for _, action := range group.actions {
			inGroups[action] = true
		}
	}

	for _, action := range startHere {
		if !inGroups[action] {
			t.Errorf("Start here offers %s, which the key reference does not list", action)
		}
	}
}

// Five is the whole point. A "start here" that grows into a second reference
// is one nobody reads either.
func TestStartHereStaysShort(t *testing.T) {
	const most = 6

	if len(startHere) > most {
		t.Errorf("Start here lists %d keys, want at most %d", len(startHere), most)
	}
}

// This reads the rendered screen rather than the group list, so a mistake
// that put an action in helpGroups but never rendered it would still be
// caught.
func TestEveryActionIsStillOnTheRenderedHelpScreen(t *testing.T) {
	body := helpReference(keymap.Default())

	for _, action := range keymap.AllActions() {
		if n := strings.Count(body, action.Describe()); n != 1 {
			t.Errorf("%s appears %d times in the rendered reference, want 1", action, n)
		}
	}
}
