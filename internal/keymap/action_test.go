package keymap

import "testing"

// A Go map answers false for a key it does not hold, so an action added
// without a decision would silently claim dv teaches it nothing about it.
// The classification has to be made, not defaulted.
func TestEveryActionIsClassified(t *testing.T) {
	for _, a := range AllActions() {
		if _, ok := familiar[a]; !ok {
			t.Errorf("action %q is in no familiarity class; add it to familiar", a)
		}
	}
}

// The classification counts what a user must be taught. If it drifts, the
// key reference silently starts teaching the wrong list.
func TestTheKeyReferenceDoesNotSilentlyStartTeachingTheWrongList(t *testing.T) {
	var known, ours int
	for _, a := range AllActions() {
		if a.Familiar() {
			known++
			continue
		}
		ours++
	}

	if known != 16 || ours != 30 {
		t.Errorf("classified %d familiar and %d dv's own, want 16 and 30", known, ours)
	}
}

// These five look conventional and are not, or look unusual and are. Each is
// a judgement the design made explicitly; a later change to any of them
// should be a decision rather than a drift.
func TestTheJudgementCallsHold(t *testing.T) {
	for _, tt := range []struct {
		action   Action
		familiar bool
		why      string
	}{
		{ActionToggleComment, false, "the chord is conventional but what it does differs between editors"},
		{ActionDuplicateLine, false, "VS Code uses the same chord to select the next occurrence"},
		{ActionDeleteLine, false, "a DataGrip convention, not a universal one"},
		{ActionNextPane, false, "universal in GUI dialogs, not in terminal applications"},
		{ActionPrevPane, false, "universal in GUI dialogs, not in terminal applications"},
		{ActionCopyOrCancel, false, "with nothing selected ⌘C cancels the running statement"},
		{ActionSaveFile, true, "saving is saving"},
		{ActionFind, true, "finding is finding; that it also searches results is a description, not a class"},
		{ActionSearchHistory, false, "searching history is not what that chord means elsewhere"},
	} {
		if got := tt.action.Familiar(); got != tt.familiar {
			t.Errorf("%q.Familiar() = %v, want %v — %s", tt.action, got, tt.familiar, tt.why)
		}
	}
}
