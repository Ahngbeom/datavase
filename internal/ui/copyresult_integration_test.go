//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

func TestTheCopyKeyOffersAFormatAndCopiesTheWholeResult(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 1 AS a, 'x|y' AS b UNION ALL SELECT 2, 'z'")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 2 })

	h.do(keymap.ActionCopyResult)
	h.waitFor("the format chooser", func(a *App) bool { return a.pages.HasPage(pageCopyFormat) })
	h.press(tcell.KeyEnter) // Markdown is first

	h.waitFor("the clipboard", func(a *App) bool { return strings.HasPrefix(a.clipboard, "| a | b |") })
	if !h.inspect(func(a *App) bool { return strings.Contains(a.clipboard, `x\|y`) }) {
		t.Error("the pipe inside a value was not escaped")
	}
	h.waitFor("the notice", func(a *App) bool { return strings.Contains(a.status.message, "2 rows copied as Markdown") })
}

func TestClickingCopyOnTheResultHeaderOpensTheChooser(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.status.phase == phaseDone })

	h.clickZone(zoneCopyResult, -1)
	h.waitFor("the format chooser", func(a *App) bool { return a.pages.HasPage(pageCopyFormat) })
}

func TestCopyingWithNoResultSaysSo(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.do(keymap.ActionCopyResult)
	h.waitFor("the notice", func(a *App) bool { return a.status.message == "no result to copy" })
}

// The header hint before anything has run ("run a statement to see rows
// here") is not the copy label, and a stray zoneCopyResult over it would
// resolve a click there to "copy the result" instead of whatever the hint
// actually says.
func TestNoResultPublishesNoCopyZone(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.settle()

	found := h.inspect(func(a *App) bool {
		for _, zones := range a.hits.rows {
			for _, z := range zones {
				if z.target == zoneCopyResult {
					return true
				}
			}
		}
		return false
	})
	if found {
		t.Error("a zoneCopyResult zone exists with no result to copy")
	}
}
