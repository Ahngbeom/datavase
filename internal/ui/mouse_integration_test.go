//go:build integration

package ui

import (
	"slices"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// The schema name on the top bar is where someone looks to see which schema
// an unqualified statement will reach, so it is where they reach to change
// it. Finding it inert sends them to the key reference.
func TestClickingTheSchemaOnTheTopBarOffersToChangeIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.clickZone(zoneSchema, -1)

	h.waitFor("the schema chooser to open", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageUseSchema
	})
}

// The help key is written on the top bar. A hint that names a key and does
// nothing when pressed on is worse than no hint.
func TestClickingTheHelpHintOpensTheKeyReference(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.clickZone(zoneHelp, -1)

	h.waitFor("the key reference to open", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageHelp
	})
}

// mouse.go's early return is the one line the whole task exists to add: with
// the mouse off, a zone that used to open something must go back to doing
// nothing, not keep working underneath the setting meant to silence it.
func TestTurningTheMouseOffLeavesTheHelpHintInert(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.inspect(func(a *App) bool {
		a.mouseEnabled = false
		return true
	})

	h.clickZone(zoneHelp, -1)

	h.inspect(func(a *App) bool {
		if name, _ := a.pages.GetFrontPage(); name == pageHelp {
			t.Error("the help hint opened the key reference with the mouse off")
		}
		return true
	})
}

// A misclick on the production marker must not be able to look like it
// changed the environment.
func TestTheEnvironmentChipAnswersNoClick(t *testing.T) {
	h := newHarness(t, config.EnvProd)

	var before string
	h.inspect(func(a *App) bool {
		before, _ = a.pages.GetFrontPage()
		return true
	})

	h.click(1, 0)

	var after string
	h.inspect(func(a *App) bool {
		after, _ = a.pages.GetFrontPage()
		return true
	})
	if after != before {
		t.Errorf("clicking the environment chip opened %q", after)
	}
}

// A tab strip that names its tabs and cannot be clicked is a list of things
// you must find a key for.
func TestClickingATabNameShowsThatTab(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.buf.RowCount() > 0 })

	h.clickZone(zoneTab, 1)

	h.waitFor("the second tab to be shown", func(a *App) bool {
		return a.resultTabs.active == 1
	})
}

// The editor's header holds one unnamed tab, so it published no zone at all
// before the region-name zone existed — a click there did nothing. The spec
// asks for the whole header to answer a click ("region header, the region's
// name: focus that region"), not only the part that names a tab, and the
// editor is the case that had nothing at all until this.
func TestClickingTheEditorsHeaderFocusesTheEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.focusGrid()

	x, y := h.regionHeaderPosition(func(a *App) *tabbed { return a.editorRegion })
	h.click(x, y)

	h.waitFor("the editor to hold focus, and its header's marker to move with it", func(a *App) bool {
		return a.app.GetFocus() == a.editor && a.editorRegion.HasFocus()
	})
}

// Sorting by clicking the column header is what every grid does, and the key
// for it is one nobody guesses.
func TestClickingAColumnHeaderSortsByThatColumn(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 2 AS n UNION ALL SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.buf.RowCount() == 2 })

	x, y := h.gridHeaderPosition(0)
	h.click(x, y)

	h.waitFor("the rows to be sorted by the clicked column", func(a *App) bool {
		return a.content.sorted() && a.content.sortCol == 0
	})
}

// The DDL, plan and sessions tabs share the grid's screen rect once it has
// drawn there — tview does not reset a hidden primitive's rect when it stops
// being the one shown (contextAt's own comment, menu.go) — so a click where
// the grid used to be must not act on a result the user cannot see.
func TestClickingOverTheDDLTabDoesNotSortTheHiddenGrid(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 2 AS n UNION ALL SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.buf.RowCount() == 2 })
	x, y := h.gridHeaderPosition(0)

	h.app.app.QueueUpdateDraw(func() { h.app.resultTabs.show(tabDDL) })
	h.settle()

	h.click(x, y)
	h.settle()

	if h.inspect(func(a *App) bool { return a.content.sorted() }) {
		t.Error("a click where the grid's header used to be sorted the hidden result, with the DDL tab showing")
	}
}

// The same stale rect a click must not sort by, a double click must not open
// a row from — reading a table's DDL must not risk the row inspector opening
// on a row nobody can see.
func TestDoubleClickingOverTheDDLTabDoesNotOpenTheHiddenGridsRow(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1 AS id, 'ada@example.com' AS email")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.buf.RowCount() > 0 })
	x, y := h.gridHeaderPosition(0)

	h.app.app.QueueUpdateDraw(func() { h.app.resultTabs.show(tabDDL) })
	h.settle()

	h.doubleClick(x, y+1)
	h.settle()

	if h.inspect(func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageConfirm
	}) {
		t.Error("a double click where the grid used to be opened the row inspector, with the DDL tab showing")
	}
}

// The mouse must not be able to reach an ordering the keyboard cannot, or the
// two paths have started to diverge.
func TestClickingAHeaderAndPressingSortAgreeOnTheOrder(t *testing.T) {
	byKey := newHarness(t, config.EnvDev)
	byKey.typeSQL("SELECT 2 AS n UNION ALL SELECT 1")
	byKey.do(keymap.ActionRun)
	byKey.waitFor("two rows", func(a *App) bool { return a.buf.RowCount() == 2 })
	byKey.do(keymap.ActionSortColumn)
	keyOrder := byKey.gridColumn(0)

	byClick := newHarness(t, config.EnvDev)
	byClick.typeSQL("SELECT 2 AS n UNION ALL SELECT 1")
	byClick.do(keymap.ActionRun)
	byClick.waitFor("two rows", func(a *App) bool { return a.buf.RowCount() == 2 })
	x, y := byClick.gridHeaderPosition(0)
	byClick.click(x, y)
	clickOrder := byClick.gridColumn(0)

	if !slices.Equal(keyOrder, clickOrder) {
		t.Errorf("sorting by key gave %v and by click %v", keyOrder, clickOrder)
	}
}

// Clicking the column named "b" has to sort by "b" — checked by finding
// where that glyph is actually drawn (gridHeaderGlyphPosition reads the
// simulated screen directly) rather than by asking a.grid.CellAt for
// column 1's position, which is the same lookup a click resolves through
// and so cannot catch the two disagreeing.
func TestClickingTheSecondHeaderSortsByTheSecondColumnNotTheFirst(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1 AS a, 2 AS b UNION ALL SELECT 2, 1")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.buf.RowCount() == 2 })

	x, y := h.gridHeaderGlyphPosition('b')
	h.click(x, y)

	if got := h.gridColumn(1); !slices.Equal(got, []string{"1", "2"}) {
		t.Errorf("clicking column b's header sorted %v, want ascending by b", got)
	}
	if got := h.gridColumn(0); slices.Equal(got, []string{"1", "2"}) {
		t.Errorf("column a is also ascending; the click sorted by column 0, not the one clicked")
	}
}

// A grid is the wrong shape for a wide row, and the row view is the answer.
// Reaching it is a chord nobody guesses, and double click is what every grid
// in the world already means by "open this".
func TestDoubleClickingARowOpensItInFull(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1 AS id, 'ada@example.com' AS email")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.buf.RowCount() > 0 })

	x, y := h.gridHeaderPosition(0)
	h.doubleClick(x, y+1)

	h.waitFor("the row view to open", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageConfirm
	})
	// pageConfirm is the generic dialog page, shared with confirmation
	// dialogs — asserting it alone would pass for any of them. The row
	// view's title is what says it was actually this one that opened.
	if !h.waitForScreen("j/k step") {
		t.Errorf("the row view's title did not appear on screen:\n%s", h.text())
	}
}

// A tree that expands on Enter and does nothing on a click is a tree that
// looks broken to everyone who has used one before.
//
// No branch in mouse.go accompanies this test. tview.TreeView's own
// MouseHandler already moves the selection and calls the node's selected
// function — a.onTreeSelect, the same one Enter reaches — on a left click,
// so Task 5's capture hands a click on a tree node back to tview untouched
// and tview does the rest. This test is what tells us if a tview upgrade
// ever takes that away.
func TestClickingATreeNodeDoesWhatEnterDoes(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.do(keymap.ActionToggleSidebar)
	h.waitFor("the schema tree", func(a *App) bool { return a.sidebarVisible })

	// Nothing has selected a node yet, so GetCurrentNode is nil until the
	// click below sets it. The root — the node offset 0 will land on — is
	// asked directly instead, which is the actual state the click is about
	// to change.
	before := h.inspect(func(a *App) bool {
		return a.tree.GetRoot().IsExpanded()
	})

	x, y := h.treeNodePosition(0)
	h.click(x, y)

	h.waitFor("the clicked node to change how Enter would have changed it", func(a *App) bool {
		node := a.tree.GetCurrentNode()
		return node != nil && node.IsExpanded() != before
	})
}
