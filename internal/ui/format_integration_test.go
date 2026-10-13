//go:build integration

package ui

import (
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/gdamore/tcell/v2"
)

// A query copied on macOS arrives in a bracketed paste with LF line ends,
// which tcell reports as Ctrl+J; the query used to land on one line.
func TestAPastedQueryKeepsItsLines(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.buffer("", 0)

	events := []tcell.Event{tcell.NewEventPaste(true)}
	for i, line := range []string{"SELECT a", "FROM t"} {
		if i > 0 {
			events = append(events, tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl))
		}
		for _, r := range line {
			events = append(events, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		}
	}
	events = append(events, tcell.NewEventPaste(false))
	// The simulation's queue is short; a paste is longer than it.
	for _, ev := range events {
		for h.screen.PostEvent(ev) != nil {
			time.Sleep(time.Millisecond)
		}
	}

	h.waitFor("the pasted query, line breaks intact", func(a *App) bool {
		return a.editor.GetText() == "SELECT a\nFROM t"
	})
}
