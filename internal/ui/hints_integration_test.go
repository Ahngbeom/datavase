//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// A region met for the first time says what is in it, without dropping the
// still-unread greeting that was already there. Met again — even only in
// passing, with nothing else happening in between — its hint is gone from
// the underlying field, not merely hidden by whatever is drawn over it: by
// then the bar has a row count to carry and the user has the menu.
func TestARegionOffersItsCommandsTheFirstTimeOnly(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	// editor -> grid: ctxResult's first visit. Nothing has happened yet, so
	// the greeting's "for keys" clause and the grid's own hint show together
	// — refreshHints appends rather than replaces on a still-unread bar.
	h.do(keymap.ActionNextPane)
	h.waitFor("the newly focused region to join its commands to the unread greeting", func(a *App) bool {
		line, _ := a.status.renderWidth(200)
		return strings.Contains(line, a.keyLabel(keymap.ActionSortColumn)) && strings.Contains(line, "for keys")
	})

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("the result to displace the hints", func(a *App) bool {
		line, _ := a.status.renderWidth(200)
		return !strings.Contains(line, a.keyLabel(keymap.ActionSortColumn))
	})

	// typeSQL left focus on the editor. Moving back to the grid revisits
	// ctxResult with nothing new to say about it — the phase is still
	// phaseDone from the query above, so the render gate already hides every
	// hint regardless; this checks the field itself, which refreshHints has
	// to trim back rather than leave sitting there for the next idle draw.
	h.do(keymap.ActionPrevPane)
	h.settle()

	if h.inspect(func(a *App) bool {
		return strings.Contains(strings.Join(a.status.hints, " "), a.keyLabel(keymap.ActionSortColumn))
	}) {
		t.Error("a region already visited still carries its hint in the underlying field")
	}
}

// Three regions visited in sequence, with nothing in between to clear the
// bar, must not pile up: only the last one's clauses belong there. The
// sidebar has to be shown first — with it hidden, Tab only ever toggles
// between the editor and the grid, and that pair alone can never reach a
// third region without an intervening notice to reset from, so the bug this
// pins down would be unreachable without it.
func TestARegionsHintReplacesThePreviousRegionsRatherThanJoiningIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.showSidebar()

	h.do(keymap.ActionNextPane) // editor -> grid: ctxResult's first visit
	h.waitFor("the grid to offer its commands", func(a *App) bool {
		line, _ := a.status.renderWidth(200)
		return strings.Contains(line, a.keyLabel(keymap.ActionSortColumn))
	})

	h.do(keymap.ActionNextPane) // grid -> tree: ctxTree's first visit
	h.waitFor("the tree to offer its commands", func(a *App) bool {
		line, _ := a.status.renderWidth(200)
		return strings.Contains(line, a.keyLabel(keymap.ActionRefreshSchema))
	})

	h.do(keymap.ActionNextPane) // tree -> editor: ctxEditor's first visit
	h.settle()

	if !h.inspect(func(a *App) bool {
		line, _ := a.status.renderWidth(200)
		// The still-unread greeting survives every move — nothing has
		// happened yet — but only ever one region's clauses ride beside it.
		return strings.Contains(line, "for keys") &&
			strings.Contains(line, a.keyLabel(keymap.ActionSearchHistory)) &&
			!strings.Contains(line, a.keyLabel(keymap.ActionSortColumn)) &&
			!strings.Contains(line, a.keyLabel(keymap.ActionRefreshSchema))
	}) {
		t.Error("the bar still carries a region's hint after the keyboard left it")
	}
}
