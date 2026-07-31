//go:build integration

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// nestedDirs builds a directory two levels deep, which is the case that cannot
// be reached without completing a segment: typing "<root>/mi/2" matches
// nothing, because "<root>/mi" is not a directory to read children from.
func nestedDirs(t *testing.T) (root, deep string) {
	t.Helper()

	root = t.TempDir()
	deep = filepath.Join(root, "migrations", "2024")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	resolved, err := filepath.EvalSymlinks(deep)
	if err != nil {
		t.Fatal(err)
	}
	return root, resolved
}

// openAttach opens the attach dialog and waits for it.
func (h *harness) openAttach() {
	h.t.Helper()

	h.do(keymap.ActionFindFile)
	h.waitFor("the attach dialog", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageAttach
	})
}

// Descending used to mean typing the whole path: choosing a row attached it
// rather than moving into it, so every directory below the first had to be
// spelled out in full.
func TestTabCompletesADirectoryPathSoTheNextSegmentCanBeTyped(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root, deep := nestedDirs(t)

	h.openAttach()
	h.typeInto(filepath.Join(root, "mi"))

	// Without this the rest of the path has nothing to hang off: "…/mi" is not
	// a directory, so "…/mi/2" lists nothing to choose.
	h.press(tcell.KeyTab)

	h.typeInto(string(filepath.Separator) + "2")
	h.press(tcell.KeyEnter)

	h.waitFor("the nested directory to be attached", func(a *App) bool {
		return a.wt != nil && a.wt.Root == deep
	})
}

// Completing from the list has to hand typing back, or the path stops halfway
// with the caret somewhere that cannot extend it.
func TestCompletingFromTheListReturnsToTyping(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root, deep := nestedDirs(t)

	h.openAttach()
	h.typeInto(filepath.Join(root, "mi"))

	h.press(tcell.KeyDown) // into the list
	h.waitFor("focus to reach the list", func(a *App) bool {
		_, isList := a.app.GetFocus().(*tview.List)
		return isList
	})

	h.press(tcell.KeyTab)
	h.waitFor("focus to return to the field", func(a *App) bool {
		_, isField := a.app.GetFocus().(*tview.InputField)
		return isField
	})

	h.typeInto(string(filepath.Separator) + "2")
	h.press(tcell.KeyEnter)

	h.waitFor("the nested directory to be attached", func(a *App) bool {
		return a.wt != nil && a.wt.Root == deep
	})
}

// A directory attached once is nearly always the one wanted again, and the
// dialog used to open on the working directory with no memory of any of them.
func TestADirectoryAttachedBeforeIsOfferedWithNothingTyped(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root := newWorktree(t)

	h.attachWorktree(root)
	h.app.app.QueueUpdateDraw(func() { h.app.detach() })
	h.settle()

	h.openAttach()

	if !h.waitForScreen("recent ·") {
		t.Errorf("the dialog does not offer the directory just attached; screen:\n%s", h.text())
	}
	// The row says what it is, so a checkout and a plain folder are told apart
	// before either is attached.
	if !h.waitForScreen("git repository") {
		t.Errorf("the recent row does not say it is a repository; screen:\n%s", h.text())
	}
}

// Every other dialog has nothing to complete, and Tab has always moved into
// the results there. Taking that away to make room for completion would be a
// trade nobody asked for.
func TestTabStillMovesIntoTheResultsWhereThereIsNothingToComplete(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)
	h.waitFor("the palette", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pagePalette
	})

	h.press(tcell.KeyTab)
	h.waitFor("focus to reach the list", func(a *App) bool {
		_, isList := a.app.GetFocus().(*tview.List)
		return isList
	})
}
