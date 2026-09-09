package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The spine is a single column of colour down the whole screen.
//
// It is a column rather than a row for two reasons. Terminals are wide and
// short, so a row is the scarcer thing to spend. And a field can be dropped:
// the bar sheds fields to fit, so a cue living among them can vanish on a
// narrow terminal. A column of the frame cannot be squeezed out.
//
// Its colours are pinned rather than named because the top bar's chip
// (topbar.go, chip()) renders in the same two colours to read as the frame's
// continuation; a named terminal colour would let the two drift apart the
// moment the terminal's palette changed.
var (
	spineColour = tcell.NewRGBColor(0x2E, 0x33, 0x3A)
	spineText   = tcell.NewRGBColor(0x9A, 0xA3, 0xAD)
)

// newSpine builds the column. A Box fills its rect with its background, which
// is the whole of what this has to do.
func newSpine() *tview.Box {
	return tview.NewBox().SetBackgroundColor(spineColour)
}

// colourTag renders a foreground/background pair as a tview colour tag.
//
// The values are emitted as hex rather than by name so that the pinned spine
// colours survive the trip through tview's tag parser; tcell downsamples them
// to the nearest palette entry on a terminal that cannot show them.
func colourTag(fg, bg tcell.Color) string {
	return fmt.Sprintf("#%06x:#%06x", fg.Hex(), bg.Hex())
}
