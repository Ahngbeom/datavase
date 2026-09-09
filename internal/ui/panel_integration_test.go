//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// focusSchemaPane puts focus on the schema pane without depending on how
// many Tab presses that happens to take.
func (h *harness) focusSchemaPane() {
	h.t.Helper()

	// A pane that is not on screen cannot hold focus, and the schema pane
	// starts hidden.
	h.showSidebar()
	h.app.app.QueueUpdateDraw(func() { h.app.app.SetFocus(h.app.schemaPrimitive()) })
	h.settle()
}

func TestSchemaPaneHasTabs(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.showSidebar()

	got := h.text()
	for _, want := range []string{tabTree, tabTables} {
		if !strings.Contains(got, want) {
			t.Errorf("the schema pane does not offer the %q tab:\n%s", want, got)
		}
	}
}

func TestResultPaneHasTabs(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	got := h.text()
	if !strings.Contains(got, tabResults) {
		t.Errorf("the result pane does not offer the %q tab:\n%s", tabResults, got)
	}
}

// The tables tab lists what the cache holds, so it fills without a server
// round trip.
func TestTablesTabListsTheCurrentSchema(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.focusSchemaPane()
	h.do(keymap.ActionCycleTab)

	got := h.text()
	if !strings.Contains(got, "customers") {
		t.Errorf("the tables tab does not list the cached tables:\n%s", got)
	}
	if !strings.Contains(got, "filter") {
		t.Errorf("the tables tab has no filter field:\n%s", got)
	}
}

func TestTablesTabFilters(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.focusSchemaPane()
	h.do(keymap.ActionCycleTab)
	h.typeInto("invoice")

	got := h.text()
	if !strings.Contains(got, "invoices") {
		t.Errorf("the filter hid the matching table:\n%s", got)
	}
	if strings.Contains(got, "customer_notes") {
		t.Errorf("the filter kept a non-matching table:\n%s", got)
	}
}

// buildLayout puts schemaTabs and rightPane in one horizontal Flex, so with
// the sidebar open their headers can land on the same screen row. The
// hitmap used to be keyed by row alone and replace whatever a row already
// held, so whichever region recorded second — the one holding focus, since
// tview.Flex.Draw defers a focused item's Draw — silently erased the
// other's zones from the hitmap.
func TestBothHeadersSharingARowPublishTheirOwnZones(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.showSidebar()
	h.settle()

	var schemaRow, editorRow int
	var zones []zone
	h.inspect(func(a *App) bool {
		schemaRow = a.schemaTabs.headerRow
		editorRow = a.editorRegion.headerRow
		zones = a.hits.rows[editorRow]
		return true
	})

	if schemaRow != editorRow {
		t.Fatalf("schemaTabs and editorRegion drew their headers on rows %d and %d; this test needs them shared", schemaRow, editorRow)
	}

	var haveTab, haveRegionName bool
	for _, z := range zones {
		if z.target == zoneTab {
			haveTab = true
		}
		if z.target == zoneRegionName {
			haveRegionName = true
		}
	}
	if !haveTab {
		t.Errorf("row %d holds no zoneTab zone; the sidebar's tab strip is gone: %+v", editorRow, zones)
	}
	if !haveRegionName {
		t.Errorf("row %d holds no zoneRegionName zone; the editor header's zone is gone: %+v", editorRow, zones)
	}
}
