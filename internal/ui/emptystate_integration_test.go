//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// The first screen of a finder is the one a new user meets, and on an empty
// datasource it said "no matching tables" above "type part of a name" —
// a report on a search nobody had run, contradicting the invitation under it.
func TestAnUnsearchedFinderInvitesRatherThanReportsAFailure(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	// History, because a fresh session has genuinely run nothing — an empty
	// place rather than a search that failed.
	h.do(keymap.ActionSearchHistory)
	if !h.waitForScreen("nothing has been run yet") {
		t.Fatalf("the empty history did not say what it was:\n%s", h.text())
	}

	screen := h.text()
	for _, failure := range []string{"no matching", "matches \"\""} {
		if strings.Contains(screen, failure) {
			t.Errorf("a finder nobody has typed into reports a failed search:\n%s", screen)
		}
	}
}

// Once something has been typed the answer is a failure, and it names what was
// looked for so a typo is visible in the answer as well as in the field.
func TestASearchThatFindsNothingSaysWhatItLookedFor(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionGoToTable)
	if !h.waitForScreen("go to table") {
		t.Fatalf("the finder never opened:\n%s", h.text())
	}

	for _, r := range "zzqqxx" {
		h.inject(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	if !h.waitForScreen("zzqqxx") {
		t.Fatalf("the term never reached the field:\n%s", h.text())
	}
	if !h.waitForScreen(`no table matches "zzqqxx"`) {
		t.Errorf("a search that found nothing did not say so, or did not repeat the term:\n%s", h.text())
	}
}
