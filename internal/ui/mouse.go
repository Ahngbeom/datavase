package ui

import (
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// bindMouse turns a click that landed on something into what that thing does.
//
// It is global for the reason the key router is: the primitive that receives
// a click is often not the one that should act. A region header is a TextView
// that cannot take focus, and clicking a tab name there has to change the
// Pages below it.
//
// Dispatch is a switch over the action rather than an early return on
// anything that is not a left click, because a right-click and a
// double-click branch both have to run before a left click is decided —
// each is its own case here, not a guard this one has to be inverted around.
//
// Only a click that lands in a zone is claimed. Everything else is handed
// back untouched, because tview already selects rows and scrolls correctly
// and re-deciding that here would be a second, worse implementation.
func (a *App) bindMouse() {
	a.app.SetMouseCapture(func(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if ev == nil {
			return ev, action
		}
		// Mouse reporting itself stays on regardless: the client turns it on
		// for its own terminal (attach.go), and this setting lives on the
		// server. Off only stops dv claiming the click, so the zones and the
		// context menu go quiet and tview's own handling — a table's default
		// row selection, for instance — is whatever is left underneath.
		if !a.mouseEnabled {
			return ev, action
		}

		// tview keeps one "last click" timestamp shared across every button
		// (Application.fireMouseActions), not one per button. So the right
		// click that just opened the menu sets it, and a left click on the
		// menu within DoubleClickInterval afterwards is read as completing a
		// double click rather than a click of its own — and List's own
		// MouseHandler only acts on a click, not a double click, so the row
		// would silently not respond. That is not a slow-user edge case: it
		// is the exact gesture this feature is for, read the row and click
		// it, so the first click on a freshly opened menu must not be the
		// one that gets eaten.
		if action == tview.MouseLeftDoubleClick && a.menuOpen() {
			action = tview.MouseLeftClick
		}

		switch action {
		case tview.MouseLeftClick:
			return a.mouseLeftClick(ev, action)
		case tview.MouseLeftDoubleClick:
			return a.mouseLeftDoubleClick(ev, action)
		case tview.MouseRightClick:
			return a.mouseRightClick(ev, action)
		}
		return ev, action
	})
}

// dialogOpen reports whether something other than the main page is showing.
//
// It owns the pointer the way it owns the keyboard: every handler below
// checks this before it acts, so a click cannot reach through a dialog to
// whatever is under it.
func (a *App) dialogOpen() bool {
	name, _ := a.pages.GetFrontPage()
	return name != pageMain
}

// menuOpen reports whether the right-click menu is the front page.
//
// Separate from dialogOpen because the double-click rewrite above is only
// safe where a right click is what put the front page there in the first
// place — the menu is the only surface that opens on one today.
func (a *App) menuOpen() bool {
	name, _ := a.pages.GetFrontPage()
	return name == pageMenu
}

// gridVisible reports whether x, y lands on the grid and the grid is the
// result pane's own current tab, rather than DDL, plan or sessions sharing
// its rect.
//
// tview does not reset a hidden primitive's rect when it stops being drawn
// (contextAt's own comment, menu.go), so a.grid.InRect keeps matching a
// screen position long after the DDL tab has replaced it there. Every
// caller that would otherwise act on the grid from a screen position has to
// gate on this, or it acts on a result the user cannot see.
func (a *App) gridVisible(x, y int) bool {
	return a.resultTabs.current() == tabResults && a.grid.InRect(x, y)
}

// zoneAt is the dialog guard and the hitmap lookup together.
//
// Task 5 folded both into mouseLeftClick alone. A second per-action handler
// needs the same guard, so keeping them separate would only have meant
// duplicating the pair the moment that handler was added.
func (a *App) zoneAt(ev *tcell.EventMouse) (zone, bool) {
	if a.dialogOpen() {
		return zone{}, false
	}
	x, y := ev.Position()
	return a.hits.at(x, y)
}

// mouseLeftClick resolves a left click, and hands it back untouched when
// nothing claims it.
func (a *App) mouseLeftClick(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if a.dialogOpen() {
		return ev, action
	}

	// The grid is checked before the hitmap and outside it entirely: it owns
	// its own layout and has no zones of its own.
	if a.clickGridHeader(ev) {
		return nil, action
	}

	z, ok := a.zoneAt(ev)
	if !ok {
		return ev, action
	}
	_, row := ev.Position()
	if !a.mouseAction(z, row) {
		return ev, action
	}
	// Swallowed: a header click that also reached tview would select the row
	// under it as well as doing what was asked.
	return nil, action
}

// mouseLeftDoubleClick opens the row a double click landed on — what every
// grid already means by that gesture, and otherwise a chord nobody guesses.
func (a *App) mouseLeftDoubleClick(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if a.dialogOpen() {
		return ev, action
	}

	x, y := ev.Position()
	if !a.gridVisible(x, y) {
		return ev, action
	}
	row, col := a.grid.CellAt(x, y)
	// CellAt returns column -1 for a row's populated span past its last
	// column — a double click landing in that trailing space is a double
	// click on nothing, the same as landing above row 0.
	if row <= 0 || col < 0 {
		return ev, action
	}

	// showRow reads the grid's own selection, and Inspect only opens it when
	// the grid holds focus — both have to be true first, the way they would
	// be if the row had been reached from the keyboard.
	a.grid.Select(row, col)
	a.app.SetFocus(a.grid)
	a.refreshHints()
	a.dispatch(keymap.ActionInspect)
	return nil, action
}

// mouseRightClick opens the palette's own commands, filtered to where the
// click landed — the answer to "what can I do here" that the left-click
// zones and the command palette do not give on their own.
//
// A position contextAt does not claim is handed back untouched, per its own
// doc comment on what false means there.
func (a *App) mouseRightClick(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if a.dialogOpen() {
		return ev, action
	}

	x, y := ev.Position()
	ctx, ok := a.contextAt(x, y)
	if !ok {
		return ev, action
	}
	a.showMenu(ctx, x, y)
	return nil, action
}

// clickGridHeader sorts by the column a click landed on, reaching the same
// sortColumn the key does: it reads the selection, so the selection has to
// move first, or a mouse sort and a key sort could land on different orders.
//
// The grid is not zone-based. tview.Table lays its columns out by content
// width, so there is no way to compute a column's screen position ahead of
// time the way a zone does — CellAt asks tview directly, which is the only
// thing that actually knows.
func (a *App) clickGridHeader(ev *tcell.EventMouse) bool {
	x, y := ev.Position()
	if !a.gridVisible(x, y) {
		return false
	}
	row, col := a.grid.CellAt(x, y)
	if row != 0 || col < 0 {
		return false
	}

	selRow, _ := a.grid.GetSelection()
	if selRow < 1 {
		selRow = 1
	}
	a.grid.Select(selRow, col)
	a.sortColumn()
	return true
}

// treeNodeAt finds the node drawn at a screen row, resolving a right click's
// position the same way TreeView's own left-click handling does
// (MouseHandler in treeview.go: offset plus the row inside the inner rect,
// indexing into the nodes visible in expanded order).
//
// TreeView keeps that visible-node list to itself, built by walking the root
// and recursing into expanded children (its unexported process()); Walk is
// the exported way to rebuild the same order from outside the package, which
// a right click needs — it resolves the node before running a command,
// rather than while tview is already handling the click itself.
func (a *App) treeNodeAt(y int) *tview.TreeNode {
	root := a.tree.GetRoot()
	if root == nil {
		return nil
	}

	_, rectY, _, _ := a.tree.GetInnerRect()
	target := a.tree.GetScrollOffset() + (y - rectY)
	if target < 0 {
		return nil
	}

	var found *tview.TreeNode
	index := -1
	root.Walk(func(node, parent *tview.TreeNode) bool {
		index++
		if index == target {
			found = node
		}
		return node.IsExpanded()
	})
	return found
}

// tableItemAt finds the tables-tab item index drawn at a screen row,
// resolving a right click's position the same way List's own MouseHandler
// does (indexAtPoint in list.go: the row inside the inner rect, offset by
// how much has scrolled).
//
// indexAtPoint halves that row count when the list shows secondary text —
// a.tableList never does (buildTablesTab: ShowSecondaryText(false), so that
// each table draws on exactly one row), so this only reconstructs the half
// of indexAtPoint that applies to it, using GetOffset in place of the
// private field indexAtPoint reads directly.
func (a *App) tableItemAt(y int) int {
	_, rectY, _, height := a.tableList.GetInnerRect()
	if y < rectY || y >= rectY+height {
		return -1
	}

	offset, _ := a.tableList.GetOffset()
	index := y - rectY + offset
	if index < 0 || index >= a.tableList.GetItemCount() {
		return -1
	}
	return index
}

// mouseAction performs a zone's click and reports whether it consumed it.
//
// Every branch reaches the same function the key does. That is what lets the
// tests written as h.do(action) stand as this layer's regression net, and
// what stops the mouse coming to do more or less than the keyboard.
func (a *App) mouseAction(z zone, row int) bool {
	switch z.target {
	case zoneDataSource:
		return a.dispatch(keymap.ActionSwitchDataSource)
	case zoneSchema:
		return a.dispatch(keymap.ActionUseSchema)
	case zoneHelp:
		return a.dispatch(keymap.ActionHelp)
	case zoneTab:
		// Named, not cycled: the point of a strip is that you can go
		// straight to the one you can see.
		if pane := a.paneFor(row); pane != nil && z.index >= 0 && z.index < len(pane.names) {
			pane.show(pane.names[z.index])
			a.app.SetFocus(pane)
			a.refreshHints()
			return true
		}
	case zoneRegionName:
		if pane := a.paneFor(row); pane != nil {
			a.app.SetFocus(pane)
			a.refreshHints()
			return true
		}
	case zoneStatusMode:
		a.showKeyboardChooser()
		return true
	case zoneStatusWrites:
		a.disableWrites()
		return true
	}
	return false
}

// recorderFor gives a region a recorder that also remembers which region
// owns the row, so a click on a tab or a region name knows which strip it
// was in — the hitmap alone only carries a zone's target and index, not who
// drew it.
func (a *App) recorderFor(pane *tabbed) func(row int, zones []zone) {
	return func(row int, zones []zone) {
		a.hits.set(row, zones)
		if a.paneRows == nil {
			a.paneRows = make(map[int]*tabbed)
		}
		a.paneRows[row] = pane
	}
}

// paneFor looks up which tabbed region drew the header at a screen row.
func (a *App) paneFor(row int) *tabbed {
	return a.paneRows[row]
}
