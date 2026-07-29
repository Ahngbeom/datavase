//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// Without this the first keystroke of a modal editor does nothing visible,
// and the application reads as broken rather than as modal.
func TestStatusBarShowsTheModeAndPendingSequence(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("SELECT id", 0)

	if !strings.Contains(h.text(), "NORMAL") {
		t.Errorf("the status bar does not show the mode:\n%s", h.text())
	}

	h.typeInto("i")
	h.waitFor("the insert indicator", func(a *App) bool {
		return strings.Contains(a.currentStatus().vimMode, "INSERT")
	})
	if !strings.Contains(h.text(), "INSERT") {
		t.Errorf("the status bar does not show insert mode:\n%s", h.text())
	}

	// A half-typed operator has to be visible too.
	h.press(tcell.KeyEscape)
	h.typeInto("d")
	if got := h.text(); !strings.Contains(got, "NORMAL d") {
		t.Errorf("the status bar does not show the pending sequence:\n%s", got)
	}
}

// The other keyboards have no mode, and a status bar claiming one would be
// noise at best.
func TestStatusBarHasNoModeOnTheDataGripKeyboard(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if strings.Contains(h.text(), "NORMAL") {
		t.Errorf("the status bar shows a mode on a non-modal keyboard:\n%s", h.text())
	}
}

// tview's list is the one panel that does not answer to j and k itself.
func TestTablesListMovesWithJAndK(t *testing.T) {
	h := newVimHarness(t)
	h.seedCache(completionSnapshot())

	h.focusSchemaPane()
	h.do(keymap.ActionCycleTab)
	h.waitFor("the tables list", func(a *App) bool {
		return a.tableList.GetItemCount() > 1
	})

	h.inspect(func(a *App) bool {
		a.app.SetFocus(a.tableList)
		a.tableList.SetCurrentItem(0)
		return true
	})

	h.typeInto("j")
	h.waitFor("the second table", func(a *App) bool {
		return a.tableList.GetCurrentItem() == 1
	})

	h.typeInto("k")
	h.waitFor("the first table", func(a *App) bool {
		return a.tableList.GetCurrentItem() == 0
	})

	h.typeInto("G")
	h.waitFor("the last table", func(a *App) bool {
		return a.tableList.GetCurrentItem() == a.tableList.GetItemCount()-1
	})

	h.typeInto("gg")
	h.waitFor("the first table again", func(a *App) bool {
		return a.tableList.GetCurrentItem() == 0
	})
}

// Someone who did not choose a modal editor has to be able to find the way
// out from inside it.
func TestHelpShowsTheVimKeysAndTheWayOut(t *testing.T) {
	h := newVimHarness(t)

	var help string
	h.inspect(func(a *App) bool {
		help = a.helpText()
		return true
	})

	for _, want := range []string{"dd", "insert before the cursor", "preset: datagrip"} {
		if !strings.Contains(help, want) {
			t.Errorf("the key reference is missing %q:\n%s", want, help)
		}
	}
}

func TestHelpOmitsTheVimKeysOnTheDataGripKeyboard(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	var help string
	h.inspect(func(a *App) bool {
		help = a.helpText()
		return true
	})

	if strings.Contains(help, "insert before the cursor") {
		t.Errorf("the vim reference is shown on a non-modal keyboard:\n%s", help)
	}
}

// An empty editor is where a modal keyboard is most confusing: typing does
// nothing at all until insert mode is entered.
func TestPlaceholderSaysHowToStartTyping(t *testing.T) {
	h := newVimHarness(t)

	var placeholder string
	h.inspect(func(a *App) bool {
		placeholder = a.editorPlaceholder()
		return true
	})

	if !strings.Contains(placeholder, "i to type") {
		t.Errorf("placeholder = %q, want it to say how to start typing", placeholder)
	}
}
