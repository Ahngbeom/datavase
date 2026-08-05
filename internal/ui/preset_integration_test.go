//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// The empty editor is exactly where a modal keyboard is most confusing:
// typing does nothing until insert mode is entered, and the placeholder is
// the only thing on screen that says so.
//
// It was composed once, when the widgets were built, so switching to the vim
// keyboard from the command palette — which is an advertised way to get there
// — left the editor promising that typing would work.
func TestSwitchingToTheModalKeyboardSaysHowToStartTyping(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if screen := h.text(); strings.Contains(screen, "press i to type") {
		t.Fatalf("the ordinary keyboard advertised insert mode:\n%s", screen)
	}

	h.inspect(func(a *App) bool { a.setPreset(keymap.PresetVim); return true })
	h.waitFor("the vim keyboard", func(a *App) bool { return a.keys.Modal() })

	if !h.waitForScreen("press i to type") {
		t.Errorf("the modal editor never said how to start typing:\n%s", h.text())
	}
}

// And back: an ordinary editor that tells people to press i first sends them
// to type a letter into their own statement.
func TestSwitchingAwayFromTheModalKeyboardTakesTheHintBack(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.inspect(func(a *App) bool { a.setPreset(keymap.PresetVim); return true })
	if !h.waitForScreen("press i to type") {
		t.Fatalf("the modal editor never said how to start typing:\n%s", h.text())
	}

	h.inspect(func(a *App) bool { a.setPreset(keymap.PresetDataGrip); return true })
	h.waitFor("the ordinary keyboard", func(a *App) bool { return !a.keys.Modal() })

	deadline := 0
	for deadline < 50 && strings.Contains(h.text(), "press i to type") {
		h.settle()
		deadline++
	}
	if strings.Contains(h.text(), "press i to type") {
		t.Errorf("the ordinary editor still advertised insert mode:\n%s", h.text())
	}
}

// A keyboard that could not be built changes nothing, so the hint must not
// change either — a placeholder describing a keyboard the session does not
// have is worse than a stale one.
func TestARefusedPresetLeavesTheHintAlone(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.inspect(func(a *App) bool { a.setPreset(keymap.Preset("nonesuch")); return true })
	h.settle()

	if screen := h.text(); strings.Contains(screen, "press i to type") {
		t.Errorf("a refused preset changed the editor hint:\n%s", screen)
	}
}
