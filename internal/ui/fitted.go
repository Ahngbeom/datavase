package ui

import (
	"strings"

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

	// heightFor is what the contents actually need at a width, or nil for a
	// dialog that should keep its full size whatever is in it.
	//
	// Without it the size was a maximum and nothing else, so six lines of row
	// detail sat in a box twenty-eight rows tall — and twenty-two empty rows
	// inside a border read as a pane that has not finished loading rather than
	// as one that is done.
	//
	// The finders deliberately do not set it. Their contents change on every
	// keystroke, and a box that grows and shrinks under the fingers is worse
	// to read than one with room to spare.
	heightFor func(width int) int
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
	if f.heightFor != nil {
		// Never below three: a border, one row of content, a border.
		h = min(h, max(3, f.heightFor(w)))
	}

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

// sizedTo tells the dialog how tall its contents are at a width.
func (f *fitted) sizedTo(heightFor func(width int) int) *fitted {
	f.heightFor = heightFor
	return f
}

// dialogHeight is how many rows a body of text needs inside a bordered box of
// the given outer width, counting the border.
//
// It measures visible cells rather than bytes: the text reaching here carries
// colour tags, and counting those as width would make every coloured line look
// wider than the reader sees it.
func dialogHeight(text string, width int) int {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}

	rows := 0
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		cells := visibleCost(line)
		if cells <= inner {
			rows++
			continue
		}
		rows += (cells + inner - 1) / inner
	}
	// The border above and below.
	return rows + 2
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
