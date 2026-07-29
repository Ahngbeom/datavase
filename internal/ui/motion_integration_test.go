//go:build integration

package ui

import (
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// caret reads the editor's caret as a byte offset, from the UI goroutine.
func (h *harness) caret() int {
	h.t.Helper()

	var offset int
	done := make(chan struct{})
	h.app.app.QueueUpdateDraw(func() {
		offset = h.app.caretOffset(h.app.editor.GetText())
		close(done)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		h.t.Fatal("the interface stopped responding")
	}
	return offset
}

// selection reads the current selection from the UI goroutine.
func (h *harness) selection() string {
	h.t.Helper()

	var text string
	done := make(chan struct{})
	h.app.app.QueueUpdateDraw(func() {
		if h.app.editor.HasSelection() {
			text, _, _ = h.app.editor.GetSelection()
		}
		close(done)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		h.t.Fatal("the interface stopped responding")
	}
	return text
}

func TestWordMovementMovesTheCaret(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT user_id FROM t")
	h.moveCaret(len("SELECT user_id"))

	h.do(keymap.ActionWordLeft)
	if got := h.caret(); got != len("SELECT ") {
		t.Errorf("after word-left the caret is at %d, want %d", got, len("SELECT "))
	}

	h.do(keymap.ActionWordRight)
	if got := h.caret(); got != len("SELECT user_id") {
		t.Errorf("after word-right the caret is at %d, want %d", got, len("SELECT user_id"))
	}
}

// The underscore is what separates this from tview's own word movement,
// which would stop inside the identifier.
func TestWordMovementTreatsAnIdentifierAsOneWord(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT user_id")
	h.moveCaret(len("SELECT user_id"))

	h.do(keymap.ActionWordLeft)

	if got := h.caret(); got != len("SELECT ") {
		t.Errorf("caret is at %d, want %d — the underscore split the identifier",
			got, len("SELECT "))
	}
}

func TestLineMovement(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1\nFROM users")
	h.moveCaret(len("SELECT 1\nFROM"))

	h.do(keymap.ActionLineStart)
	if got := h.caret(); got != len("SELECT 1\n") {
		t.Errorf("after line-start the caret is at %d, want %d", got, len("SELECT 1\n"))
	}

	h.do(keymap.ActionLineEnd)
	if got := h.caret(); got != len("SELECT 1\nFROM users") {
		t.Errorf("after line-end the caret is at %d, want the end of the line", got)
	}
}

func TestSelectWordExtendsTheSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT alpha beta")
	h.moveCaret(len("SELECT "))

	h.do(keymap.ActionSelectWordRight)
	if got := h.selection(); got != "alpha" {
		t.Errorf("selection = %q, want %q", got, "alpha")
	}

	h.do(keymap.ActionSelectWordRight)
	if got := h.selection(); got != "alpha beta" {
		t.Errorf("selection = %q, want %q", got, "alpha beta")
	}
}

// The anchor's whole purpose: reversing direction shrinks the selection
// rather than flipping it to the other side.
func TestReversingDirectionShrinksTheSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT alpha beta gamma")
	h.moveCaret(len("SELECT "))

	h.do(keymap.ActionSelectWordRight)
	h.do(keymap.ActionSelectWordRight)
	if got := h.selection(); got != "alpha beta" {
		t.Fatalf("selection = %q, want %q", got, "alpha beta")
	}

	h.do(keymap.ActionSelectWordLeft)

	// "alpha " and not "alpha": moving left seeks a word's *start*, so the
	// caret lands at the beginning of "beta" rather than at the end of
	// "alpha". VS Code behaves the same way — the point here is that the
	// selection shrank from the right edge instead of flipping to the left
	// of the anchor.
	if got := h.selection(); got != "alpha " {
		t.Errorf("after reversing, selection = %q, want it shrunk to %q", got, "alpha ")
	}
}

// Shrinking all the way past the anchor turns the selection around, which is
// what dragging back beyond where you started should do.
func TestShrinkingPastTheAnchorReversesTheSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT alpha beta")
	h.moveCaret(len("SELECT "))

	h.do(keymap.ActionSelectWordRight) // "alpha"
	h.do(keymap.ActionSelectWordLeft)  // back to the anchor: empty
	h.do(keymap.ActionSelectWordLeft)  // past it

	if got := h.selection(); got != "SELECT " {
		t.Errorf("selection = %q, want %q once the caret passed the anchor", got, "SELECT ")
	}
}

func TestSelectToLineEndAndBack(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT alpha beta")
	h.moveCaret(len("SELECT "))

	h.do(keymap.ActionSelectLineEnd)
	if got := h.selection(); got != "alpha beta" {
		t.Errorf("selection = %q, want the rest of the line", got)
	}

	h.do(keymap.ActionSelectLineStart)
	if got := h.selection(); got != "SELECT " {
		t.Errorf("selection = %q, want %q", got, "SELECT ")
	}
}

// Plain movement drops the selection, as it does everywhere else.
func TestPlainMovementClearsTheSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT alpha beta")
	h.moveCaret(len("SELECT "))

	h.do(keymap.ActionSelectWordRight)
	if h.selection() == "" {
		t.Fatal("nothing was selected")
	}

	h.do(keymap.ActionWordRight)

	if got := h.selection(); got != "" {
		t.Errorf("selection = %q, want it cleared", got)
	}
}

func TestDeleteWordLeftThroughTheEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT user_id")
	h.moveCaret(len("SELECT user_id"))

	h.do(keymap.ActionDeleteWordLeft)

	if got := h.editorText(); got != "SELECT " {
		t.Errorf("editor holds %q, want %q", got, "SELECT ")
	}
}

func TestDeleteToLineStartThroughTheEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1\nFROM users")
	h.moveCaret(len("SELECT 1\nFROM users"))

	h.do(keymap.ActionDeleteToLineStart)

	if got := h.editorText(); got != "SELECT 1\n" {
		t.Errorf("editor holds %q, want %q", got, "SELECT 1\n")
	}
}

// Deleting with a selection removes the selection, not a word beyond it.
func TestDeleteWordWithASelectionRemovesTheSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT alpha beta")
	h.moveCaret(len("SELECT "))
	h.do(keymap.ActionSelectWordRight)

	h.do(keymap.ActionDeleteWordLeft)

	if got := h.editorText(); got != "SELECT  beta" {
		t.Errorf("editor holds %q, want the selection removed", got)
	}
}

// A deletion has to be one undo step, like the other edit commands.
func TestDeleteWordIsASingleUndoStep(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT user_id")
	h.moveCaret(len("SELECT user_id"))

	h.do(keymap.ActionDeleteWordLeft)
	if h.editorText() == "SELECT user_id" {
		t.Fatal("delete-word did nothing")
	}

	h.undo()

	if got := h.editorText(); got != "SELECT user_id" {
		t.Errorf("after one undo the editor holds %q, want the original text", got)
	}
}
