//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

// The reference answers "which key does X" and never answered "what do I do
// now". Five keys at the top do, and they have to be the first thing on the
// screen rather than the seventh group down.
func TestTheKeyReferenceOpensWithTheFewKeysToStartFrom(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	var help string
	h.inspect(func(a *App) bool {
		help = a.helpText()
		return true
	})

	start := strings.Index(help, "Start here")
	if start < 0 {
		t.Fatalf("the key reference has no opening section:\n%s", help)
	}
	if first := strings.Index(help, helpGroups[0].title); first >= 0 && start > first {
		t.Error("the reference groups are printed before the keys to start from")
	}

	// Generated from the live map rather than written out, so a rebound key is
	// still named correctly here.
	h.inspect(func(a *App) bool {
		opening := help[start:]
		if next := strings.Index(opening, helpGroups[0].title); next > 0 {
			opening = opening[:next]
		}
		for _, action := range startHere {
			bindings := a.keys.DisplayBindings(action)
			if len(bindings) == 0 {
				continue
			}
			if !strings.Contains(opening, bindings[0].Label(onMac)) {
				t.Errorf("Start here does not name the key bound to %s:\n%s", action, opening)
			}
			if !strings.Contains(opening, action.Describe()) {
				t.Errorf("Start here lists %s with no description:\n%s", action, opening)
			}
		}
		return true
	})
}
