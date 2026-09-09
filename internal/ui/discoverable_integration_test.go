//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// Two features that worked and could not be found: stepping between rows
// without leaving the row view, and the dot that says which schema an
// unqualified statement will reach.
func TestTheRowViewShowsHowToStepAndHowToLeave(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 1 AS a UNION ALL SELECT 2", 2)

	h.do(keymap.ActionNextPane)
	h.waitFor("the grid", func(a *App) bool { return a.app.GetFocus() == a.grid })
	h.do(keymap.ActionInspect)

	if !h.waitForScreen("row 1 of 2") {
		t.Fatalf("the row view never opened:\n%s", h.text())
	}
	screen := h.text()
	for _, want := range []string{"j/k step", "Esc close"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the row view does not mention %q:\n%s", want, screen)
		}
	}
}

func TestTheSchemaTreeExplainsItsMarker(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.showSidebar()

	if !h.waitForScreen(currentSchemaMarker + " current schema") {
		t.Errorf("the schema pane does not explain its marker:\n%s", h.text())
	}
}
