package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
)

// keys stands in for the live key map: the hints name whatever the map says,
// so a rebinding can never leave the header advertising a key that does
// nothing.
func testKeys() affordanceKeys {
	k := affordanceKeys{
		run:      "⌘↩",
		runAll:   "⌘⇧↩",
		complete: "^Space",
		history:  "⌘⇧F",
		copy:     "⌘⇧C",
		sort:     "⌘⇧S",
		inspect:  "⌘I",
		row:      "⌘⇧R",
	}
	k.short.run, k.short.complete = "F5", "^Space"
	k.short.copy, k.short.sort, k.short.inspect, k.short.row = "F3", "F12", "F4", "F8"
	return k
}

// wide is more room than any hint needs, for the tests that are about what a
// hint says rather than how it copes with a narrow header.
const wide = 200

// Only the region the keyboard is in speaks. Three regions offering their
// keys at once is a screen shouting, and a reader who has stopped seeing any
// of it learns nothing from the one that mattered.
func TestOnlyTheFocusedRegionOffersItsKeys(t *testing.T) {
	k := testKeys()

	if got := editorAffordance(false, wide, k); got != "" {
		t.Errorf("the unfocused editor said %q, want nothing", got)
	}
	if got := treeAffordance(false, k); got != "" {
		t.Errorf("the unfocused tree said %q, want nothing", got)
	}
	if got := gridAffordance(false, true, wide, k); got != "" {
		t.Errorf("the unfocused grid said %q, want nothing", got)
	}
}

// What the editor can do that is not obvious from looking at it: everything
// except typing.
func TestTheEditorOffersRunningCompletionAndHistory(t *testing.T) {
	got := editorAffordance(true, wide, testKeys())

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

	got := gridAffordance(true, true, wide, k)
	for _, want := range []string{"⌘⇧C copy", "⌘⇧S sort", "⌘I row"} {
		if !strings.Contains(got, want) {
			t.Errorf("the grid hint is %q, want it to name %q", got, want)
		}
	}

	if got := gridAffordance(true, false, wide, k); got != "" {
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
		"editor": editorAffordance(true, room, k),
		"grid":   gridAffordance(true, true, room, k),
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

// A header too narrow for the keys this application teaches names the
// fallbacks instead of being cut off mid-word.
//
// The long labels are what a machine without the Apple glyphs draws:
// "Super+↩ run · Ctrl+Space complete" is thirty-four cells where the editor
// header often has thirty.
func TestANarrowHeaderNamesTheShorterKeys(t *testing.T) {
	k := affordanceKeys{run: "Super+↩", complete: "Ctrl+Space"}
	k.short.run, k.short.complete = "F5", "Ctrl+Space"

	wide := editorAffordance(true, 40, k)
	if !strings.Contains(wide, "Super+↩") {
		t.Errorf("a wide header said %q, want the key this application teaches", wide)
	}

	narrow := editorAffordance(true, 30, k)
	if !strings.Contains(narrow, "F5") {
		t.Errorf("a narrow header said %q, want the fallback key", narrow)
	}
	if got := visibleCost(narrow); got > 30 {
		t.Errorf("the narrow form is %d cells, want at most 30: %q", got, narrow)
	}
}

// Whatever the labels look like on this machine, both regions' hints fit the
// header they are drawn in.
func TestTheHintsFitOnEitherLabelStyle(t *testing.T) {
	const room = 80 - sidebarWidth - 1 - 12

	was := onMac
	defer func() { onMac = was }()

	for _, mac := range []bool{true, false} {
		onMac = mac
		k := liveKeys(t)

		for name, hint := range map[string]string{
			"editor": editorAffordance(true, room, k),
			"grid":   gridAffordance(true, true, room, k),
		} {
			if got := visibleCost(hint); got > room {
				t.Errorf("with onMac=%v the %s hint is %d cells, want at most %d: %q",
					mac, name, got, room, hint)
			}
		}
	}
}

// liveKeys builds the labels from the real map, which is what the interface
// draws with.
func liveKeys(t *testing.T) affordanceKeys {
	t.Helper()

	m := keymap.Default()
	first := func(a keymap.Action) string {
		if b := m.DisplayBindings(a); len(b) > 0 {
			return b[0].Label(onMac)
		}
		return ""
	}
	shortest := func(a keymap.Action) string {
		best := ""
		for _, b := range m.DisplayBindings(a) {
			if l := b.Label(onMac); best == "" || visibleCost(l) < visibleCost(best) {
				best = l
			}
		}
		return best
	}

	k := affordanceKeys{
		run:      first(keymap.ActionRun),
		complete: first(keymap.ActionComplete),
		copy:     first(keymap.ActionCopyResult),
		sort:     first(keymap.ActionSortColumn),
		inspect:  first(keymap.ActionInspect),
	}
	k.short.run, k.short.complete = shortest(keymap.ActionRun), shortest(keymap.ActionComplete)
	k.short.copy = shortest(keymap.ActionCopyResult)
	k.short.sort = shortest(keymap.ActionSortColumn)
	k.short.inspect = shortest(keymap.ActionInspect)
	return k
}
