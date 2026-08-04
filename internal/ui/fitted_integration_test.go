//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// resize changes the simulated terminal and lets the interface redraw.
func (h *harness) resize(width, height int) {
	h.t.Helper()
	h.screen.SetSize(width, height)
	h.settle()
}

// borderIntact reports whether the drawn screen contains a complete box.
//
// Both corner sets are accepted: tview draws a focused primitive with double
// lines and an unfocused one with single lines, and a dialog is focused by
// definition.
func borderIntact(screen string, width int) bool {
	const (
		topLeft     = "┌╔"
		bottomRight = "┘╝"
	)

	var hasTop, hasBottom bool
	for _, line := range strings.Split(screen, "\n") {
		if len([]rune(line)) > width {
			return false
		}
		if strings.ContainsAny(line, topLeft) {
			hasTop = true
		}
		if strings.ContainsAny(line, bottomRight) {
			hasBottom = true
		}
	}
	return hasTop && hasBottom
}

// The reported failure: on a small terminal the key reference was clipped.
// A fixed-size dialog is simply cut off by tview rather than shrunk.
func TestHelpFitsASmallTerminal(t *testing.T) {
	sizes := []struct{ width, height int }{
		{width: 120, height: 40},
		{width: 80, height: 24},
		{width: 60, height: 20},
		{width: 40, height: 15},
	}

	for _, size := range sizes {
		name := itoa(size.width) + "x" + itoa(size.height)
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, config.EnvDev)
			h.resize(size.width, size.height)

			h.do(keymap.ActionHelp)

			got := h.text()
			if !borderIntact(got, size.width) {
				t.Errorf("the help dialog is clipped at %s:\n%s", name, got)
			}
			// And it is actually the help, not an empty frame.
			if !strings.Contains(got, "keys") {
				t.Errorf("the help title is missing at %s:\n%s", name, got)
			}
		})
	}
}

// A dialog opened on a large screen must follow the window down.
func TestDialogFollowsAResize(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.resize(120, 40)

	h.do(keymap.ActionHelp)
	if !borderIntact(h.text(), 120) {
		t.Fatalf("the dialog is already clipped at 120x40:\n%s", h.text())
	}

	h.resize(50, 18)

	if !borderIntact(h.text(), 50) {
		t.Errorf("the dialog did not shrink with the window:\n%s", h.text())
	}
}

// Every dialog goes through the same wrapper, so each one has to survive a
// small screen.
func TestEveryDialogFitsASmallTerminal(t *testing.T) {
	dialogs := []struct {
		name   string
		action keymap.Action
		setup  func(h *harness)
		// open replaces the key press for a dialog that has no binding of its
		// own. The first-run card is one: it appears by itself, and is reached
		// again only by name.
		open func(h *harness)
	}{
		{name: "help", action: keymap.ActionHelp},
		{name: "command palette", action: keymap.ActionCommandPalette},
		{name: "history", action: keymap.ActionSearchHistory},
		{name: "go to table", action: keymap.ActionGoToTable},
		{
			name:   "completion",
			action: keymap.ActionComplete,
			setup: func(h *harness) {
				h.seedCache(completionSnapshot())
				h.typeSQL("SELECT * FROM customer")
			},
		},
		{
			name:   "confirmation",
			action: keymap.ActionRun,
			setup:  func(h *harness) { h.typeSQL("DELETE FROM dv_seq") },
		},
		{
			name: "getting started",
			open: func(h *harness) { h.app.app.QueueUpdateDraw(h.app.showIntro) },
		},
	}

	for _, d := range dialogs {
		t.Run(d.name, func(t *testing.T) {
			h := newHarness(t, config.EnvDev)
			if d.setup != nil {
				d.setup(h)
			}
			h.resize(50, 16)

			if d.open != nil {
				d.open(h)
			} else {
				h.do(d.action)
			}

			if !borderIntact(h.text(), 50) {
				t.Errorf("the %s dialog is clipped at 50x16:\n%s", d.name, h.text())
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
