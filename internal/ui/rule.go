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
// are made of the same stroke.
type rule struct {
	*tview.Box
	vertical bool
}

func newRule(vertical bool) *rule {
	return &rule{Box: tview.NewBox(), vertical: vertical}
}

func (r *rule) Draw(screen tcell.Screen) {
	r.Box.DrawForSubclass(screen, r)

	x, y, width, height := r.GetInnerRect()
	style := tcell.StyleDefault.
		Foreground(colourMuted).
		Background(r.GetBackgroundColor())

	if r.vertical {
		for i := 0; i < height; i++ {
			screen.SetContent(x, y+i, tview.Borders.Vertical, nil, style)
		}
		return
	}
	for i := 0; i < width; i++ {
		screen.SetContent(x+i, y, tview.Borders.Horizontal, nil, style)
	}
}
