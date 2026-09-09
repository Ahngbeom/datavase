//go:build integration

package ui

import (
	"context"
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

// The grid cuts a value at CellLimit and there was no way to see the rest.
// This is the whole point of the row view, driven through the real interface.
func TestInspectingARowShowsTheValueTheGridCutShort(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	long := strings.Repeat("abcdefghij", 30)
	h.typeSQL("SELECT '" + long + "' AS bio")
	h.do(keymap.ActionRun)
	h.waitFor("the row to arrive", func(a *App) bool { return a.buf.RowCount() == 1 })

	h.focusGrid()
	h.do(keymap.ActionInspect)

	if !h.waitForScreen("bio") {
		t.Fatalf("the row view did not open:\n%s", h.text())
	}
	// The grid shows 200 runes of it; the view has to show more.
	if !h.waitForScreen(long[250:280]) {
		t.Errorf("the row view stops where the grid did:\n%s", h.text())
	}
}

// Inspect only ever reads the grid now, so pressing it with focus anywhere
// else must say so rather than doing nothing.
func TestInspectWithFocusOffTheGridSaysSo(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionInspect)

	if !h.waitForScreen("select a result row first") {
		t.Errorf("inspecting off the grid gave no feedback:\n%s", h.text())
	}
}

// With the grid focused the copy key used to fall through to "nothing
// selected and nothing running". Lifting one value out of a result is an
// everyday action and there was no way to do it.
func TestCopyingFromTheGridTakesTheSelectedValue(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 'ada@example.com' AS email")
	h.do(keymap.ActionRun)
	h.waitFor("the row to arrive", func(a *App) bool { return a.buf.RowCount() == 1 })

	h.focusGrid()
	h.do(keymap.ActionCopyOrCancel)

	h.waitFor("the value to reach the clipboard", func(a *App) bool {
		return a.readClipboard() == "ada@example.com"
	})
}

// The types have been kept in the buffer since the first release and nothing
// ever showed them.
func TestTheRowViewNamesTheColumnsType(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 'ada' AS email")
	h.do(keymap.ActionRun)
	h.waitFor("the row to arrive", func(a *App) bool { return a.buf.RowCount() == 1 })

	h.focusGrid()
	h.do(keymap.ActionInspect)

	// What the driver reports, not what SQL was written: MariaDB answers
	// VARCHAR here, and the point is that the column's own type reaches the
	// screen at all.
	if !h.waitForScreen("VARCHAR") {
		t.Errorf("the row view does not name the column's type:\n%s", h.text())
	}
}
