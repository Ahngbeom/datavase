package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// pasted feeds a bracketed paste through pasteScreen the way tcell reports
// one, and returns the keys tview's paste buffer would see.
func pasted(t *testing.T, keys ...*tcell.EventKey) []*tcell.EventKey {
	t.Helper()

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(sim.Fini)

	screen := &pasteScreen{Screen: sim}
	events := append([]tcell.Event{tcell.NewEventPaste(true)}, eventsOf(keys)...)
	events = append(events, tcell.NewEventPaste(false))
	for _, ev := range events {
		if err := sim.PostEvent(ev); err != nil {
			t.Fatalf("PostEvent() error = %v", err)
		}
	}

	var out []*tcell.EventKey
	for {
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventKey:
			out = append(out, ev)
		case *tcell.EventPaste:
			if ev.End() {
				return out
			}
		}
	}
}

func eventsOf(keys []*tcell.EventKey) []tcell.Event {
	out := make([]tcell.Event, len(keys))
	for i, k := range keys {
		out[i] = k
	}
	return out
}

func lineFeed() *tcell.EventKey { return tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl) }
func carriageReturn() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
}
func letter(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

func enters(keys []*tcell.EventKey) int {
	n := 0
	for _, k := range keys {
		if k.Key() == tcell.KeyEnter {
			n++
		}
	}
	return n
}

// A query copied on macOS separates its lines with LF. tcell reports that as
// Ctrl+J, which tview's paste buffer throws away, and the query arrived as
// one line.
func TestAPastedLineFeedStillBreaksTheLine(t *testing.T) {
	got := pasted(t, letter('a'), lineFeed(), letter('b'))

	if enters(got) != 1 {
		t.Errorf("a pasted LF produced %d line breaks, want 1", enters(got))
	}
}

// Text copied on Windows carries CRLF; turning both halves into a break would
// double-space every pasted query.
func TestAPastedCRLFIsOneLineBreak(t *testing.T) {
	got := pasted(t, letter('a'), carriageReturn(), lineFeed(), letter('b'))

	if enters(got) != 1 {
		t.Errorf("a pasted CRLF produced %d line breaks, want 1", enters(got))
	}
}

// Outside a paste Ctrl+J is the stand-in for Ctrl+Enter, and turning it into
// Enter would make "run" type a newline instead.
func TestCtrlJTypedOutsideAPasteIsLeftAlone(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(sim.Fini)

	screen := &pasteScreen{Screen: sim}
	if err := sim.PostEvent(lineFeed()); err != nil {
		t.Fatalf("PostEvent() error = %v", err)
	}

	ev, ok := screen.PollEvent().(*tcell.EventKey)
	if !ok || ev.Key() != tcell.KeyCtrlJ {
		t.Errorf("Ctrl+J outside a paste became %v", ev)
	}
}
