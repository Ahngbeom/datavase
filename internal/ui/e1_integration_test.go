//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// Switching keyboards has to be reachable without editing a file and
// restarting: the moment someone discovers the keys are wrong is the moment
// they are already inside the application.
func TestPaletteOffersEveryPreset(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)
	h.typeInto("keymap")

	got := h.text()
	for _, p := range keymap.Presets() {
		if !strings.Contains(got, "keymap "+string(p)) {
			t.Errorf("the palette does not offer %q:\n%s", "keymap "+string(p), got)
		}
	}
}

// Choosing from the palette must actually run the command.
func TestPaletteSwitchesThePreset(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)
	h.typeInto("keymap vscode")
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	h.waitFor("the vscode keyboard", func(a *App) bool {
		return a.keys.Preset() == keymap.PresetVSCode
	})
}

func TestSwitchingPresetTakesEffectImmediately(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.app.app.QueueUpdateDraw(func() { h.app.setPreset(keymap.PresetVSCode) })
	h.settle()

	h.waitFor("the vscode keyboard", func(a *App) bool {
		return a.keys.Preset() == keymap.PresetVSCode
	})
	// The switch is worthless if the user is not told it happened.
	if !strings.Contains(h.text(), "vscode") {
		t.Errorf("the status bar does not report the new keyboard:\n%s", h.text())
	}

	// And the key reference must show the new keyboard, not the old one.
	if help := h.app.helpText(); !strings.Contains(help, "⇧K") {
		t.Errorf("the key reference still shows the old keyboard:\n%s", help)
	}
}
