//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// A result wider than the terminal scrolls sideways, and the columns that go
// off the left take the row's identity with them: a screenful beginning at
// "d" is indistinguishable from one whose query never selected an id. Every
// other way this result can be incomplete — a cut stream, an injected LIMIT —
// says so on the bar, and this one did not.
func TestTheBarSaysWhenColumnsHaveScrolledOffTheLeft(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL(wideSelect, 1)

	h.do(keymap.ActionNextPane)
	h.waitFor("the grid", func(a *App) bool { return a.app.GetFocus() == a.grid })

	// Far enough right that the first columns are certainly gone.
	for i := 0; i < 6; i++ {
		h.press(tcell.KeyRight)
	}

	h.waitFor("the scrolled-off columns to be reported", func(a *App) bool {
		return strings.Contains(a.currentStatus().render(), "left of view")
	})
	if !h.waitForScreen("left of view") {
		t.Errorf("the bar never said the grid had scrolled:\n%s", h.text())
	}
}

// Scrolled back to where it started there is nothing to admit, and a notice
// that never goes away is one nobody reads by the second day.
func TestTheColumnNoticeGoesAwayWhenTheGridScrollsBack(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL(wideSelect, 1)

	h.do(keymap.ActionNextPane)
	h.waitFor("the grid", func(a *App) bool { return a.app.GetFocus() == a.grid })

	for i := 0; i < 6; i++ {
		h.press(tcell.KeyRight)
	}
	h.waitFor("the notice", func(a *App) bool {
		return strings.Contains(a.currentStatus().render(), "left of view")
	})

	for i := 0; i < 6; i++ {
		h.press(tcell.KeyLeft)
	}
	h.waitFor("the notice to go", func(a *App) bool {
		return !strings.Contains(a.currentStatus().render(), "left of view")
	})
}

// The offset survives a tab switch, and a bar warning about a grid nobody is
// looking at is a warning that gets learned as noise.
func TestTheColumnNoticeIsSilentOnAnotherTab(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL(wideSelect, 1)

	h.do(keymap.ActionNextPane)
	h.waitFor("the grid", func(a *App) bool { return a.app.GetFocus() == a.grid })
	for i := 0; i < 6; i++ {
		h.press(tcell.KeyRight)
	}
	h.waitFor("the notice", func(a *App) bool {
		return strings.Contains(a.currentStatus().render(), "left of view")
	})

	h.inspect(func(a *App) bool { a.resultTabs.show(tabDDL); return true })
	h.waitFor("the notice to go quiet", func(a *App) bool {
		return !strings.Contains(a.currentStatus().render(), "left of view")
	})
}

// A statement whose columns cannot all fit on a terminal at once. The values
// are what make it wide: eight narrow columns fit side by side on the
// simulated eighty, and a grid that never scrolls tests nothing.
const wideSelect = "SELECT " +
	"REPEAT('a',18) AS a, REPEAT('b',18) AS b, REPEAT('c',18) AS c, REPEAT('d',18) AS d, " +
	"REPEAT('e',18) AS e, REPEAT('f',18) AS f, REPEAT('g',18) AS g, REPEAT('h',18) AS h"

// Saying how many columns have gone is half the answer. The other half is
// keeping the one that says which row this is: a screenful beginning at
// "total_cents" is unreadable however honestly the bar reports it, because
// nothing on it identifies the row.
//
// The first column of a SELECT is the one people put the id in, so it is
// pinned and the rest scroll past it.
func TestTheFirstColumnStaysWhileTheRestScroll(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL(wideSelect, 1)

	h.do(keymap.ActionNextPane)
	h.waitFor("the grid", func(a *App) bool { return a.app.GetFocus() == a.grid })

	for i := 0; i < 6; i++ {
		h.press(tcell.KeyRight)
	}
	h.waitFor("the grid to scroll", func(a *App) bool {
		_, column := a.grid.GetOffset()
		return column > 0
	})

	screen := h.text()
	if !strings.Contains(screen, "a") || !strings.Contains(screen, "aaaaaaaaaaaaaaaaaa") {
		t.Errorf("the first column went off the left with the others:\n%s", screen)
	}
}

// The pinned column is on screen, so it is not one of the columns that have
// gone — and the count must not quietly start including it.
func TestThePinnedColumnIsNotCountedAsGone(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL(wideSelect, 1)

	h.do(keymap.ActionNextPane)
	h.waitFor("the grid", func(a *App) bool { return a.app.GetFocus() == a.grid })

	for i := 0; i < 6; i++ {
		h.press(tcell.KeyRight)
	}
	h.waitFor("the grid to scroll", func(a *App) bool {
		_, column := a.grid.GetOffset()
		return column > 0
	})

	h.inspect(func(a *App) bool {
		_, column := a.grid.GetOffset()
		// Columns 1..column are hidden; column 0 is pinned and on screen.
		if got := a.columnsOffView(); got != column {
			t.Errorf("columnsOffView() = %d with an offset of %d", got, column)
		}
		return true
	})
}
