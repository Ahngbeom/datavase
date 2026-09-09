package ui

import "testing"

// The list every dialog shows has no headings, and the arrows there must
// keep behaving exactly as they did.
func TestSteppingThroughAListWithoutHeadingsIsUnchanged(t *testing.T) {
	choice := func() searchItem { return searchItem{primary: "row", accept: func() {}} }
	items := []searchItem{choice(), choice(), choice()}

	if got := nextChoice(items, 0); got != 1 {
		t.Errorf("Down from 0 = %d, want 1", got)
	}
	// Down from the last wraps, which is what tview's list has always done.
	if got := nextChoice(items, 2); got != 0 {
		t.Errorf("Down from the last row = %d, want it to wrap to 0", got)
	}
	if got := prevChoice(items, 1); got != 0 {
		t.Errorf("Up from 1 = %d, want 0", got)
	}
	// Up from the first does not wrap; the caller hands typing back instead.
	if got := prevChoice(items, 0); got != -1 {
		t.Errorf("Up from the first row = %d, want -1", got)
	}
}

// "no matching command" is a row with nothing to run, and the arrows treat it
// as what it is: there is nowhere to step, rather than a row to highlight.
func TestAListOfNothingButAMessageHasNowhereToStep(t *testing.T) {
	items := []searchItem{message("no matching command", "press Escape to close")}

	if got := firstChoice(items); got != -1 {
		t.Errorf("firstChoice = %d, want -1", got)
	}
	if got := nextChoice(items, 0); got != -1 {
		t.Errorf("nextChoice = %d, want -1", got)
	}
	if got := prevChoice(items, 0); got != -1 {
		t.Errorf("prevChoice = %d, want -1", got)
	}
}
