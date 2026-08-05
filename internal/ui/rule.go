package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// rule is the hairline that separates two regions.
//
// It replaces the box each region used to draw around itself. Three boxes cost
// six rows and six columns of pure chrome on a terminal where rows are the
// scarcest thing there is, and their titles said what the tab strip two rows
// below already said. One line does the same work and says nothing.
//
// It draws tview's own border runes so a region boundary and a dialog's edge
// are made of the same stroke — and, where two rules meet, tview's own
// junctions, so they are made of the same corner as well.
type rule struct {
	*tview.Box
	vertical bool
}

func newRule(vertical bool) *rule {
	return &rule{Box: tview.NewBox(), vertical: vertical}
}

// Draw lays the line down and mends whatever joint it arrives at.
//
// Neither rule can know which of them is drawn first: tview's Flex defers the
// child that holds focus to the end of the frame, so the rule below the body
// is laid down before the sidebar's and the rule above it afterwards. Each
// rule therefore looks in every direction a joint could come from and draws
// the glyph the pair of them add up to. Whichever goes last is right, and the
// one that went first was right too.
//
// The upshot is that no rule has to be told about the layout: a joint exists
// exactly where two hairlines are found touching.
func (r *rule) Draw(screen tcell.Screen) {
	r.Box.DrawForSubclass(screen, r)

	x, y, width, height := r.GetInnerRect()
	style := tcell.StyleDefault.
		Foreground(colourMuted).
		Background(r.GetBackgroundColor())

	if r.vertical {
		// A horizontal rule ending against either end of this column shares
		// that cell with it, and the shared cell is a T pointing this way.
		if y > 0 && drawnRule(screen, x, y-1) == tview.Borders.Horizontal {
			screen.SetContent(x, y-1, tview.Borders.TopT, nil, style)
		}
		if drawnRule(screen, x, y+height) == tview.Borders.Horizontal {
			screen.SetContent(x, y+height, tview.Borders.BottomT, nil, style)
		}
		for i := 0; i < height; i++ {
			screen.SetContent(x, y+i, tview.Borders.Vertical, nil, style)
		}
		return
	}

	// A rule that begins beside a vertical one shares no cell with it, so this
	// junction belongs to the column on its left rather than to any cell of
	// this rule.
	if x > 0 && drawnRule(screen, x-1, y) == tview.Borders.Vertical {
		screen.SetContent(x-1, y, tview.Borders.LeftT, nil, style)
	}
	for i := 0; i < width; i++ {
		above := drawnRule(screen, x+i, y-1) == tview.Borders.Vertical && y > 0
		below := drawnRule(screen, x+i, y+1) == tview.Borders.Vertical

		glyph := tview.Borders.Horizontal
		switch {
		case above && below:
			glyph = tview.Borders.Cross
		case above:
			glyph = tview.Borders.BottomT
		case below:
			glyph = tview.Borders.TopT
		}
		screen.SetContent(x+i, y, glyph, nil, style)
	}
}

// drawnRule is the hairline already on screen at a cell, or zero for anything
// that is not one.
//
// The colour is part of the question. A tree's own graphics are box drawing
// too, and so is anything the user has typed into the editor; joining onto one
// of those would put a junction in the middle of somebody's text.
func drawnRule(screen tcell.Screen, x, y int) rune {
	if x < 0 || y < 0 {
		return 0
	}
	mainc, _, style, _ := screen.GetContent(x, y)
	if fg, _, _ := style.Decompose(); fg != colourMuted {
		return 0
	}
	return mainc
}
