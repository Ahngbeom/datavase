//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// previewFixture makes a table with three rows and lists it in the tables
// tab, and leaves the editor holding text a preview must not disturb.
func previewFixture(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, config.EnvDev)

	h.typeSQL("CREATE TABLE IF NOT EXISTS preview_t (id INT); TRUNCATE preview_t; INSERT INTO preview_t VALUES (1),(2),(3)")
	h.do(keymap.ActionRunAll)
	h.waitFor("the fixture", func(a *App) bool { return a.batch == nil && a.running == nil })

	schema := h.app.conn.DataSource().Database
	h.seedCache(catalog.Snapshot{
		Schemas: []string{schema},
		Tables:  map[string][]catalog.Table{schema: {{Name: "preview_t"}}},
	})
	h.typeSQL("-- my draft")
	return h
}

func TestEnterOnATableInTheTablesTabShowsItsRows(t *testing.T) {
	h := previewFixture(t)

	h.showSidebar()
	h.app.app.QueueUpdateDraw(func() {
		h.app.schemaTabs.show(tabTables)
		h.app.renderTables()
		h.app.app.SetFocus(h.app.tableList)
	})
	h.waitFor("the table listed", func(a *App) bool { return len(a.listedTables) == 1 })
	h.press(tcell.KeyEnter)

	h.waitFor("three rows", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 3 })
	if got := h.editorText(); got != "-- my draft" {
		t.Errorf("editor = %q; a preview must not replace what was being written", got)
	}
	h.waitFor("the SQL in the status bar", func(a *App) bool {
		return strings.Contains(a.status.message, "LIMIT 100")
	})
}

func TestDoubleClickingATableInTheTreeShowsItsRows(t *testing.T) {
	h := previewFixture(t)
	schema := h.app.conn.DataSource().Database

	h.showSidebar()
	h.waitForBackgroundRefresh(h.app.conn.DataSource().Name)
	// Expanding the schema through onTreeSelect directly, rather than a real
	// click, keeps that click out of tview's own double-click tracking —
	// otherwise the double click below can land within DoubleClickInterval
	// of this unrelated one and get read as its second half instead.
	h.app.app.QueueUpdateDraw(func() {
		for _, n := range h.app.tree.GetRoot().GetChildren() {
			if ref, ok := n.GetReference().(*nodeRef); ok && ref.schema == schema {
				h.app.onTreeSelect(n)
			}
		}
	})
	h.waitFor("the table node", func(a *App) bool {
		for _, n := range a.tree.GetRoot().GetChildren() {
			for _, c := range n.GetChildren() {
				if ref, ok := c.GetReference().(*nodeRef); ok && ref.table == "preview_t" {
					return true
				}
			}
		}
		return false
	})

	// Other tables the shared test database has accumulated may sort before
	// preview_t, so its row is found by walking the tree rather than assumed.
	var offset int
	h.inspect(func(a *App) bool {
		offset, _ = treeOffset(a.tree.GetRoot(), func(ref *nodeRef) bool {
			return ref.kind == nodeTable && ref.table == "preview_t"
		})
		return true
	})
	tx, ty := h.treeNodePosition(offset)
	h.doubleClick(tx, ty)
	h.waitFor("three rows", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 3 })
}

// treeOffset finds a node's position among the tree's visible rows — the
// same pre-order tview.TreeView itself walks to draw them — so a
// double-click lands on the right row regardless of what else sorts before
// it in a shared test database.
func treeOffset(node *tview.TreeNode, match func(*nodeRef) bool) (int, bool) {
	i := 0
	var walk func(n *tview.TreeNode) (int, bool)
	walk = func(n *tview.TreeNode) (int, bool) {
		mine := i
		i++
		if ref, ok := n.GetReference().(*nodeRef); ok && match(ref) {
			return mine, true
		}
		if n.IsExpanded() {
			for _, c := range n.GetChildren() {
				if off, ok := walk(c); ok {
					return off, true
				}
			}
		}
		return 0, false
	}
	return walk(node)
}

func TestAPreviewWhileAStatementRunsIsRefused(t *testing.T) {
	h := previewFixture(t)
	h.typeSQL("SELECT SLEEP(5)")
	h.do(keymap.ActionRun)
	h.waitFor("running", func(a *App) bool { return a.running != nil })

	h.app.app.QueueUpdateDraw(func() { h.app.previewTable("x", "y") })
	h.waitFor("the refusal", func(a *App) bool {
		return strings.HasPrefix(a.status.message, "a statement is already running")
	})
	h.do(keymap.ActionCancel)
}
