package ui

import (
	"strings"
	"testing"
)

// keys stands in for the live key map: the hints name whatever the map says,
// so a rebinding can never leave the header advertising a key that does
// nothing.
func testKeys() affordanceKeys {
	return affordanceKeys{
		run:      "⌘↩",
		runAll:   "⌘⇧↩",
		complete: "^Space",
		history:  "⌘⇧F",
		copy:     "⌘⇧C",
		sort:     "⌘⇧S",
		inspect:  "⌘I",
		row:      "⌘⇧R",
	}
}

// Only the region the keyboard is in speaks. Three regions offering their
// keys at once is a screen shouting, and a reader who has stopped seeing any
// of it learns nothing from the one that mattered.
func TestOnlyTheFocusedRegionOffersItsKeys(t *testing.T) {
	k := testKeys()

	if got := editorAffordance(false, k); got != "" {
		t.Errorf("the unfocused editor said %q, want nothing", got)
	}
	if got := treeAffordance(false, k); got != "" {
		t.Errorf("the unfocused tree said %q, want nothing", got)
	}
	if got := gridAffordance(false, true, k); got != "" {
		t.Errorf("the unfocused grid said %q, want nothing", got)
	}
}

// What the editor can do that is not obvious from looking at it: everything
// except typing.
func TestTheEditorOffersRunningCompletionAndHistory(t *testing.T) {
	got := editorAffordance(true, testKeys())

	for _, want := range []string{"⌘↩ run", "^Space complete"} {
		if !strings.Contains(got, want) {
			t.Errorf("the editor hint is %q, want it to name %q", got, want)
		}
	}
}

// The preview is one of the four things this client does, and nothing on
// screen said it existed.
func TestTheTreeOffersThePreviewItIsThereFor(t *testing.T) {
	got := treeAffordance(true, testKeys())

	if !strings.Contains(got, "previews") {
		t.Errorf("the tree hint is %q, want it to name the preview", got)
	}
}

// A grid with rows can be sorted, opened and copied. An empty one can do
// none of those, and saying so would be three keys that answer nothing.
func TestTheGridOffersItsKeysOnlyOnceThereAreRows(t *testing.T) {
	k := testKeys()

	got := gridAffordance(true, true, k)
	for _, want := range []string{"⌘⇧C copy", "⌘⇧S sort", "⌘I row"} {
		if !strings.Contains(got, want) {
			t.Errorf("the grid hint is %q, want it to name %q", got, want)
		}
	}

	if got := gridAffordance(true, false, k); got != "" {
		t.Errorf("an empty grid offered %q, want nothing", got)
	}
}

// The hints are the first thing a narrow region drops, so they have to be
// short enough to survive an ordinary width first.
func TestEveryHintFitsTheRegionThatCarriesIt(t *testing.T) {
	k := testKeys()

	// The editor and the result share the width beside the schema pane on an
	// eighty-column terminal, less the tab strip and the gap before a detail.
	const room = 80 - sidebarWidth - 1 - 12

	for name, hint := range map[string]string{
		"editor": editorAffordance(true, k),
		"grid":   gridAffordance(true, true, k),
	} {
		if got := visibleCost(hint); got > room {
			t.Errorf("the %s hint is %d cells wide, want at most %d: %q", name, got, room, hint)
		}
	}

	// The schema pane is narrower still, and its tab strip takes most of it.
	// Cells, not bytes: the marker is one column and three bytes.
	paneRoom := sidebarWidth - 2 - visibleCost("▸tree tables") - 2
	if got := visibleCost(treeAffordance(true, k)); got > paneRoom {
		t.Errorf("the tree hint is %d cells wide, want at most %d", got, paneRoom)
	}
}

// An empty result invites the statement rather than describing the gap.
func TestAnEmptyResultSaysWhatToPress(t *testing.T) {
	got := resultHint(resultState{}, testKeys())

	if !strings.Contains(got, "⌘↩") {
		t.Errorf("the empty result says %q, want it to name the run key", got)
	}
}

// A missing table is answered by the tree, which the server's message has no
// way of knowing about.
func TestAMissingTablePointsAtTheTree(t *testing.T) {
	got := failureDirection("Error 1146 (42S02): Table 'app.orders' doesn't exist")

	if !strings.Contains(got, "tree") {
		t.Errorf("failureDirection() = %q, want it to point at the tree", got)
	}
}

// Only failures with somewhere to point get a pointer. For the rest there is
// nothing this interface can add, and inventing something would be noise
// beside the one line that matters.
func TestAnOrdinaryFailureGetsNoDirection(t *testing.T) {
	got := failureDirection("Error 1064 (42000): You have an error in your SQL syntax")

	if got != "" {
		t.Errorf("failureDirection() = %q, want nothing", got)
	}
}
