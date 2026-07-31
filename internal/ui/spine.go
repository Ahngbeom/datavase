package ui

import (
	"fmt"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The environment spine is a single column of colour down the whole screen.
//
// It replaces a badge that lived among the status bar's fields, and it is a
// column rather than a row for two reasons. Terminals are wide and short, so a
// row is the scarcer thing to spend. And a field can be dropped: the bar sheds
// fields to fit, so on a narrow terminal the one cue that mattered was the one
// that disappeared. A column of the frame cannot be squeezed out.
//
// These are the only pinned colours in the interface. Everything else is a
// named colour and follows the user's terminal theme, but a production cue that
// goes pale because someone softened their theme's red is not a cue.
var (
	spineProd  = tcell.NewRGBColor(0xC2, 0x3B, 0x30)
	spineStage = tcell.NewRGBColor(0x8A, 0x5A, 0x12)
	spineDev   = tcell.NewRGBColor(0x2E, 0x33, 0x3A)
)

// Text drawn on the spine's colour, in the environment chip at the top left.
// Production and stage are loud on purpose; dev is the ninety-five per cent
// case and stays quiet, which is what leaves the other two any force.
var (
	spineTextLoud  = tcell.ColorWhite
	spineTextQuiet = tcell.NewRGBColor(0x9A, 0xA3, 0xAD)
)

// envStyle is how one environment is drawn wherever it appears.
type envStyle struct {
	fg, bg tcell.Color
}

func envStyleFor(env config.Env) envStyle {
	switch env {
	case config.EnvProd:
		return envStyle{fg: spineTextLoud, bg: spineProd}
	case config.EnvStage:
		return envStyle{fg: spineTextLoud, bg: spineStage}
	default:
		return envStyle{fg: spineTextQuiet, bg: spineDev}
	}
}

// newSpine builds the column. A Box fills its rect with its background, which
// is the whole of what this has to do — and the environment cannot change
// mid-session, so the colour is decided once.
func newSpine(env config.Env) *tview.Box {
	return tview.NewBox().SetBackgroundColor(envStyleFor(env).bg)
}

// colourTag renders a foreground/background pair as a tview colour tag.
//
// The values are emitted as hex rather than by name so that the pinned spine
// colours survive the trip through tview's tag parser; tcell downsamples them
// to the nearest palette entry on a terminal that cannot show them.
func colourTag(fg, bg tcell.Color) string {
	return fmt.Sprintf("#%06x:#%06x", fg.Hex(), bg.Hex())
}
