//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// runSQL types a statement, runs it and waits for the rows.
func (h *harness) runSQL(sql string, wantRows int) {
	h.t.Helper()

	h.typeSQL(sql)
	h.do(keymap.ActionRun)
	h.waitFor("the result", func(a *App) bool { return a.buf.RowCount() == wantRows })
}

// The reason a comparator exists at all. Values reach the buffer as bytes over
// the text protocol, so a sort that read them as they arrive would put 100
// before 9 — and it would look exactly like a sorted column.
func TestSortingANumericColumnDoesNotOrderItAsText(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 9 AS n UNION ALL SELECT 100 UNION ALL SELECT 10", 3)

	h.inspect(func(a *App) bool { a.grid.Select(1, 0); return true })
	h.do(keymap.ActionSortColumn)

	want := []string{"9", "10", "100"}
	h.waitFor("the column to be sorted as numbers", func(a *App) bool {
		for row := 1; row <= 3; row++ {
			if a.content.GetCell(row, 0).Text != want[row-1] {
				return false
			}
		}
		return true
	})
}

// A character column keeps the server's own ordering. Improving on it here
// would mean the grid and an ORDER BY disagreeing about the same column.
func TestSortingATextColumnKeepsTextOrder(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT '9' AS s UNION ALL SELECT '100' UNION ALL SELECT '10'", 3)

	h.inspect(func(a *App) bool { a.grid.Select(1, 0); return true })
	h.do(keymap.ActionSortColumn)

	want := []string{"10", "100", "9"}
	h.waitFor("the column to be sorted as text", func(a *App) bool {
		for row := 1; row <= 3; row++ {
			if a.content.GetCell(row, 0).Text != want[row-1] {
				return false
			}
		}
		return true
	})
}

// Everything that acts on the selected row reads the grid's row number, and
// after a sort that is no longer the buffer's. Copying is the one where the
// wrong answer is silent — it goes to the clipboard and nowhere else.
func TestCopyingAfterASortTakesTheRowOnScreen(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 3 AS n, 'three' AS word UNION ALL SELECT 1, 'one' UNION ALL SELECT 2, 'two'", 3)

	h.focusGrid()
	h.inspect(func(a *App) bool { a.grid.Select(1, 0); return true })
	h.do(keymap.ActionSortColumn)
	h.waitFor("the sort", func(a *App) bool { return a.content.GetCell(1, 0).Text == "1" })

	// The top row on screen is now n=1, which arrived second.
	h.inspect(func(a *App) bool { a.grid.Select(1, 1); return true })
	h.do(keymap.ActionCopyOrCancel)

	h.waitFor("the value to be copied", func(a *App) bool { return a.readClipboard() == "one" })
}

// The selection is on a row, not on a screen position. A sort that leaves the
// cursor where it was puts a different row under it without saying so.
func TestTheSelectionFollowsItsRowThroughASort(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 3 AS n UNION ALL SELECT 1 UNION ALL SELECT 2", 3)

	// Sit on the first row, which holds 3.
	h.inspect(func(a *App) bool { a.grid.Select(1, 0); return true })
	h.do(keymap.ActionSortColumn)

	h.waitFor("the selection to follow the 3 to the bottom", func(a *App) bool {
		row, _ := a.grid.GetSelection()
		return a.content.GetCell(row, 0).Text == "3"
	})
}

// What the notice says is settled in sortNotice, which is a table test. This
// is the part that cannot be: that it reaches the status bar at all, and that
// a complete result is not described as a cut one.
func TestSortingSaysWhatItSorted(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 2 AS n UNION ALL SELECT 1", 2)

	h.inspect(func(a *App) bool { a.grid.Select(1, 0); return true })
	h.do(keymap.ActionSortColumn)

	if !h.waitForScreen("sorted by n") {
		t.Fatalf("the sort said nothing; screen:\n%s", h.text())
	}
	if got := h.text(); strings.Contains(got, "not the whole ordering") {
		t.Errorf("a complete result was described as cut short:\n%s", got)
	}
}

// The header is the only thing that distinguishes the server's ordering from
// this application's rearrangement of it.
func TestASortedColumnSaysSoInItsHeader(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 2 AS n UNION ALL SELECT 1", 2)

	h.inspect(func(a *App) bool { a.grid.Select(1, 0); return true })

	h.do(keymap.ActionSortColumn)
	h.waitFor("the ascending marker", func(a *App) bool {
		return strings.Contains(a.content.GetCell(0, 0).Text, "↑")
	})

	h.do(keymap.ActionSortColumn)
	h.waitFor("the descending marker", func(a *App) bool {
		return strings.Contains(a.content.GetCell(0, 0).Text, "↓")
	})

	// And the third press takes the marker away with the ordering.
	h.do(keymap.ActionSortColumn)
	h.waitFor("the marker to go", func(a *App) bool {
		return a.content.GetCell(0, 0).Text == "n"
	})
}
