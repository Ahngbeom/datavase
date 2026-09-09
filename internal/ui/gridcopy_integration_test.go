//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// twoColumnResult leaves a two-row result on screen and the keyboard in the
// grid, which is where both copy keys mean something.
func twoColumnResult(t *testing.T) *harness {
	t.Helper()

	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1 AS id, 'a@example.com' AS email UNION ALL SELECT 2, 'b@example.com'")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool {
		return a.status.phase == phaseDone && a.buf.RowCount() == 2
	})
	h.focusGrid()
	return h
}

// Selecting the whole query before running it is ordinary, and the selection
// survives the run. The copy key in the results has to mean the cell under
// the cursor anyway — otherwise reaching a value means going back to the
// editor and unselecting first, which nobody guesses.
func TestTheCopyKeyOnTheGridTakesTheCellWhileTheEditorStillHoldsASelection(t *testing.T) {
	h := twoColumnResult(t)

	h.selectAllText()
	h.focusGrid()
	h.waitFor("the editor's selection", func(a *App) bool { return a.editor.HasSelection() })

	h.do(keymap.ActionCopyOrCancel)

	h.waitFor("the cell on the clipboard", func(a *App) bool { return a.clipboard == "1" })
	if h.inspect(func(a *App) bool { return strings.Contains(a.clipboard, "SELECT") }) {
		t.Error("the editor's selection was copied instead of the cell under the cursor")
	}
}

// A row goes to a spreadsheet, so its values arrive as columns.
func TestCopyingARowPutsItsValuesOnTheClipboardTabSeparated(t *testing.T) {
	h := twoColumnResult(t)

	h.do(keymap.ActionCopyRow)

	h.waitFor("the row on the clipboard", func(a *App) bool {
		return a.clipboard == "1\ta@example.com"
	})
	h.waitFor("the notice", func(a *App) bool {
		return strings.Contains(a.status.message, "2 values copied")
	})
}

// The cursor decides which row, so moving it moves what the key copies.
func TestCopyingARowFollowsTheGridCursor(t *testing.T) {
	h := twoColumnResult(t)

	h.inspect(func(a *App) bool {
		a.grid.Select(2, 0) // row 1 is the header
		return true
	})
	h.do(keymap.ActionCopyRow)

	h.waitFor("the second row", func(a *App) bool {
		return a.clipboard == "2\tb@example.com"
	})
}

// The key means one result row, and the editor has no rows.
func TestCopyingARowFromTheEditorSaysWhereToStandInstead(t *testing.T) {
	h := twoColumnResult(t)

	h.inspect(func(a *App) bool {
		a.app.SetFocus(a.editor)
		return true
	})
	h.waitFor("the editor to hold focus", func(a *App) bool { return a.app.GetFocus() == a.editor })

	h.do(keymap.ActionCopyRow)
	h.waitFor("the notice", func(a *App) bool { return a.status.message == "select a result row first" })
}
