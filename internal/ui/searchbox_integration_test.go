//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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

		// Weight, not colour. A category is not a state anyone could forget
		// they are in, and drawing it in the notice colour would leave no
		// colour that meant a state like that and nothing else. Bold also
		// survives a monochrome terminal.
		_, _, attrs := cells[row*width+col].Style.Decompose()
		if attrs&tcell.AttrBold == 0 {
			t.Errorf("the %q heading is drawn with no weight of its own", paletteCategories[0])
		}
		if fg, _, _ := cells[row*width+col].Style.Decompose(); fg == colourNotice {
			t.Errorf("the %q heading spends the notice colour", paletteCategories[0])
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
