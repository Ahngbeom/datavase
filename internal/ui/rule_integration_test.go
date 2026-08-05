//go:build integration

package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/rivo/tview"
)

// column reads what is drawn down one column of the screen.
func (h *harness) column(x int) []rune {
	h.t.Helper()
	h.settle()

	cells, width, height := h.screen.GetContents()
	out := make([]rune, 0, height)
	for row := 0; row < height; row++ {
		runes := []rune(string(cells[row*width+x].Bytes))
		if len(runes) == 0 {
			out = append(out, ' ')
			continue
		}
		out = append(out, runes[0])
	}
	return out
}

// Each region boundary is a hairline, and the hairlines meet. They used to
// meet without joining: the rule under the top bar ran straight through the
// column the sidebar's rule starts in, and the rule between the editor and the
// results stopped one cell short of it. On a terminal that reads as a
// rendering fault rather than as a layout — and the dialogs in this same
// application join their own corners, so the two sat on one screen disagreeing
// about what a corner looks like.
func TestTheHairlinesJoinWhereTheyMeet(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.showSidebar()

	var x, y, height int
	h.inspect(func(a *App) bool {
		x, y, _, height = a.sidebarRule.GetRect()
		return true
	})

	down := h.column(x)
	if got := down[y-1]; got != tview.Borders.TopT {
		t.Errorf("the sidebar's rule starts under a %q, want %q",
			string(got), string(tview.Borders.TopT))
	}
	if got := down[y+height]; got != tview.Borders.BottomT {
		t.Errorf("the sidebar's rule ends above a %q, want %q",
			string(got), string(tview.Borders.BottomT))
	}

	// Somewhere down the column, the rule between the editor and the results
	// arrives from the right.
	joins := 0
	for i := 0; i < height; i++ {
		if down[y+i] == tview.Borders.LeftT {
			joins++
		}
	}
	if joins != 1 {
		t.Errorf("the editor and result rule joins the sidebar's %d times, want once:\n%s",
			joins, h.text())
	}
}

// With the sidebar away there is nothing to join, and a junction in the middle
// of a plain rule is the same fault the other way round.
func TestAPlainRuleHasNoJunctions(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	cells, width, height := h.screen.GetContents()
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			for _, unwanted := range []rune{tview.Borders.TopT, tview.Borders.BottomT} {
				if []rune(string(cells[row*width+col].Bytes))[0] == unwanted {
					t.Errorf("a %q was drawn at row %d col %d with no sidebar to join:\n%s",
						string(unwanted), row, col, h.text())
				}
			}
		}
	}
}
