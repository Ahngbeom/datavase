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
	//
	// Both platforms are rendered rather than whichever this machine is: the
	// modifiers become Apple glyphs on macOS and words elsewhere, so a test
	// that read the label whole would pass here and fail on CI. The letter is
	// the part that actually distinguishes the two keyboards — VS Code
	// deletes a line with K, DataGrip with Y.
	for _, mac := range []bool{true, false} {
		t.Run(map[bool]string{true: "mac", false: "other"}[mac], func(t *testing.T) {
			restore := onMac
			onMac = mac
			t.Cleanup(func() { onMac = restore })

			var help string
			h.inspect(func(a *App) bool {
				help = a.helpText()
				return true
			})

			line := helpLine(t, help, "delete the current line")
			if !strings.Contains(line, "K") {
				t.Errorf("line-delete is %q, want the VS Code K binding", line)
			}
		})
	}
}

// helpLine returns the row of the key reference documenting an action.
func helpLine(t *testing.T, help, description string) string {
	t.Helper()

	for _, line := range strings.Split(help, "\n") {
		if strings.Contains(line, description) {
			return line
		}
	}
	t.Fatalf("no line documents %q:\n%s", description, help)
	return ""
}
