//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// searchFor opens the prompt with "/" and types a pattern, without confirming.
func (h *harness) searchFor(pattern string) {
	h.t.Helper()

	h.inject(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	h.waitFor("the search prompt", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageSearch
	})
	h.typeInto(pattern)
}

const searchBuffer = "SELECT id FROM users;\nSELECT name FROM orders;\n"

// The whole point: the caret ends up at the match.
func TestSearchingMovesTheCaretToTheMatch(t *testing.T) {
	h := newVimHarness(t)
	h.buffer(searchBuffer, 0)

	h.searchFor("orders")
	h.press(tcell.KeyEnter)

	want := strings.Index(searchBuffer, "orders")
	h.waitFor("the caret to reach the match", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == want
	})
}

// Matching as the pattern grows is what makes the prompt worth typing into
// rather than a dialog to fill in and submit.
func TestTheCaretFollowsThePatternAsItIsTyped(t *testing.T) {
	h := newVimHarness(t)
	h.buffer(searchBuffer, 0)

	h.searchFor("orders")
	h.waitFor("the caret to move before Enter", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == strings.Index(searchBuffer, "orders")
	})
}

// Typing one character too many has to give the caret back.
//
// Leaving it where the last pattern that still matched had reached shows a
// match for a prefix of what is on screen, which reads as a match for the
// whole of it — the caret sits on "br" while the prompt says "brX".
func TestAPatternThatStopsMatchingReturnsTheCaret(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("alpha\nbravo\n", 0)

	h.searchFor("br")
	h.waitFor("the caret to reach the prefix match", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == len("alpha\n")
	})

	h.typeInto("X")
	h.waitFor("the caret to come back", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == 0
	})
}

// Abandoning a search has to put back where it started, not where the last
// keystroke happened to land.
func TestEscapingASearchRestoresTheCaret(t *testing.T) {
	h := newVimHarness(t)
	const start = 3
	h.buffer(searchBuffer, start)

	h.searchFor("orders")
	h.waitFor("the caret to move", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) != start
	})

	h.press(tcell.KeyEscape)
	h.waitFor("the caret to come back", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == start
	})
}

// A search that quietly does nothing is indistinguishable from a keyboard that
// has stopped working.
func TestASearchWithNoMatchSaysSo(t *testing.T) {
	h := newVimHarness(t)
	h.buffer(searchBuffer, 0)

	h.searchFor("nowhere")
	h.press(tcell.KeyEnter)

	if !h.waitForScreen("no match") {
		t.Errorf("nothing said the search found nothing; screen:\n%s", h.text())
	}
}

func TestRepeatingStepsForwardsAndBack(t *testing.T) {
	h := newVimHarness(t)
	h.buffer(searchBuffer, 0)

	first := strings.Index(searchBuffer, "SELECT")
	second := strings.Index(searchBuffer[first+1:], "SELECT") + first + 1

	h.searchFor("SELECT")
	h.press(tcell.KeyEnter)
	h.waitFor("the first match", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == first
	})

	h.inject(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	h.waitFor("the second match", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == second
	})

	h.inject(tcell.NewEventKey(tcell.KeyRune, 'N', tcell.ModNone))
	h.waitFor("the first match again", func(a *App) bool {
		return a.caretOffset(a.editor.GetText()) == first
	})
}

// Searching in visual mode extends the selection, as every other motion there
// does.
func TestSearchingInVisualModeSelectsUpToTheMatch(t *testing.T) {
	h := newVimHarness(t)
	h.buffer(searchBuffer, 0)

	h.inject(tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModNone))
	h.waitFor("visual mode", func(a *App) bool { return a.vimSelecting() })

	h.searchFor("FROM")
	h.press(tcell.KeyEnter)

	h.waitFor("the selection to reach the match", func(a *App) bool {
		return strings.HasPrefix(h.appSelection(a), "SELECT id ")
	})
}

func (h *harness) appSelection(a *App) string {
	if !a.editor.HasSelection() {
		return ""
	}
	text, _, _ := a.editor.GetSelection()
	return text
}

// The results are the other half. A value in row 400 is exactly what a grid
// search is for.
func TestSearchingTheResultsSelectsTheMatchingCell(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 'alpha' AS a UNION ALL SELECT 'beta' UNION ALL SELECT 'gamma'")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("3 rows") {
		t.Fatalf("the statement never finished:\n%s", h.text())
	}

	h.focusGrid()
	h.searchFor("gamma")
	h.press(tcell.KeyEnter)

	h.waitFor("the third row to be selected", func(a *App) bool {
		row, _ := a.grid.GetSelection()
		return row == 3 // the header is row 0
	})
}

// The grid displays a truncated, markup-escaped copy of each value. Searching
// that copy rather than the data means a bracket in the data can never be
// found, and neither can anything past the truncation.
func TestSearchingTheResultsLooksAtTheDataNotTheDisplayedCopy(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	// Neither target is in the first row, which is selected before any search
	// runs — an assertion satisfied by the starting position proves nothing.
	// Row 2 holds a bracket, which the display doubles for the markup parser;
	// row 3 hides its needle past the 200-rune display limit.
	h.typeSQL("SELECT 'filler' AS a" +
		" UNION ALL SELECT 'a[b'" +
		" UNION ALL SELECT CONCAT(REPEAT('x', 300), 'needle')")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("3 rows") {
		t.Fatalf("the statement never finished:\n%s", h.text())
	}

	for _, tc := range []struct {
		pattern string
		wantRow int
	}{
		{"a[b", 2},
		{"needle", 3},
	} {
		h.focusGrid()
		h.searchFor(tc.pattern)
		h.press(tcell.KeyEnter)

		h.waitFor("row "+tc.pattern, func(a *App) bool {
			row, _ := a.grid.GetSelection()
			return row == tc.wantRow
		})
	}
}

// focusGrid puts the results under the keyboard, which is what decides that a
// search means the results rather than the editor.
func (h *harness) focusGrid() {
	h.t.Helper()

	h.inspect(func(a *App) bool {
		a.app.SetFocus(a.grid)
		return true
	})
	h.waitFor("the grid to hold focus", func(a *App) bool { return a.app.GetFocus() == a.grid })
}

// The history did not disappear when ⌘F stopped meaning it; it moved, and the
// key it moved to has to work.
func TestTheQueryHistoryIsStillReachable(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionSearchHistory)
	h.waitFor("the history dialog", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageHistory
	})
}

// The prompt must not blank the screen: the text being searched is the one
// thing that has to stay visible while the pattern is typed.
func TestTheSearchPromptLeavesTheTextOnScreen(t *testing.T) {
	h := newVimHarness(t)
	h.buffer(searchBuffer, 0)

	h.searchFor("orders")

	if !strings.Contains(h.text(), "SELECT id FROM users") {
		t.Errorf("the prompt hid the buffer it is searching:\n%s", h.text())
	}
}
