//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// The placeholder is what a newcomer reads before typing anything, and it
// has to be gone the moment they do: a hint left sitting beside real SQL
// reads as part of the statement.
func TestTheEmptyEditorShowsWhatToDoUntilSomethingIsTyped(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if !h.waitForScreen("Type a query") {
		t.Fatalf("an empty editor does not say what to do; screen:\n%s", h.text())
	}

	h.inspect(func(a *App) bool {
		a.app.SetFocus(a.editor)
		return true
	})
	h.inject(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))
	h.waitFor("the typed character", func(a *App) bool { return a.editor.GetText() == "S" })

	if strings.Contains(h.text(), "Type a query") {
		t.Errorf("the placeholder is still showing beside typed text; screen:\n%s", h.text())
	}
}

// The placeholder is drawn, never held. If it were text in the buffer, Run
// would send it to the server and copy would put it on the clipboard.
func TestThePlaceholderIsNeverPartOfWhatRuns(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if got := h.editorText(); got != "" {
		t.Fatalf("an untouched editor holds %q", got)
	}

	h.do(keymap.ActionRun)
	h.waitFor("the empty editor to be reported", func(a *App) bool {
		return a.status.message != ""
	})
	if h.inspect(func(a *App) bool { return a.status.phase != phaseIdle || a.running != nil }) {
		t.Error("an editor showing only its placeholder sent something to the server")
	}
}

// The tree takes a third of an eighty-column terminal, which is where a
// default one starts. The help key is what the placeholder ends on, so a hint
// wider than what is left would cut exactly that.
func TestTheEmptyEditorHintFitsBesideTheTreeAtEightyColumns(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.resize(80, 24)
	h.showSidebar()

	var want string
	h.inspect(func(a *App) bool {
		want = editorPlaceholder(a.sidebarVisible, a.helpKeyLabel())
		return true
	})
	if !h.waitForScreen(want) {
		t.Errorf("the empty editor's hint %q is not drawn whole at 80x24; screen:\n%s", want, h.text())
	}
}

// With the tree put away there is no table to double-click, and an empty
// editor still suggesting one points at something that is not on screen.
func TestHidingTheTreeTakesThePreviewOutOfTheEmptyEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.showSidebar()
	if !h.waitForScreen("dbl-click a table") {
		t.Fatalf("the empty editor does not offer the preview with the tree showing; screen:\n%s", h.text())
	}

	h.do(keymap.ActionToggleSidebar)
	h.waitFor("the schema pane to go", func(a *App) bool { return !a.sidebarVisible })
	if strings.Contains(h.text(), "dbl-click a table") {
		t.Errorf("the empty editor still offers the hidden tree's preview; screen:\n%s", h.text())
	}
}
