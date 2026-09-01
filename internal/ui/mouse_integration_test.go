//go:build integration

package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
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

// Mouse reporting disables the terminal's own text selection. Someone who
// copies by dragging must be able to turn this off — and must lose only the
// ways in, never a capability.
//
// menuEntries is filtered from paletteCommands by construction, so checking
// a menu entry's name for membership in that same list can never fail — it
// is not evidence that turning the mouse off costs nothing, only that
// menuEntries does what it obviously does. What has to hold is that the
// real palette, filtered the way a person actually reaches a command by
// name, still finds everything a right-click menu would have offered.
func TestWithTheMouseOffEveryMenuCommandIsStillNamedInThePalette(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.inspect(func(a *App) bool {
		a.mouseEnabled = false
		return true
	})

	seen := make(map[string]bool)
	for _, ctx := range allMenuContexts() {
		for _, e := range menuEntries(paletteCommands(), ctx, func(keymap.Action) string { return "" }) {
			if seen[e.name] {
				continue
			}
			seen[e.name] = true

			h.do(keymap.ActionCommandPalette)
			h.waitFor("the palette to open for "+e.name, func(a *App) bool {
				name, _ := a.pages.GetFrontPage()
				return name == pagePalette
			})

			h.typeInto(e.name)
			if !strings.Contains(h.text(), e.name) {
				t.Errorf("%q is offered on right click but the palette filter cannot find it with the mouse off:\n%s", e.name, h.text())
			}

			h.press(tcell.KeyEscape)
			h.waitFor("the palette to close for "+e.name, func(a *App) bool {
				name, _ := a.pages.GetFrontPage()
				return name == pageMain
			})
		}
	}
}
