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

func TestUseSchemaOpensASearchablePicker(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.do(keymap.ActionUseSchema)

	got := h.text()
	if !strings.Contains(got, "schema") {
		t.Errorf("the schema picker did not open:\n%s", got)
	}
	if !strings.Contains(got, testmysql.DefaultDatabase) {
		t.Errorf("the picker does not list the current schema:\n%s", got)
	}
}

// Choosing a schema has to change where an unqualified query actually goes,
// not merely what the status bar says.
func TestUseSchemaChangesWhereQueriesRun(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.inspect(func(a *App) bool {
		a.useSchema("information_schema")
		return true
	})
	h.waitFor("the new schema", func(a *App) bool {
		return a.currentSchema() == "information_schema"
	})

	h.typeSQL("SELECT DATABASE()")
	h.do(keymap.ActionRun)

	h.waitForScreen("information_schema")
}

// And the status bar has to agree, since it is the only thing on screen that
// says which schema an unqualified name will hit.
func TestUseSchemaShowsInTheStatusBar(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.inspect(func(a *App) bool {
		a.useSchema("information_schema")
		return true
	})

	h.waitFor("the top bar to follow", func(a *App) bool {
		return a.currentTopBar().schema == "information_schema"
	})
	if !strings.Contains(h.text(), "@information_schema") {
		t.Errorf("the top bar does not show the chosen schema:\n%s", h.text())
	}
}

// The picker is reachable by its key from anywhere, including mid-edit.
func TestUseSchemaPickerCloses(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())

	h.do(keymap.ActionUseSchema)
	h.press(tcell.KeyEscape)

	if strings.Contains(h.text(), "choose a schema") {
		t.Errorf("the schema picker did not close:\n%s", h.text())
	}
}
