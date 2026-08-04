package ui

import "testing"

// isHeading reports whether a row names a group rather than offering a choice.
func isHeading(it searchItem) bool { return it.accept == nil }

// The arrows are walked over the palette's own rows rather than a fixture: a
// list built here would only prove the walk works on the list it was given,
// and the grouping it has to survive is the one the palette actually produces.
func paletteRowsForBrowsing() []searchItem {
	return paletteItems("", func(command) func() { return func() {} })
}

// Walking the whole list is the point. A test that presses Down a fixed number
// of times lands wherever the first group's size happens to put it, and stops
// testing the boundary the moment a command is added to that group.
func TestSteppingDownNeverLandsOnAHeading(t *testing.T) {
	items := paletteRowsForBrowsing()

	at := firstChoice(items)
	if at < 0 {
		t.Fatal("the palette offers nothing to step through")
	}
	if isHeading(items[at]) {
		t.Fatal("the list opens with the highlight on a heading")
	}

	// One full circuit, so the wrap at the end is walked too — that is where
	// Down lands back on row zero, which is a heading.
	for step := 0; step < len(items); step++ {
		next := nextChoice(items, at)
		if next < 0 {
			t.Fatalf("Down had nowhere to go from row %d of %d", at, len(items))
		}
		if isHeading(items[next]) {
			t.Fatalf("Down from row %d landed on the heading %q", at, items[next].primary)
		}
		at = next
	}
}

// Up is the same walk backwards, and additionally has to run out rather than
// wrap: the caller hands typing back when it does.
func TestSteppingUpNeverLandsOnAHeadingAndRunsOut(t *testing.T) {
	items := paletteRowsForBrowsing()

	at := len(items) - 1
	for {
		prev := prevChoice(items, at)
		if prev < 0 {
			break
		}
		if isHeading(items[prev]) {
			t.Fatalf("Up from row %d landed on the heading %q", at, items[prev].primary)
		}
		if prev >= at {
			t.Fatalf("Up from row %d went to row %d rather than upwards", at, prev)
		}
		at = prev
	}

	// Running out has to happen at the topmost command, not somewhere in the
	// middle: this is the row from which Up returns to the search field.
	if at != firstChoice(items) {
		t.Errorf("Up ran out at row %d, want the first command at row %d", at, firstChoice(items))
	}
}

// The list every other dialog shows has no headings at all, and the arrows
// there must keep behaving exactly as they did.
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
