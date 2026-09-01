package ui

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// menuContext is a place a right click can land.
type menuContext int

const (
	ctxEditor menuContext = iota
	ctxResult
	ctxTree
	ctxTables
	ctxTopBar
	ctxStatusBar
)

func allMenuContexts() []menuContext {
	return []menuContext{ctxEditor, ctxResult, ctxTree, ctxTables, ctxTopBar, ctxStatusBar}
}

// menuEntry is one row: what it does, and the key that already does it.
type menuEntry struct {
	name string
	key  string
	run  func(a *App)
}

// menuEntries are the commands offered where the pointer is.
//
// They come from paletteCommands rather than from a table of their own, so
// there is no such thing as a menu-only command: anything seen here can be
// found again by name in the palette and on the ":" line. Four menu tables
// would be four more things to go stale, which is what startHere, helpGroups
// and vim.Reference() were each built to avoid.
func menuEntries(cmds []command, ctx menuContext, key func(keymap.Action) string) []menuEntry {
	var out []menuEntry
	for _, cmd := range cmds {
		for _, c := range cmd.contexts {
			if c != ctx {
				continue
			}
			entry := menuEntry{name: cmd.name, run: cmd.run}
			if cmd.covers != keymap.ActionNone {
				entry.key = key(cmd.covers)
			}
			out = append(out, entry)
			break
		}
	}
	return out
}

// menuKeyGap is the least space between a name and its key that still reads
// as two columns rather than one run of words.
const menuKeyGap = 2

// layoutMenu renders the rows at a width.
//
// The key column goes before a name is abbreviated: a key can be looked up in
// the palette, while a cut name is the whole of the row gone.
func layoutMenu(entries []menuEntry, width int) []string {
	widest := 0
	for _, e := range entries {
		if n := visibleCost(e.name); n > widest {
			widest = n
		}
	}
	keyColumn := 0
	for _, e := range entries {
		if n := visibleCost(e.key); n > keyColumn {
			keyColumn = n
		}
	}

	withKeys := keyColumn > 0 && widest+menuKeyGap+keyColumn <= width

	rows := make([]string, 0, len(entries))
	for _, e := range entries {
		name := result.Truncate(e.name, width)
		if !withKeys {
			rows = append(rows, result.EscapeTags(name))
			continue
		}
		pad := strings.Repeat(" ", widest-visibleCost(name)+menuKeyGap)
		rows = append(rows, fmt.Sprintf("%s%s%s",
			result.EscapeTags(name), pad, tag(colourMuted, e.key)))
	}
	return rows
}

// menuMaxContentWidth caps how wide a menu grows for a long name — a right
// click menu answers "what can I do here", not "read a paragraph", and a
// handful of short rows is what it is for.
const menuMaxContentWidth = 40

// menuContentWidth is the budget layoutMenu actually lays a row out at.
//
// It is the terminal's own width, capped at menuMaxContentWidth, minus the
// two columns the border costs — not the constant alone. layoutMenu decides
// whether a row keeps its key column against whatever budget it is given;
// deciding that against a fixed 40 regardless of how wide the terminal
// actually is meant the choice was made against the wrong number, and
// menu.Draw's own clamp then clipped the oversized row instead of
// layoutMenu dropping the key cleanly — the one substitution the whole
// design exists to make instead of an abbreviation.
func (a *App) menuContentWidth() int {
	width := menuMaxContentWidth
	if a.screen != nil {
		if screenW, _ := a.screen.Size(); screenW > 0 {
			width = min(width, screenW-2)
		}
	}
	return max(1, width)
}

// contextAt resolves a screen position to the place a right click landed.
//
// It checks the same widgets a left click already reaches rather than the
// hitmap, because the hitmap only carries zones for header rows — a right
// click inside a pane's body, which is most of the screen, has no zone at
// all.
//
// The tree and the tables list share one region, swapped by a.schemaTabs,
// and the grid shares its own region with the DDL, plan and sessions tabs,
// swapped by a.resultTabs; tview does not reset a hidden page's rect when
// it stops being drawn, so whichever of a region's tabs was last on top
// keeps that region's coordinates and InRect on it keeps matching there
// too. Checking which tab is actually current, not just which rect a
// position falls in, is what a stale rect from a sibling tab cannot fool.
func (a *App) contextAt(x, y int) menuContext {
	switch {
	case a.resultTabs.current() == tabResults && a.grid.InRect(x, y):
		return ctxResult
	case a.schemaTabs.current() == tabTree && a.tree.InRect(x, y):
		return ctxTree
	case a.schemaTabs.current() == tabTables && (a.tableList.InRect(x, y) || a.tableFilter.InRect(x, y)):
		return ctxTables
	case a.topBar.InRect(x, y):
		return ctxTopBar
	case a.statusBar.InRect(x, y):
		return ctxStatusBar
	}
	return ctxEditor
}

// focusContext moves focus onto what a context's commands expect to be
// looking at, and — for the grid and the tree — moves the selection to
// where the click actually landed.
//
// A right click does not move focus or selection the way tview's own click
// handling does for a left click — bindMouse swallows it before that runs.
// Without the focus half, "inspect" chosen from a menu opened on a result
// row would read the focus left over from wherever the pointer was before
// and show a table's definition instead of the row, because inspect and
// selectedTable both decide what to show by asking which widget has focus.
// Without the selection half, right-clicking a row that is not the one
// already selected and choosing "copy row" would copy the wrong row — the
// menu's whole promise of acting on "here" would be false in the context it
// is used in most.
func (a *App) focusContext(ctx menuContext, x, y int) {
	switch ctx {
	case ctxResult:
		if row, col := a.grid.CellAt(x, y); row >= 1 && col >= 0 {
			a.grid.Select(row, col)
		}
		a.app.SetFocus(a.grid)
	case ctxTree:
		if node := a.treeNodeAt(y); node != nil {
			a.tree.SetCurrentNode(node)
		}
		a.app.SetFocus(a.tree)
	case ctxTables:
		if index := a.tableItemAt(y); index >= 0 {
			a.tableList.SetCurrentItem(index)
		}
		a.app.SetFocus(a.tableList)
	case ctxEditor:
		a.app.SetFocus(a.editor)
	}
}

// showMenu opens the palette's own commands, filtered to where the click
// landed.
//
// It is built as its own widget rather than newSearchBox: a menu has no
// search field to type into, and a handful of rows positioned at the click is
// a different shape of thing than a filtered, centred list.
func (a *App) showMenu(ctx menuContext, x, y int) {
	entries := menuEntries(paletteCommands(), ctx, a.keyLabel)
	if len(entries) == 0 {
		return
	}

	rows := layoutMenu(entries, a.menuContentWidth())

	width := 0
	for _, r := range rows {
		if n := visibleCost(r); n > width {
			width = n
		}
	}

	list := tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)
	for i, row := range rows {
		run := entries[i].run
		list.AddItem(row, "", 0, func() {
			a.closeMenu()
			a.focusContext(ctx, x, y)
			run(a)
		})
	}
	list.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			a.closeMenu()
			return nil
		}
		return ev
	})
	list.SetBorder(true)
	list.SetBackgroundColor(tcell.ColorBlack)

	// +2 on each axis for the border. There is no height cap: a context menu
	// is at most a handful of rows, unlike the browsable dialogs that need
	// one to keep from running off a small terminal.
	a.pages.AddPage(pageMenu, newMenu(list, x, y, width+2, len(entries)+2), true, true)
	a.app.SetFocus(list)
}

func (a *App) closeMenu() {
	a.pages.RemovePage(pageMenu)
	a.app.SetFocus(a.editor)
}

// menu floats a primitive at a screen position, flipping about the point so
// a menu opened near the right or bottom edge stays fully visible instead of
// being clipped by it — the same problem fitted solves by centring instead.
type menu struct {
	*tview.Box
	content       tview.Primitive
	x, y          int
	width, height int
}

func newMenu(content tview.Primitive, x, y, width, height int) *menu {
	return &menu{Box: tview.NewBox(), content: content, x: x, y: y, width: width, height: height}
}

func (m *menu) Draw(screen tcell.Screen) {
	_, _, screenW, screenH := m.GetRect()

	w := min(m.width, screenW)
	h := min(m.height, screenH)

	x, y := m.x, m.y
	if x+w > screenW {
		x = m.x - w
	}
	if x < 0 {
		x = 0
	}
	if y+h > screenH {
		y = m.y - h
	}
	if y < 0 {
		y = 0
	}

	m.content.SetRect(x, y, w, h)
	m.content.Draw(screen)
}

func (m *menu) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.content.InputHandler()
}

func (m *menu) Focus(delegate func(p tview.Primitive)) { delegate(m.content) }

func (m *menu) HasFocus() bool { return m.content.HasFocus() }

func (m *menu) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return m.content.MouseHandler()
}
