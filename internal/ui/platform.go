package ui

import "runtime"

// onMac decides whether key labels use the Apple glyphs (⌘, ⇧, ⌫) or the
// spelled-out names. It is a variable so tests can render both.
var onMac = runtime.GOOS == "darwin"
