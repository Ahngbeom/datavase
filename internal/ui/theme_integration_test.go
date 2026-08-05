//go:build integration

package ui

import (
	"fmt"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// theme.go declares four roles and gives each one value, and the spine adds
// three pinned ones. Anything else on screen is a colour nobody chose — which
// is how the empty editor's placeholder came to be green, the colour every
// other program on the machine uses for success.
func (h *harness) unclaimedForegrounds() map[tcell.Color]string {
	h.t.Helper()
	h.settle()

	claimed := map[tcell.Color]bool{
		tcell.ColorDefault: true,
		tcell.ColorBlack:   true,
		// The four roles.
		colourAccent: true,
		colourNotice: true,
		colourDanger: true,
		colourMuted:  true,
		// Text drawn on the spine's colour, and on a selection.
		spineTextLoud:    true,
		spineTextQuiet:   true,
		tcell.ColorWhite: true,
	}

	found := map[tcell.Color]string{}
	cells, width, height := h.screen.GetContents()
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			c := cells[row*width+col]
			fg, _, _ := c.Style.Decompose()
			if claimed[fg] {
				continue
			}
			if _, seen := found[fg]; !seen {
				found[fg] = fmt.Sprintf("row %d col %d %q", row, col, string(c.Bytes))
			}
		}
	}
	return found
}

func TestTheEmptyEditorDrawsNoColourThisApplicationDidNotChoose(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	for colour, where := range h.unclaimedForegrounds() {
		t.Errorf("%v is on screen at %s and is not one of this interface's roles:\n%s",
			colour, where, h.text())
	}
}

// The finders show a second line under each row, which is the other place
// tview's green reached.
func TestAFinderDrawsNoColourThisApplicationDidNotChoose(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionGoToTable)
	if !h.waitForScreen("go to table") {
		t.Fatalf("the finder never opened:\n%s", h.text())
	}

	for colour, where := range h.unclaimedForegrounds() {
		t.Errorf("%v is on screen at %s and is not one of this interface's roles:\n%s",
			colour, where, h.text())
	}
}
