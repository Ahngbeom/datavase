//go:build integration

package ui

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/history"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// Running a statement has to land in the history, or the search has nothing
// to find.
func TestRunningAStatementIsRecorded(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 4242 AS marker")

	h.do(keymap.ActionRun)
	if !h.waitForScreen("1 row") {
		t.Fatalf("the statement never finished:\n%s", h.text())
	}

	// Recording happens on a background goroutine.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := h.history.Search(context.Background(), "4242", 10)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(entries) > 0 {
			if entries[0].Rows != 1 {
				t.Errorf("recorded Rows = %d, want 1", entries[0].Rows)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("the statement was never recorded in the history")
}

func TestHistorySearchOpensAndFilters(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	name := h.app.conn.DataSource().Name
	for _, sql := range []string{"SELECT 111 AS alpha", "SELECT 222 AS beta"} {
		if err := h.history.Add(context.Background(), historyEntry(name, sql)); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	h.do(keymap.ActionSearchHistory)

	got := h.text()
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatalf("history did not list the recorded statements:\n%s", got)
	}

	// Typing filters the list.
	h.typeInto("alpha")
	got = h.text()
	if strings.Contains(got, "beta") {
		t.Errorf("the search term did not filter the list:\n%s", got)
	}
}

// Choosing an entry puts it back in the editor, which is the whole point.
func TestChoosingAHistoryEntryFillsTheEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	name := h.app.conn.DataSource().Name
	if err := h.history.Add(context.Background(), historyEntry(name, "SELECT 777 AS gamma")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	h.do(keymap.ActionSearchHistory)
	h.press(tcell.KeyDown)  // move focus into the list
	h.press(tcell.KeyEnter) // accept

	if got := h.editorText(); !strings.Contains(got, "777") {
		t.Errorf("editor holds %q, want the chosen statement", got)
	}
}

func TestCommandPaletteOpens(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)

	got := h.text()
	for _, want := range []string{"export csv", "history"} {
		if !strings.Contains(got, want) {
			t.Errorf("the palette does not offer %q:\n%s", want, got)
		}
	}
}

// The list is longer than a modest terminal — which is why the filter exists,
// and why checking that a command near the end is on screen unfiltered tests
// the length of the list rather than the palette. Filtering is the route a
// user actually takes to those commands.
func TestThePaletteFilterReachesACommandBelowTheFold(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCommandPalette)
	if strings.Contains(h.text(), "leave datavase") {
		t.Skip("quit is on screen unfiltered; the filter is not what is under test here")
	}

	h.typeInto("quit")

	if !strings.Contains(h.text(), "leave datavase") {
		t.Errorf("filtering for \"quit\" does not reach it:\n%s", h.text())
	}
}

// The write lock is the guard's escape hatch; unlocking must be visible.
func TestUnlockingWritesIsAnnouncedAndVisible(t *testing.T) {
	h := newHarness(t, config.EnvProd)

	h.app.app.QueueUpdateDraw(func() { h.app.enableWrites() })
	h.settle()

	if !strings.Contains(strings.ToLower(h.text()), "writes on") {
		t.Errorf("the status bar does not show that writes are unlocked:\n%s", h.text())
	}

	// And an unlocked production write is now a confirmation, not a refusal.
	h.typeSQL("UPDATE dv_seq SET n = n WHERE n = 1")
	h.do(keymap.ActionRun)

	got := h.text()
	if strings.Contains(got, "Refused") {
		t.Errorf("the write was still refused after unlocking:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "run it?") {
		t.Errorf("no confirmation appeared:\n%s", got)
	}
}

func TestExportWritesACSVFile(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { os.Chdir(previous) })

	h.typeSQL("SELECT 1 AS id, 'alice' AS name")
	h.do(keymap.ActionRun)
	// Wait on the status bar, not on a value: "alice" is also sitting in the
	// editor, so matching it would not mean the result had arrived.
	if !h.waitForScreen("1 row") {
		t.Fatalf("the statement never finished:\n%s", h.text())
	}

	h.app.app.QueueUpdateDraw(func() { h.app.exportResult(formatCSV) })
	h.settle()

	matches, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one CSV file, got %v (err %v)\nscreen:\n%s", matches, err, h.text())
	}

	file, err := os.Open(matches[0])
	if err != nil {
		t.Fatalf("opening the export: %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("the export is not valid CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("export has %d records, want a header and one row", len(records))
	}
	if records[0][1] != "name" || records[1][1] != "alice" {
		t.Errorf("export = %v, want the queried values", records)
	}
}

func TestExportWithNoResultSaysSo(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.app.app.QueueUpdateDraw(func() { h.app.exportResult(formatCSV) })
	h.settle()

	if !strings.Contains(h.text(), "no result to export") {
		t.Errorf("exporting without a result gave no feedback:\n%s", h.text())
	}
}

// Go-to-table searches the cache and drops a starter query in the editor.
func TestGoToTableFillsTheEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.do(keymap.ActionGoToTable)
	if !strings.Contains(h.text(), "customers") {
		t.Fatalf("go-to-table did not list the cached tables:\n%s", h.text())
	}

	h.typeInto("invoices")
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	got := h.editorText()
	if !strings.Contains(got, "invoices") {
		t.Errorf("editor holds %q, want a query against the chosen table", got)
	}
	if !strings.HasPrefix(got, "SELECT") {
		t.Errorf("editor holds %q, want a SELECT starter", got)
	}
}

// The starter query is placed, not executed: the guard and the user still
// decide when anything runs.
func TestGoToTableDoesNotRunTheQuery(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.do(keymap.ActionGoToTable)
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	if strings.Contains(h.text(), "rows ·") {
		t.Errorf("go-to-table ran the query instead of placing it:\n%s", h.text())
	}
}

// typeInto sends characters to whatever currently has focus.
func (h *harness) typeInto(text string) {
	h.t.Helper()

	keys := make([]*tcell.EventKey, 0, len(text))
	for _, r := range text {
		keys = append(keys, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	h.inject(keys...)
}

// historyEntry builds a recorded statement for the tests.
func historyEntry(datasource, sql string) history.Entry {
	return history.Entry{
		DataSource: datasource,
		SQL:        sql,
		Rows:       1,
		Elapsed:    time.Millisecond,
		At:         time.Now(),
	}
}
