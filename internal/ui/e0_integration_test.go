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

// A session that assumed the default keyboard says which one, once. Anyone
// who wrote a config without a preset finds out here rather than by pressing
// a key that used to do something else.
func TestAnAssumedKeyboardIsAnnouncedOnce(t *testing.T) {
	h := newHarnessAssumingPreset(t, config.EnvDev)

	h.waitFor("the opening to name the assumed keyboard", func(a *App) bool {
		return strings.Contains(a.status.renderWidth(200), "datagrip")
	})

	h.runCommand("keymap vim")

	// "in the palette for another" is the assumed-keyboard clause's own
	// wording — nothing else this package renders says it — so this pins the
	// clause going, not the word "keymap", which setPreset's own confirmation
	// also uses for an unrelated reason.
	h.waitFor("the announcement to go once a keyboard is chosen", func(a *App) bool {
		return !strings.Contains(a.status.renderWidth(200), "in the palette for another")
	})
}
