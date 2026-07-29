//go:build integration

package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

// focusSchemaPane puts focus on the schema pane without depending on how
// many Tab presses that happens to take.
func (h *harness) focusSchemaPane() {
	h.t.Helper()
	h.app.app.QueueUpdateDraw(func() { h.app.app.SetFocus(h.app.schemaPrimitive()) })
	h.settle()
}

func TestSchemaPaneHasTabs(t *testing.T) {
	h := newHarness(t, config.EnvDev)

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
	for _, want := range []string{tabResults, tabDDL} {
		if !strings.Contains(got, want) {
			t.Errorf("the result pane does not offer the %q tab:\n%s", want, got)
		}
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

// Choosing a table writes a starter query, the same as go-to-table does.
func TestTablesTabOpensATable(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.focusSchemaPane()
	h.do(keymap.ActionCycleTab)
	h.typeInto("invoices")
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	if got := h.editorText(); !strings.Contains(got, "invoices") {
		t.Errorf("editor holds %q, want a query against the chosen table", got)
	}
}

// Cycling affects the focused pane only; otherwise one key would move two
// panes at once and neither would be predictable.
func TestCycleTabAffectsTheFocusedPaneOnly(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	// Focus the result pane and cycle it.
	h.app.app.QueueUpdateDraw(func() { h.app.app.SetFocus(h.app.resultPrimitive()) })
	h.settle()
	h.do(keymap.ActionCycleTab)

	if got := h.currentTabs(); got.result != tabDDL {
		t.Errorf("the result pane is on %q, want %q", got.result, tabDDL)
	}
	if got := h.currentTabs(); got.schema != tabTree {
		t.Errorf("the schema pane moved to %q; it did not have focus", got.schema)
	}
}

type tabState struct{ schema, result string }

func (h *harness) currentTabs() tabState {
	h.t.Helper()

	var state tabState
	done := make(chan struct{})
	h.app.app.QueueUpdateDraw(func() {
		state = tabState{
			schema: h.app.schemaTabs.current(),
			result: h.app.resultTabs.current(),
		}
		close(done)
	})
	<-done
	return state
}

// Inspecting a table has to switch the result pane to the definition; a DDL
// that loads into a hidden tab is the same as not loading at all.
func TestInspectShowsTheDefinition(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	// No seeding here: the background refresh has already cached the real
	// server's schema, and SHOW CREATE needs a table that genuinely exists.
	seedSequenceTable(t, h)

	h.focusSchemaPane()
	h.app.app.QueueUpdateDraw(func() {
		h.app.selectedSchema = testmysql.DefaultDatabase
	})
	h.settle()
	h.do(keymap.ActionCycleTab)
	h.typeInto("dv_seq")
	h.press(tcell.KeyDown)

	h.do(keymap.ActionInspect)

	if !h.waitForScreen("CREATE TABLE") {
		t.Errorf("the definition never appeared:\n%s", h.text())
	}
	if got := h.currentTabs(); got.result != tabDDL {
		t.Errorf("the result pane is on %q, want it switched to %q", got.result, tabDDL)
	}
}

// Running a query afterwards returns the pane to the grid.
func TestRunningAQueryReturnsToTheResultsTab(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.app.app.QueueUpdateDraw(func() { h.app.resultTabs.show(tabDDL) })
	h.settle()

	h.typeSQL("SELECT 1 AS n")
	h.do(keymap.ActionRun)

	if !h.waitForScreen("1 rows") {
		t.Fatalf("the statement never finished:\n%s", h.text())
	}
	if got := h.currentTabs(); got.result != tabResults {
		t.Errorf("the result pane is on %q, want it back on %q", got.result, tabResults)
	}
}

// Pressing inspect with nothing selected must explain itself.
func TestInspectWithNoTableSelectedSaysSo(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionInspect)

	if !h.waitForScreen("select a table") {
		t.Errorf("inspecting with no selection gave no feedback:\n%s", h.text())
	}
}

// seedSequenceTable makes sure dv_seq exists and is in the cache, so the
// inspect test has a real table to read a definition from.
func seedSequenceTable(t *testing.T, h *harness) {
	t.Helper()

	ctx := context.Background()
	if _, err := h.app.conn.Exec(ctx,
		"CREATE TABLE IF NOT EXISTS dv_seq (n INT PRIMARY KEY)"); err != nil {
		t.Fatalf("creating dv_seq: %v", err)
	}

	snap, err := catalog.FetchSnapshot(ctx, h.app.conn)
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
	if err := h.cache.Save(ctx, h.app.conn.DataSource().Name, snap); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}
