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

	for _, want := range []string{"dd", "insert before the cursor", "For an ordinary editor"} {
		if !strings.Contains(help, want) {
			t.Errorf("the key reference is missing %q:\n%s", want, help)
		}
	}
}

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

// The way out has to be readable without scrolling through the thing being
// escaped.
//
// It used to sit at the foot of the vim reference, past forty keys and a full
// modal table — so the answer to "I cannot type into this" was behind the
// whole of what the reader was trying to leave.
func TestTheWayOutOfTheModalEditorComesBeforeTheVimKeys(t *testing.T) {
	h := newVimHarness(t)

	var help string
	h.inspect(func(a *App) bool {
		help = a.helpText()
		return true
	})

	// The hatch's own words. "keymap datagrip" is no anchor: commandHelpText
	// lists every palette command, that one included, so a test looking for it
	// passes with no escape hatch anywhere.
	escape := strings.Index(help, "For an ordinary editor")
	reference := strings.Index(help, "insert before the cursor")
	if escape < 0 || reference < 0 {
		t.Fatalf("the key reference is missing the escape hatch or the vim keys:\n%s", help)
	}
	if escape > reference {
		t.Error("the way out of the modal editor is printed after the vim reference")
	}
}

// The hint is only true on a modal keyboard. Telling a DataGrip user to press
// i first would be advice that types an i.
func TestNoModalHintOnAnOrdinaryKeyboard(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	var help string
	h.inspect(func(a *App) bool {
		help = a.helpText()
		return true
	})

	if strings.Contains(help, "press i first") {
		t.Errorf("the modal hint is shown on a non-modal keyboard:\n%s", help)
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
