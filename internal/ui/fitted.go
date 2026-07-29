package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// dialogMargin is the space kept between a dialog and the screen edge, so a
// shrunk dialog still reads as floating above the interface rather than
// replacing it.
const dialogMargin = 2

// fitted centres a primitive and shrinks it to whatever room there is.
//
// The size is decided in Draw rather than when the dialog is built. A tview
// Flex given a fixed size larger than its parent simply clips the child — it
// does not shrink it — which is why the help screen was cut off on smaller
// terminals. Deciding per frame also means a dialog follows the window when
// it is resized while open.
type fitted struct {
	*tview.Box
	content tview.Primitive

	// maxWidth and maxHeight are the preferred size; the dialog never grows
	// past them even on a large screen, because a key reference stretched
	// across a wide terminal is harder to read, not easier.
	maxWidth, maxHeight int
}

func newFitted(content tview.Primitive, maxWidth, maxHeight int) *fitted {
	return &fitted{
		Box:       tview.NewBox(),
		content:   content,
		maxWidth:  maxWidth,
		maxHeight: maxHeight,
	}
}

func (f *fitted) Draw(screen tcell.Screen) {
	f.Box.DrawForSubclass(screen, f)

	x, y, width, height := f.GetInnerRect()

	w := min(f.maxWidth, width-2*dialogMargin)
	h := min(f.maxHeight, height-2*dialogMargin)

	// On a screen too small for any margin, use everything available rather
	// than collapsing to nothing.
	if w < 1 {
		w = max(1, width)
	}
	if h < 1 {
		h = max(1, height)
	}

	f.content.SetRect(x+(width-w)/2, y+(height-h)/2, w, h)
	f.content.Draw(screen)
}

func (f *fitted) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return f.content.InputHandler()
}

func (f *fitted) Focus(delegate func(p tview.Primitive)) {
	delegate(f.content)
}

func (f *fitted) HasFocus() bool {
	return f.content.HasFocus()
}

func (f *fitted) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return f.content.MouseHandler()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
