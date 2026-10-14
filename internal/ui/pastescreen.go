package ui

import "github.com/gdamore/tcell/v2"

// pasteScreen keeps the line breaks in a bracketed paste.
//
// Inside a paste tcell reports a bare line feed — what macOS clipboards hold —
// as Ctrl+J, and tview's paste buffer keeps only runes, Enter and Tab, so a
// pasted query arrived as one line. Terminals that rewrite LF to CR on paste
// hide this; Ghostty, WezTerm and kitty send it as it is.
//
// tview reads the screen only through PollEvent, which is why the fix sits
// here: pasted keys never reach an input capture.
type pasteScreen struct {
	tcell.Screen
	pasting bool
	// afterCR is set when the last pasted key was a carriage return, so the
	// LF of a CRLF pair is not counted as a second break.
	afterCR bool
}

func (s *pasteScreen) PollEvent() tcell.Event {
	for {
		ev := s.Screen.PollEvent()

		switch e := ev.(type) {
		case *tcell.EventPaste:
			s.pasting = e.Start()
			s.afterCR = false
		case *tcell.EventKey:
			if !s.pasting {
				return ev
			}
			afterCR := s.afterCR
			s.afterCR = e.Key() == tcell.KeyEnter
			if e.Key() == tcell.KeyCtrlJ {
				if afterCR {
					continue
				}
				return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			}
		}
		return ev
	}
}
