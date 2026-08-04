//go:build integration

package ui

import (
	"os"
	"path/filepath"
	"strings"
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

// The palette now opens with a heading as its first row, and Enter used to
// take row zero outright. On the one dialog whose whole purpose is to run
// something, that made Enter a key that does nothing.
func TestEnterRunsTheFirstCommandPastTheHeading(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)
	h.waitFor("the palette", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pagePalette
	})

	// Nothing is running, so the first command — cancel — says so. That notice
	// is proof Enter reached a command rather than falling on the heading.
	h.press(tcell.KeyEnter)
	h.waitFor("the first command to run", func(a *App) bool {
		return a.status.message == "nothing is running"
	})

	h.waitFor("the palette to close", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name != pagePalette
	})
}

// A heading drawn in the same colour as everything else reads as a command
// with no description, which is what one looked like before it was styled.
//
// It also pins the mechanism. The colour is a tview tag in the row's text, and
// escaping that text — which every other row is deliberately put through —
// leaves the tag's brackets behind as "[Running[" while the colour still
// applies. Checking the colour alone would not notice.
func TestThePaletteHeadingsAreDrawnAsHeadings(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)
	h.waitFor("the palette", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pagePalette
	})

	// Cell by cell rather than through h.text(): the column a heading starts at
	// is what the style has to be read from, and a rendered line is full of
	// multi-byte box drawing, so a byte offset into it is not a column.
	cells, width, height := h.screen.GetContents()
	column := func(row int, want string) int {
		for col := 0; col < width; col++ {
			var run strings.Builder
			for i := col; i < width && run.Len() < len(want); i++ {
				run.Write(cells[row*width+i].Bytes)
			}
			if run.String() == want {
				return col
			}
		}
		return -1
	}

	found := false
	for row := 0; row < height; row++ {
		col := column(row, paletteCategories[0])
		if col < 0 {
			continue
		}
		found = true

		fg, _, _ := cells[row*width+col].Style.Decompose()
		if fg != colourNotice {
			t.Errorf("the %q heading is drawn in %v, want %v", paletteCategories[0], fg, colourNotice)
		}

		// Nothing but the name, once the dialog's own border is taken off.
		var line strings.Builder
		for c := 0; c < width; c++ {
			line.Write(cells[row*width+c].Bytes)
		}
		bare := strings.TrimSpace(strings.Trim(strings.TrimSpace(line.String()), "║"))
		if bare != paletteCategories[0] {
			t.Errorf("the heading row reads %q, want %q", bare, paletteCategories[0])
		}
	}
	if !found {
		t.Fatalf("the palette does not show the %q heading:\n%s", paletteCategories[0], h.text())
	}
}

// The walk itself is checked against the real rows in searchbox_test.go. What
// needs a running interface is where the highlight starts: tview draws it
// whether or not the list has focus, and clearing the list puts it back on row
// zero — which in a grouped list is a heading.
func TestThePaletteOpensWithTheHighlightOnACommand(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)
	h.waitFor("the palette", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pagePalette
	})

	h.press(tcell.KeyDown) // into the list
	h.waitFor("the highlight to be on a command", func(a *App) bool {
		list, isList := a.app.GetFocus().(*tview.List)
		if !isList {
			return false
		}
		primary, _ := list.GetItemText(list.GetCurrentItem())
		// A heading is the group's name and nothing else; a command row carries
		// its summary in the same line, padded away from the name.
		for _, category := range paletteCategories {
			if strings.TrimSpace(primary) == category {
				return false
			}
		}
		return strings.TrimSpace(primary) != ""
	})
}
