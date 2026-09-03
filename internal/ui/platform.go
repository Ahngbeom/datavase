package ui

import (
	"os"
	"runtime"
)

// macLabels decides whether key labels use the Apple glyphs (⌘, ⇧, ⌫) or the
// spelled-out names.
//
// $TMUX unset is the signal, not the terminal type: tmux does not forward ⌘
// to the process it hosts at all, so inside a session the Ctrl form is the
// only one of the pair that can actually arrive — labelling the glyph would
// teach a key that does nothing and is exactly how "⌘ doesn't work" gets
// reported against a machine where ⌘ works everywhere else.
func macLabels(goos, tmux string) bool {
	return goos == "darwin" && tmux == ""
}

// onMac decides whether key labels use the Apple glyphs (⌘, ⇧, ⌫) or the
// spelled-out names. It is a variable so tests can render both.
var onMac = macLabels(runtime.GOOS, os.Getenv("TMUX"))
