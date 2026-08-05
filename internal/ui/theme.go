package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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

// headingTag marks a line that names the lines beneath it — a group in the key
// reference, a category in the palette.
//
// Weight rather than colour. A heading is not a state the user could forget
// they are in, and drawing it in the notice colour spent that cue on the word
// "Editing"; there was then no colour left that meant "an injected LIMIT" and
// nothing else. Bold also survives a monochrome terminal, which is the same
// reason the active tab keeps its "▸" as well as its colour.
func headingTag(text string) string {
	return "[::b]" + text + "[::-]"
}

// applyTheme claims the library defaults this interface has an opinion about.
//
// tview reaches for its own palette wherever a widget was not told otherwise,
// and two of those defaults were showing: green for a placeholder and for a
// list's second line, blue behind a Modal's border. Setting them per widget is
// a rule every future call site has to remember, which is exactly how the
// green got in — so the defaults are claimed once, here, and the roles above
// stay the whole palette.
//
// It writes package-level state, which is what tview offers and what tview
// expects; there is one interface per process.
func applyTheme() {
	// Placeholder text and a list's secondary line. Both are the trailing
	// detail this file already assigns to colourMuted, and neither has ever
	// been a success.
	tview.Styles.TertiaryTextColor = colourMuted
}
