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

		switch action {
		case tview.MouseLeftClick:
			return a.mouseLeftClick(ev, action)
		}
		return ev, action
	})
}

// mouseLeftClick resolves a left click against the hitmap, and hands it back
// untouched when nothing claims it.
func (a *App) mouseLeftClick(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	// While a dialog is open it owns the pointer, the way it owns the
	// keyboard.
	if name, _ := a.pages.GetFrontPage(); name != pageMain {
		return ev, action
	}

	x, y := ev.Position()
	z, ok := a.hits.at(x, y)
	if !ok {
		return ev, action
	}
	if !a.mouseAction(z) {
		return ev, action
	}
	// Swallowed: a header click that also reached tview would select the row
	// under it as well as doing what was asked.
	return nil, action
}

// mouseAction performs a zone's click and reports whether it consumed it.
//
// Every branch reaches the same function the key does. That is what lets the
// tests written as h.do(action) stand as this layer's regression net, and
// what stops the mouse coming to do more or less than the keyboard.
func (a *App) mouseAction(z zone) bool {
	switch z.target {
	case zoneDataSource:
		return a.dispatch(keymap.ActionSwitchDataSource)
	case zoneSchema:
		return a.dispatch(keymap.ActionUseSchema)
	case zoneHelp:
		return a.dispatch(keymap.ActionHelp)
	}
	return false
}
