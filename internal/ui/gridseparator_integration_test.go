//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// A grid with nothing between its columns is expansion padding pretending
// to be structure — someone arriving from DataGrip cannot tell where one
// cell ends, at exactly the moment that matters most: about to copy it.
func TestTheGridDrawsAVerticalRuleBetweenColumns(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1 AS a, 2 AS b")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.buf.RowCount() > 0 })

	if !h.waitForScreen("│") {
		t.Errorf("no column separator is drawn on the grid:\n%s", h.text())
	}

	screen := h.text()
	found := false
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, "1") && strings.Contains(line, "2") && strings.Contains(line, "│") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no data row carries a separator between its values:\n%s", screen)
	}
}
