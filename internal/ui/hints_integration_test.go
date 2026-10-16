//go:build integration

package ui

import (
	"os"
	"path/filepath"
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

func welcomeShowing(a *App) bool {
	name, _ := a.pages.GetFrontPage()
	return name == pageWelcome
}

// A newcomer is greeted once. A second session on the same machine opening
// with the same dialog would be a nag, and the one place that cost lands is
// on the people who use dv most.
func TestTheWelcomeGreetsAMachineOnceAndThenNoMore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datavase", "visits")

	first := newHarnessCounting(t, path)
	first.waitFor("the welcome", welcomeShowing)
	first.press(tcell.KeyEnter)
	first.waitFor("the welcome to give way to the editor", func(a *App) bool {
		return !welcomeShowing(a) && a.app.GetFocus() == a.editor
	})

	second := newHarnessCounting(t, path)
	if second.inspect(welcomeShowing) {
		t.Error("the welcome opened again for a machine that has already seen it")
	}
}

// Escape is what every other dialog here closes on, so it closes this one.
func TestEscapeDismissesTheWelcome(t *testing.T) {
	h := newHarnessCounting(t, filepath.Join(t.TempDir(), "visits"))

	h.waitFor("the welcome", welcomeShowing)
	h.press(tcell.KeyEscape)
	h.waitFor("the welcome to close", func(a *App) bool { return !welcomeShowing(a) })
}

// Somewhere the count cannot be written costs the memory of the welcome and
// nothing else: the session still opens and still runs statements.
func TestAnUnwritableStateDirectoryDoesNotCostTheSession(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	h := newHarnessCounting(t, filepath.Join(dir, "datavase", "visits"))
	h.waitFor("the welcome", welcomeShowing)
	h.press(tcell.KeyEscape)
	h.waitFor("the welcome to close", func(a *App) bool { return !welcomeShowing(a) })

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("the statement to finish", func(a *App) bool {
		return a.status.phase == phaseDone && a.status.rows == 1
	})
}

// A session opens with a tip on the bar, and loses it the moment the bar has
// something real to say.
func TestASessionOpensWithATipThatYieldsToTheFirstNotice(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	want := tipFor(0, keymap.Default())

	if !h.inspect(func(a *App) bool {
		return strings.Contains(a.currentStatus().renderWidth(200), want)
	}) {
		t.Fatalf("the opening bar carries no tip, want %q", want)
	}

	h.do(keymap.ActionRun)
	h.waitFor("the empty editor to be reported", func(a *App) bool { return a.status.message != "" })
	if h.inspect(func(a *App) bool {
		return strings.Contains(a.currentStatus().renderWidth(200), want)
	}) {
		t.Error("the tip is still on the bar after a real notice")
	}
}
