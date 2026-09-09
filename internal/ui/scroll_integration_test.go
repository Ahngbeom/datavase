//go:build integration

package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// rowText is what is actually drawn on a screen row — what the person about
// to click is looking at, which is the only thing a click can be judged
// against.
func (h *harness) rowText(row int) string {
	h.t.Helper()

	var out strings.Builder
	h.inspect(func(a *App) bool {
		cells, width, _ := h.screen.GetContents()
		for col := 0; col < width; col++ {
			out.Write(cells[row*width+col].Bytes)
		}
		return true
	})
	return strings.TrimSpace(out.String())
}

// scrollDown turns the wheel over a position, the number of notches given.
func (h *harness) scrollDown(x, y, notches int) {
	h.t.Helper()

	for i := 0; i < notches; i++ {
		h.screen.InjectMouse(x, y, tcell.WheelDown, tcell.ModNone)
	}
	h.settle()
}

// treeOfManyTables leaves the sidebar open on a schema expanded to more
// tables than the pane can show, which is what makes scrolling mean
// something.
func treeOfManyTables(t *testing.T) *harness {
	t.Helper()

	h := newHarness(t, config.EnvDev)

	var stmts []string
	for i := 0; i < 40; i++ {
		stmts = append(stmts, fmt.Sprintf("CREATE TABLE IF NOT EXISTS scroll_%02d (id INT)", i))
	}
	h.typeSQL(strings.Join(stmts, "; "))
	h.do(keymap.ActionRunAll)
	h.waitFor("the fixture", func(a *App) bool { return a.batch == nil && a.running == nil })

	h.showSidebar()
	h.waitFor("the schema", func(a *App) bool {
		return a.tree.GetRoot() != nil && len(a.tree.GetRoot().GetChildren()) == 1
	})
	h.inspect(func(a *App) bool {
		schema := a.tree.GetRoot().GetChildren()[0]
		a.tree.SetCurrentNode(schema)
		a.onTreeSelect(schema)
		return true
	})
	h.waitFor("the tables", func(a *App) bool {
		return len(a.tree.GetRoot().GetChildren()[0].GetChildren()) > 30
	})
	return h
}

// The whole of what a click has to promise: whatever is drawn on the row is
// what gets selected, however the tree got scrolled to show it.
//
// tview's wheel handling moves the view without moving the selection, and its
// next redraw pulls the view back to wherever the selection still is — so the
// rows slide out from under the pointer between looking and clicking, and
// some other table is selected.
func TestClickingAfterScrollingSelectsTheRowOnScreen(t *testing.T) {
	h := treeOfManyTables(t)

	x, y := h.treeNodePosition(5)
	h.scrollDown(x, y, 10)

	drawn := h.rowText(y)
	h.click(x, y)

	var selected string
	h.inspect(func(a *App) bool {
		if n := a.tree.GetCurrentNode(); n != nil {
			selected = n.GetText()
		}
		return true
	})

	if selected == "" || !strings.Contains(drawn, selected) {
		t.Errorf("the row read %q before the click and %q was selected", drawn, selected)
	}
}

// Scrolling has to stay where it was put. A view that springs back the moment
// anything else redraws cannot be aimed at at all.
func TestTheTreeStaysWhereItWasScrolled(t *testing.T) {
	h := treeOfManyTables(t)

	x, y := h.treeNodePosition(5)
	h.scrollDown(x, y, 6)

	was := h.rowText(y)

	// Any redraw at all: the status bar reporting something is enough.
	h.inspect(func(a *App) bool {
		a.notice("still here")
		return true
	})
	h.settle()

	if now := h.rowText(y); now != was {
		t.Errorf("the row under the pointer was %q and became %q without anyone scrolling", was, now)
	}
}

// The tables tab is the same widget problem with a different widget: its
// list pulls the view back to the selected item just as the tree does.
func TestTheTablesTabStaysWhereItWasScrolled(t *testing.T) {
	h := treeOfManyTables(t)

	h.inspect(func(a *App) bool {
		a.schemaTabs.show(tabTables)
		a.renderTables()
		return true
	})
	h.waitFor("the tables listed", func(a *App) bool { return len(a.listedTables) > 30 })

	x, y := h.tableItemPosition(4)
	h.scrollDown(x, y, 6)
	was := h.rowText(y)

	h.inspect(func(a *App) bool {
		a.notice("still here")
		return true
	})
	h.settle()

	if now := h.rowText(y); now != was {
		t.Errorf("the row under the pointer was %q and became %q without anyone scrolling", was, now)
	}
}

// tableItemPosition is a screen position on one row of the tables list.
func (h *harness) tableItemPosition(index int) (int, int) {
	h.t.Helper()
	h.settle()

	var x, y int
	h.inspect(func(a *App) bool {
		rx, ry, _, _ := a.tableList.GetInnerRect()
		x, y = rx+2, ry+index
		return true
	})
	return x, y
}
