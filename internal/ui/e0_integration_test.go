//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

// Nothing else on screen says which schema an unqualified query will hit.
func TestStatusBarShowsTheCurrentSchema(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if !strings.Contains(h.text(), "@"+testmysql.DefaultDatabase) {
		t.Errorf("the status bar does not show the current schema:\n%s", h.text())
	}
}

// The reported hazard: choosing a table wiped whatever was being written,
// with no way to undo it.
func TestChoosingATableKeepsWorkInProgress(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("SELECT id FROM ")
	h.moveCaret(len("SELECT id FROM "))

	h.do(keymap.ActionGoToTable)
	h.typeInto("invoices")
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	got := h.editorText()
	if !strings.Contains(got, "SELECT id FROM") {
		t.Fatalf("editor holds %q; the work in progress was destroyed", got)
	}
	if !strings.Contains(got, "invoices") {
		t.Errorf("editor holds %q, want the table inserted", got)
	}
}

// An empty editor still gets the convenient starter query.
func TestChoosingATableFillsAnEmptyEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.do(keymap.ActionGoToTable)
	h.typeInto("invoices")
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	got := h.editorText()
	if !strings.HasPrefix(got, "SELECT") {
		t.Errorf("editor holds %q, want a starter query", got)
	}
}

// Whitespace is not work: a buffer holding only a newline should still get
// the starter query rather than an insertion.
func TestAWhitespaceOnlyEditorCountsAsEmpty(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("  \n\t ")

	h.do(keymap.ActionGoToTable)
	h.typeInto("invoices")
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	if got := h.editorText(); !strings.Contains(got, "SELECT") {
		t.Errorf("editor holds %q, want a starter query", got)
	}
}

// Inserting a name has to be one undo step, like every other edit command.
func TestInsertingATableNameIsUndoable(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("SELECT id FROM ")
	h.moveCaret(len("SELECT id FROM "))

	h.do(keymap.ActionGoToTable)
	h.typeInto("invoices")
	h.press(tcell.KeyDown)
	h.press(tcell.KeyEnter)

	h.undo()

	if got := h.editorText(); strings.Contains(got, "invoices") {
		t.Errorf("after one undo the editor holds %q, want the insertion reversed", got)
	}
}
