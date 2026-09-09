//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// runLine types a whole ":" command and presses Enter.
func (h *harness) runLine(line string) {
	h.t.Helper()

	h.typeInto(":")
	h.waitFor("the command line", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageCommand
	})

	h.typeInto(line)
	h.press(tcell.KeyEnter)
}

// The colon has to reach the prompt from normal mode at all — a key that a
// half-typed operator swallows looks like a key that stopped working.
func TestColonOpensACommandLine(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("SELECT 1", 0)

	h.typeInto(":")

	h.waitFor("the command line", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageCommand
	})

	h.press(tcell.KeyEscape)
	h.waitFor("the editor to have focus again", func(a *App) bool {
		return a.app.GetFocus() == a.editor
	})
	// Escaping a prompt must leave the buffer exactly as it was, not type the
	// command into it.
	h.wantEditor("SELECT 1")
}

// A command line that runs the nearest thing it can think of is worse than one
// that refuses: "c" stands in front of cancel and commit alike, and those are
// opposite outcomes.
func TestAnAmbiguousCommandIsRefusedAndSaysWhy(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("", 0)

	h.runLine("c")

	if !h.waitForScreen("could be") {
		t.Fatalf("the ambiguous command was not explained; screen:\n%s", h.text())
	}
	screen := h.text()
	for _, want := range []string{"cancel", "commit"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the refusal does not offer %q; screen:\n%s", want, screen)
		}
	}
}

func TestAnUnknownCommandSaysWhereTheCommandsAre(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("", 0)

	h.runLine("zzz")

	if !h.waitForScreen("no command") {
		t.Fatalf("an unknown command said nothing; screen:\n%s", h.text())
	}
}

// The line resolves palette commands, which is the whole reason it is not a
// second list of its own.
func TestACommandLineRunsAPaletteCommand(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("", 0)

	h.runLine("history")

	h.waitFor("the history dialog", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageHistory
	})
}
