package ui

import "github.com/gdamore/tcell/v2"

// Colours are kept together so the environment badge, which is the one
// visual cue standing between the user and a production mistake, cannot
// drift out of step with the rest of the interface.
const (
	colourHeader = tcell.ColorAqua
	colourNull   = tcell.ColorGray
	colourError  = tcell.ColorRed
	colourNotice = tcell.ColorYellow
	colourOK     = tcell.ColorGreen
	colourDim    = tcell.ColorGray
)

// Tab strip colours.
const (
	colourTabActive = tcell.ColorAqua
	colourTabIdle   = tcell.ColorGray
)

// Environment badge colours: production is red everywhere it appears.
const (
	colourEnvProd  = tcell.ColorRed
	colourEnvStage = tcell.ColorYellow
	colourEnvDev   = tcell.ColorGreen
)
