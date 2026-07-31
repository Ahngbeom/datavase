package ui

import "github.com/gdamore/tcell/v2"

// Colours: one role, one value.
//
// There used to be six names for four values — OK and dev were the same green,
// an error and production the same red. That is how the environment cue, the
// one visual thing standing between the user and a production mistake, came to
// be indistinguishable from an ordinary error message. Each name below is a
// distinct role, and the environment shares with none of them.
//
// These are named colours rather than pinned ones, so they inherit whatever
// palette the terminal was configured with. The environment spine is the sole
// exception; see spine.go for why.
const (
	// colourAccent marks what has focus: the focused region's header, the
	// active tab, a result's column names.
	colourAccent = tcell.ColorAqua

	// colourNotice is for a state the user could forget they are in — an
	// injected LIMIT, a truncated result, unlocked writes.
	colourNotice = tcell.ColorYellow

	// colourDanger is for failures, and is foreground only. The production
	// spine uses the same hue as a background, which reads as related rather
	// than as the same thing.
	colourDanger = tcell.ColorRed

	// colourMuted is everything structural: the rules between regions,
	// inactive tabs, NULL, and any trailing line of detail.
	colourMuted = tcell.ColorGray
)
