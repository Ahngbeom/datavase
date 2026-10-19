//go:build integration

package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// screenRows is the drawn screen as one rune slice per row, so a position
// found in it is a screen column.
func (h *harness) screenRows() [][]rune {
	h.t.Helper()

	var rows [][]rune
	h.inspect(func(a *App) bool {
		cells, width, height := h.screen.GetContents()
		for row := 0; row < height; row++ {
			var line []rune
			for col := 0; col < width; col++ {
				r := []rune(string(cells[row*width+col].Bytes))
				if len(r) == 0 {
					r = []rune{' '}
				}
				line = append(line, r[0])
			}
			rows = append(rows, line)
		}
		return true
	})
	return rows
}

// locate is the screen position of the first row drawing want.
func locate(rows [][]rune, want string) (x, y int, ok bool) {
	for y, row := range rows {
		if i := strings.Index(string(row), want); i >= 0 {
			return len([]rune(string(row)[:i])), y, true
		}
	}
	return 0, 0, false
}

// openCustomerCompletion types a word two tables match and asks for
// completion, which has to show a menu rather than insert one.
func openCustomerCompletion(h *harness) {
	h.t.Helper()
	h.seedCache(completionSnapshot())
	h.buffer("SELECT * FROM customer", len("SELECT * FROM customer"))

	h.do(keymap.ActionComplete)
	h.waitFor("the completion menu", func(a *App) bool { return a.menu.isOpen() })
}

// The reported problem: the list opened as a separate window, taking the
// keyboard away from the query it was completing and hiding it behind a
// dialog.
func TestCompletionLeavesTheEditorInFrontAndFocused(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	if !h.inspect(func(a *App) bool { return a.app.GetFocus() == a.editor }) {
		t.Error("the completion menu took the keyboard away from the editor")
	}
	if name := h.inspectString(func(a *App) string { name, _ := a.pages.GetFrontPage(); return name }); name != pageMain {
		t.Errorf("completion opened the %q page in front of the editor", name)
	}
	got := h.text()
	for _, want := range []string{"SELECT * FROM customer", "customers", "customer_notes"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is not on screen while the menu is open:\n%s", want, got)
		}
	}
}

// A menu drawn anywhere else makes the eye leave the word being completed.
func TestTheCompletionMenuOpensUnderTheWordBeingCompleted(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	rows := h.screenRows()
	x, y, ok := locate(rows, "SELECT * FROM customer")
	if !ok {
		t.Fatalf("the query is not on screen:\n%s", h.text())
	}
	wordX := x + len("SELECT * FROM ")

	below := string(rows[y+1][wordX:])
	if !strings.HasPrefix(below, "customer") {
		t.Errorf("the row under the caret, from the word's column, reads %q; want the first candidate there:\n%s",
			below, h.text())
	}
}

// Typing while the menu is open is how a long list gets short; a menu that
// had to be closed and reopened for each letter would not be worth opening.
func TestTypingWhileTheMenuIsOpenNarrowsIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	h.typeRunes("_n")

	if got := h.editorText(); got != "SELECT * FROM customer_n" {
		t.Fatalf("typing with the menu open left the editor holding %q", got)
	}
	if !h.inspect(func(a *App) bool { return a.menu.isOpen() }) {
		t.Fatalf("the menu closed although customer_notes still matches:\n%s", h.text())
	}
	got := h.text()
	if !strings.Contains(got, "customer_notes") {
		t.Errorf("the narrowed menu dropped the one table that still matches:\n%s", got)
	}
	if strings.Contains(got, "customers") {
		t.Errorf("the menu still offers customers after typing past it:\n%s", got)
	}
}

// Backspace edits too, and widens the list again.
func TestBackspaceWhileTheMenuIsOpenWidensIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)
	h.typeRunes("_")
	if strings.Contains(h.text(), "customers") {
		t.Fatalf("typing _ did not narrow the menu, so widening proves nothing:\n%s", h.text())
	}

	h.press(tcell.KeyBackspace2)

	if got := h.editorText(); got != "SELECT * FROM customer" {
		t.Fatalf("Backspace with the menu open left the editor holding %q", got)
	}
	if !strings.Contains(h.text(), "customers") {
		t.Errorf("deleting a character did not bring customers back into the menu:\n%s", h.text())
	}
}

// Nothing left to offer means nothing left to show; an empty box over the
// query would hide text for no reason.
func TestTypingPastEveryCandidateClosesTheMenu(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	h.typeRunes("zzz")

	if h.inspect(func(a *App) bool { return a.menu.isOpen() }) {
		t.Errorf("the menu is still open with nothing matching:\n%s", h.text())
	}
	if got := h.editorText(); got != "SELECT * FROM customerzzz" {
		t.Errorf("the editor holds %q", got)
	}
}

// Enter takes the highlighted candidate, written over the word rather than
// after it.
func TestEnterAcceptsTheHighlightedCandidateOverTheWord(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	h.press(tcell.KeyDown)
	want := "SELECT * FROM " + h.inspectString(func(a *App) string { return a.menu.current().Text })
	h.press(tcell.KeyEnter)

	if got := h.editorText(); got != want {
		t.Errorf("after Enter the editor holds %q, want %q", got, want)
	}
	if h.inspect(func(a *App) bool { return a.menu.isOpen() }) {
		t.Error("the menu stayed open after accepting")
	}
}

// Tab is the other key every editor accepts a completion with. Without the
// menu it moves focus; with it, it must not.
func TestTabAcceptsACandidateRatherThanLeavingTheEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	want := "SELECT * FROM " + h.inspectString(func(a *App) string { return a.menu.current().Text })
	h.press(tcell.KeyTab)

	if got := h.editorText(); got != want {
		t.Errorf("after Tab the editor holds %q, want %q", got, want)
	}
	if !h.inspect(func(a *App) bool { return a.app.GetFocus() == a.editor }) {
		t.Error("Tab moved focus out of the editor instead of accepting")
	}
}

// Escape is how a menu that was not wanted goes away, and it must not take
// any of the typing with it.
func TestEscapeClosesTheMenuAndLeavesTheTextAlone(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	h.press(tcell.KeyEscape)

	if h.inspect(func(a *App) bool { return a.menu.isOpen() }) {
		t.Fatalf("the menu is still open after Escape:\n%s", h.text())
	}
	if got := h.editorText(); got != "SELECT * FROM customer" {
		t.Errorf("Escape changed the editor to %q", got)
	}
	if strings.Contains(h.text(), "customer_notes") {
		t.Errorf("the menu is still drawn after Escape:\n%s", h.text())
	}
	h.typeRunes("s")
	if got := h.editorText(); got != "SELECT * FROM customers" {
		t.Errorf("typing after Escape left the editor holding %q", got)
	}
}

// Moving the caret off the word means the menu is completing something the
// caret is no longer on.
func TestMovingTheCaretClosesTheMenu(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	h.press(tcell.KeyLeft)

	if h.inspect(func(a *App) bool { return a.menu.isOpen() }) {
		t.Errorf("the menu is still open after the caret moved:\n%s", h.text())
	}
	if got := h.caret(); got != len("SELECT * FROM custome") {
		t.Errorf("Left with the menu open put the caret at %d; it should still move", got)
	}
}

// A candidate is something on screen to point at.
func TestClickingACandidateAcceptsIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	rows := h.screenRows()
	x, y, ok := locate(rows, "customer_notes")
	if !ok {
		t.Fatalf("customer_notes is not on screen:\n%s", h.text())
	}
	h.click(x, y)

	if got := h.editorText(); got != "SELECT * FROM customer_notes" {
		t.Errorf("clicking customer_notes left the editor holding %q", got)
	}
}

// paste delivers text the way a terminal's bracketed paste does, which
// bypasses the key router entirely.
func (h *harness) paste(text string) {
	h.t.Helper()
	events := []tcell.Event{tcell.NewEventPaste(true)}
	for _, r := range text {
		events = append(events, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	events = append(events, tcell.NewEventPaste(false))
	// The simulation's queue is short; a paste can be longer than it.
	for _, ev := range events {
		for h.screen.PostEvent(ev) != nil {
			time.Sleep(time.Millisecond)
		}
	}
}

// A paste is typing too. Left alone, the menu kept offering what matched
// before it, over a word that no longer matched them.
func TestPastingWhileTheMenuIsOpenNarrowsIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	h.paste("_n")
	h.waitFor("the pasted text", func(a *App) bool { return a.editor.GetText() == "SELECT * FROM customer_n" })

	if !h.inspect(func(a *App) bool { return a.menu.isOpen() }) {
		t.Fatalf("the menu closed although customer_notes still matches:\n%s", h.text())
	}
	if strings.Contains(h.text(), "customers") {
		t.Errorf("the menu still offers customers after a paste went past it:\n%s", h.text())
	}
}

func TestPastingPastEveryCandidateClosesTheMenu(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	h.paste("zzz")
	h.waitFor("the pasted text", func(a *App) bool { return a.editor.GetText() == "SELECT * FROM customerzzz" })

	if h.inspect(func(a *App) bool { return a.menu.isOpen() }) {
		t.Errorf("the menu is still open after a paste nothing matches:\n%s", h.text())
	}
}

// In a real terminal the rows that were not highlighted came out in exactly
// the editor's own colours, with no border, so they read as more of the
// query rather than as a list.
func TestTheMenuIsDrawnOnASurfaceOfItsOwn(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	openCustomerCompletion(h)

	var menuBg, editorBg tcell.Color
	var where string
	h.inspect(func(a *App) bool {
		b := a.menu.box
		// The row after the highlighted one, against the editor's own text
		// directly above the menu: the word being completed.
		row := b.y + 1
		cells, width, _ := h.screen.GetContents()
		_, menuBg, _ = cells[row*width+b.x].Style.Decompose()
		_, editorBg, _ = cells[(b.y-1)*width+b.x].Style.Decompose()
		where = fmt.Sprintf("column %d, rows %d and %d", b.x, row, b.y-1)
		return true
	})

	if menuBg == editorBg {
		t.Errorf("an unhighlighted menu row has the editor's background %v (%s):\n%s",
			menuBg, where, h.text())
	}
}

// A small terminal must not push the menu off the screen or over the very
// line it is completing.
func TestTheCompletionMenuFitsASmallTerminal(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.resize(50, 16)
	// Beside the schema pane the editor is too narrow to show the query.
	h.do(keymap.ActionToggleSidebar)
	h.waitFor("the schema pane to go", func(a *App) bool { return !a.sidebarVisible })
	openCustomerCompletion(h)

	rows := h.screenRows()
	_, y, ok := locate(rows, "SELECT * FROM customer")
	if !ok {
		t.Fatalf("the menu covers the line it is completing:\n%s", h.text())
	}
	if _, _, ok := locate(rows, "customer_notes"); !ok {
		t.Errorf("the menu is not on screen at 50x16:\n%s", h.text())
	}
	if _, cy, _ := locate(rows, "customer_notes"); cy == y {
		t.Errorf("the menu is drawn on the caret's own row:\n%s", h.text())
	}
}
