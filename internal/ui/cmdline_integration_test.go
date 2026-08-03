//go:build integration

package ui

import (
	"path/filepath"
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

// The one command that must never answer to an abbreviation. A vim user types
// ":u" meaning undo, and unlocking writes against production is the last thing
// that should be one keystroke from a reflex.
func TestTheCommandLineWillNotUnlockWritesFromAnAbbreviation(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("", 0)

	for _, line := range []string{"u", "un", "unlock", "unlock write"} {
		h.runLine(line)

		if h.inspect(func(a *App) bool { return a.status.writesEnabled }) {
			t.Fatalf(":%s unlocked writes against production", line)
		}
	}

	// And the whole name still works, so this is a rule about abbreviation
	// rather than the command having been quietly removed.
	h.runLine(cmdEnableWrites)
	h.waitFor("writes to be unlocked by the full name", func(a *App) bool {
		return a.status.writesEnabled
	})
}

// ":w" is the reflex #12 exists for, and it has to be the save the key already
// performs rather than a second route to disk with its own idea of when to ask.
func TestWriteCommandSavesTheOpenFile(t *testing.T) {
	h := newVimHarness(t)
	root := newWorktree(t)
	h.attachWorktree(root)
	h.openMigration(t)

	h.buffer(initialSQL+"-- from the command line\n", 0)
	h.runLine("w")

	h.waitFor("the buffer to match the file", func(a *App) bool { return !a.fileDirty() })

	if got := readFile(t, filepath.Join(root, "migrations", "001_init.sql")); !strings.Contains(got, "from the command line") {
		t.Errorf("the edit never reached the file:\n%s", got)
	}
}

// Quitting a modal editor is as reflexive as saving, and the buffer is the only
// copy of what has been typed into it.
func TestQuitCommandAsksAboutAnUnsavedFile(t *testing.T) {
	h := newVimHarness(t)
	h.attachWorktree(newWorktree(t))
	h.openMigration(t)

	h.buffer(initialSQL+"-- unsaved\n", 0)
	h.runLine("q")

	h.waitFor("the unsaved-changes question", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageConfirm
	})
}

func TestEditCommandOpensAFileByPath(t *testing.T) {
	h := newVimHarness(t)
	h.attachWorktree(newWorktree(t))

	h.runLine("e scratch.sql")

	h.waitFor("scratch.sql to load", func(a *App) bool { return a.openFile.rel == "scratch.sql" })
}

// A path that names nothing is refused rather than resolved to whatever is
// closest: loading the wrong file replaces the buffer, and there is no undo
// across that.
func TestEditCommandRefusesAPathThatIsNotThere(t *testing.T) {
	h := newVimHarness(t)
	h.attachWorktree(newWorktree(t))

	h.runLine("e migrations/nope.sql")

	if !h.waitForScreen("no migrations/nope.sql in") {
		t.Fatalf("the missing path was not reported; screen:\n%s", h.text())
	}
	if h.inspect(func(a *App) bool { return a.openFile.isOpen() }) {
		t.Error("a file was opened for a path that does not exist")
	}
}
