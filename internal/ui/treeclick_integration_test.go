//go:build integration

package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// quickSecondClick presses once, straight after another click, which is what
// tview reads as a double click wherever it landed.
func (h *harness) quickSecondClick(x, y int) {
	h.t.Helper()

	before := h.mouseActionCount(tview.MouseLeftDoubleClick)
	h.screen.InjectMouse(x, y, tcell.Button1, tcell.ModNone)
	h.screen.InjectMouse(x, y, tcell.ButtonNone, tcell.ModNone)
	h.awaitMouseAction(tview.MouseLeftDoubleClick, before+1)
}

// currentTreeLabel is what the tree says is selected right now.
func (h *harness) currentTreeLabel() string {
	h.t.Helper()

	var label string
	h.inspect(func(a *App) bool {
		if n := a.tree.GetCurrentNode(); n != nil {
			label = n.GetText()
		}
		return true
	})
	return label
}

// expandedTree leaves the sidebar open with a schema expanded, so the tree
// holds several rows worth clicking between.
func expandedTree(t *testing.T) *harness {
	t.Helper()

	h := newHarness(t, config.EnvDev)

	h.typeSQL("CREATE TABLE IF NOT EXISTS click_a (id INT); CREATE TABLE IF NOT EXISTS click_b (id INT)")
	h.do(keymap.ActionRunAll)
	h.waitFor("the fixture", func(a *App) bool { return a.batch == nil && a.running == nil })

	h.showSidebar()
	h.waitFor("the schema", func(a *App) bool {
		return a.tree.GetRoot() != nil && len(a.tree.GetRoot().GetChildren()) == 1
	})

	// Expand it the way the keyboard would, so the click test starts from a
	// tree that already has rows below the schema.
	h.inspect(func(a *App) bool {
		schema := a.tree.GetRoot().GetChildren()[0]
		a.tree.SetCurrentNode(schema)
		a.onTreeSelect(schema)
		return true
	})
	h.waitFor("the tables", func(a *App) bool {
		return len(a.tree.GetRoot().GetChildren()[0].GetChildren()) > 1
	})
	return h
}

// Clicking one row and then another is the ordinary way to look around a
// tree, and the second click is often quick — the eye has already found the
// row it wants.
//
// tview decides a click is a double click on elapsed time alone: two presses
// inside its 500ms interval, wherever each landed. Its TreeView handles no
// double click, so that second press reaches nothing and the selection stays
// where the first one put it. Someone doing this reads it as the tree
// refusing to select what they clicked.
func TestASecondClickElsewhereInTheTreeSelectsWhereItLanded(t *testing.T) {
	h := expandedTree(t)

	x, first := h.treeNodePosition(2)
	_, second := h.treeNodePosition(3)

	h.click(x, first)
	h.waitFor("the first row selected", func(a *App) bool {
		return a.tree.GetCurrentNode() != nil
	})
	was := h.currentTreeLabel()

	// No pause between them: back to back is exactly the case that breaks.
	// tview turns that second press into a double click, so the wait is for
	// that action rather than for another plain click.
	h.quickSecondClick(x, second)

	if got := h.currentTreeLabel(); got == was {
		t.Errorf("the second click left %q selected; it landed on a different row, "+
			"which is the row that should be selected now", got)
	}
}
