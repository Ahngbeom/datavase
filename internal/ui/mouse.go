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
// anything that is not a left click, because the double-click branch has to
// run before a left click is decided — it is its own case here, not a guard
// this one has to be inverted around.
//
// Only a click that lands in a zone is claimed. Everything else is handed
// back untouched, because tview already selects rows and scrolls correctly
// and re-deciding that here would be a second, worse implementation.
func (a *App) bindMouse() {
	a.app.SetMouseCapture(func(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if ev == nil {
			return ev, action
		}
		// Off only stops dv claiming the click, so the zones go quiet and
		// tview's own handling — a table's default row selection, for
		// instance — is whatever is left underneath.
		if !a.mouseEnabled {
			return ev, action
		}

		switch action {
		case tview.MouseLeftClick:
			return a.mouseLeftClick(ev, action)
		case tview.MouseLeftDoubleClick:
			return a.mouseLeftDoubleClick(ev, action)
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

// gridVisible reports whether x, y lands on the grid and the result pane's
// tab is the one that shows it.
//
// tview does not reset a hidden primitive's rect when it stops being drawn,
// so InRect alone would still match a grid the tab switch has since covered.
func (a *App) gridVisible(x, y int) bool {
	return a.resultTabs.current() == tabResults && a.grid.InRect(x, y)
}

// zoneAt is the dialog guard and the hitmap lookup together, so a caller can
// never forget the guard and act on zones from the page a dialog has since
// covered.
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

	// tview calls a click a double click on elapsed time alone: two presses
	// inside its interval, wherever each one landed. Neither TreeView nor
	// List handles a double click, so a second quick press anywhere in the
	// sidebar would reach nothing and the selection would stay where the
	// first one put it — the tree refusing, to the person clicking it, to
	// select the row under their pointer.
	//
	// So the row this landed on decides, not whatever is current.
	if a.sidebarVisible && a.schemaTabs.current() == tabTree && a.tree.InRect(x, y) {
		node := a.treeNodeAt(y)
		if node == nil {
			return ev, action
		}
		a.tree.SetCurrentNode(node)

		if ref, ok := node.GetReference().(*nodeRef); ok && ref.kind == nodeTable {
			a.previewTable(ref.schema, ref.table)
			return nil, action
		}
		// A schema or a column: what a single click there means, since that
		// is what this press would have been a moment later.
		a.onTreeSelect(node)
		return nil, action
	}

	// The tables tab has the same gap, and one click there already previews,
	// so the second press is read as the plain click tview took it for.
	if a.sidebarVisible && a.schemaTabs.current() == tabTables && a.tableList.InRect(x, y) {
		return ev, tview.MouseLeftClick
	}

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
	a.dispatch(keymap.ActionInspect)
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
	case zoneCopyResult:
		return a.dispatch(keymap.ActionCopyResult)
	case zoneTab:
		// Named, not cycled: the point of a strip is that you can go
		// straight to the one you can see.
		if pane := a.paneFor(row); pane != nil && z.index >= 0 && z.index < len(pane.names) {
			pane.show(pane.names[z.index])
			a.app.SetFocus(pane)
			return true
		}
	case zoneRegionName:
		if pane := a.paneFor(row); pane != nil {
			a.app.SetFocus(pane)
			return true
		}
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

// treeNodeAt is the node drawn on screen row y, found the way tview's own
// click handling finds it: the row within the box, plus how far the tree has
// been scrolled.
//
// Walking the tree rather than asking for the list is the only way in — the
// visible nodes are tview's private slice — and returning IsExpanded from
// the callback is what makes this walk visit exactly what was drawn, since
// that is the condition tview's own walk uses to descend.
func (a *App) treeNodeAt(y int) *tview.TreeNode {
	root := a.tree.GetRoot()
	if root == nil {
		return nil
	}

	_, rectY, _, _ := a.tree.GetInnerRect()
	want := y - rectY + a.tree.GetScrollOffset()
	if want < 0 {
		return nil
	}

	var found *tview.TreeNode
	at := 0
	root.Walk(func(node, _ *tview.TreeNode) bool {
		if at == want {
			found = node
		}
		at++
		return node.IsExpanded()
	})
	return found
}
