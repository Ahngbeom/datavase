//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// The bar says "running… ^C cancels" and the result pane two rows above it
// used to say "run a statement to see rows here" — at the moment the user is
// watching the screen hardest.
func TestTheResultPaneDoesNotAskForAStatementWhileOneRuns(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT SLEEP(2)")
	h.do(keymap.ActionRun)
	h.waitFor("the statement to be in flight", func(a *App) bool { return a.running != nil })

	screen := h.text()
	if strings.Contains(screen, "run a statement to see rows here") {
		t.Errorf("the result pane asks for a statement while one is running:\n%s", screen)
	}
	if !strings.Contains(screen, "waiting for the first row") {
		t.Errorf("the result pane says nothing about the statement in flight:\n%s", screen)
	}

	h.do(keymap.ActionCancel)
	h.waitFor("the statement to stop", func(a *App) bool { return a.running == nil })
}

// A write leaves the grid empty for a reason. Telling the user to run a
// statement, immediately after one ran and the bar reported what it changed,
// reads as the write having gone nowhere.
func TestTheResultPaneSaysWhyAWriteLeftNoRows(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	seedRows(t, h, 1)

	h.typeSQL("UPDATE dv_ui SET n = 2 WHERE n = 1")
	h.do(keymap.ActionRun)
	confirmWrite(t, h)
	h.waitFor("the write to finish", func(a *App) bool { return a.status.written != nil })

	screen := h.text()
	if strings.Contains(screen, "run a statement to see rows here") {
		t.Errorf("the result pane asks for a statement just after a write ran:\n%s", screen)
	}
	if !strings.Contains(screen, "no rows: that statement changed data") {
		t.Errorf("the result pane does not say why it is empty:\n%s", screen)
	}
}
