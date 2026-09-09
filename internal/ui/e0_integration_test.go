//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// Nothing else on screen says which schema an unqualified query will hit.
func TestStatusBarShowsTheCurrentSchema(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if !strings.Contains(h.text(), "@"+testmysql.DefaultDatabase) {
		t.Errorf("the status bar does not show the current schema:\n%s", h.text())
	}
}

// A session that assumed the default keyboard says which one, once. Anyone
// who wrote a config without a preset finds out here rather than by pressing
// a key that used to do something else.
func TestAnAssumedKeyboardIsAnnouncedOnce(t *testing.T) {
	h := newHarnessAssumingPreset(t, config.EnvDev)

	h.waitFor("the opening to name the assumed keyboard", func(a *App) bool {
		line, _ := a.status.renderWidth(200)
		return strings.Contains(line, "datagrip")
	})

	h.runCommand("keymap vim")

	// "in the palette for another" is the assumed-keyboard clause's own
	// wording — nothing else this package renders says it — so this pins the
	// clause going, not the word "keymap", which setPreset's own confirmation
	// also uses for an unrelated reason.
	h.waitFor("the announcement to go once a keyboard is chosen", func(a *App) bool {
		line, _ := a.status.renderWidth(200)
		return !strings.Contains(line, "in the palette for another")
	})
}
