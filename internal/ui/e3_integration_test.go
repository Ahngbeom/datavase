//go:build integration

package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/vim"
	"github.com/gdamore/tcell/v2"
)

// newVimHarness starts the interface on the vim keyboard.
//
// The preset is stated rather than assumed so that these tests keep saying
// what they mean after the default changes.
func newVimHarness(t *testing.T) *harness {
	t.Helper()

	h := newHarness(t, config.EnvDev)
	h.inspect(func(a *App) bool {
		a.setPreset(keymap.PresetVim)
		return true
	})
	h.waitFor("the vim keyboard", func(a *App) bool { return a.keys.Modal() })
	return h
}

// buffer puts text in the editor with the caret at an offset, focused.
func (h *harness) buffer(text string, caret int) {
	h.t.Helper()

	h.inspect(func(a *App) bool {
		a.editor.SetText(text, false)
		a.app.SetFocus(a.editor)
		return true
	})
	// The caret is placed in a later tick on purpose. TextArea rebuilds its
	// row index lazily, and a Select issued in the same tick as SetText is
	// resolved against the row index of the text that has just been thrown
	// away — it lands on the wrong line.
	h.inspect(func(a *App) bool {
		a.editor.Select(caret, caret)
		a.clearAnchor()
		return true
	})
	h.settle()
}

// wantEditor waits for the editor to hold exactly this text.
func (h *harness) wantEditor(want string) {
	h.t.Helper()

	var got string
	h.waitFor("the editor to hold "+want, func(a *App) bool {
		got = a.editor.GetText()
		return got == want
	})
}

// The failure everyone means by "the keyboard stopped working": a letter
// typed in normal mode ending up in the buffer.
func TestNormalModeNeverTypesIntoTheEditor(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("SELECT 1", 0)

	h.typeInto("zq!")
	h.wantEditor("SELECT 1")
}

func TestInsertModeTypes(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("", 0)

	h.typeInto("iSELECT 1")
	h.wantEditor("SELECT 1")

	h.waitFor("insert mode", func(a *App) bool { return a.vim.Mode() == vim.ModeInsert })
}

func TestVimMotionsMoveTheCaret(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("SELECT id\nFROM t", 0)

	// w to the second word, then insert before it.
	h.typeInto("wiX")
	h.wantEditor("SELECT Xid\nFROM t")
}

func TestVimOperators(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		caret int
		keys  string
		want  string
	}{
		{
			name: "dw takes the word and the space after it",
			text: "SELECT id FROM t", caret: 7,
			keys: "dw", want: "SELECT FROM t",
		},
		{
			name: "db takes the word before the caret",
			text: "SELECT id", caret: 7,
			keys: "db", want: "id",
		},
		{
			name: "d$ takes the rest of the line",
			text: "SELECT id\nFROM t", caret: 6,
			keys: "d$", want: "SELECT\nFROM t",
		},
		{
			name: "dd takes the whole line",
			text: "SELECT id\nFROM t", caret: 3,
			keys: "dd", want: "FROM t",
		},
		{
			name: "dd on the last line leaves no blank behind",
			text: "SELECT id\nFROM t", caret: 12,
			keys: "dd", want: "SELECT id",
		},
		{
			name: "x takes one character",
			text: "SELECT", caret: 0,
			keys: "x", want: "ELECT",
		},
		{
			name: "D takes to the end of the line",
			text: "SELECT id\nFROM t", caret: 6,
			keys: "D", want: "SELECT\nFROM t",
		},
		{
			name: "cw replaces a word and leaves you typing",
			text: "SELECT id", caret: 7,
			keys: "cwname", want: "SELECT name",
		},
		{
			name: "cc clears the line and keeps its indent",
			text: "SELECT id\n  FROM t", caret: 13,
			keys: "ccWHERE x", want: "SELECT id\n  WHERE x",
		},
		{
			name: "dgg takes everything above",
			text: "a\nb\nc", caret: 4,
			keys: "dgg", want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newVimHarness(t)
			h.buffer(tt.text, tt.caret)

			h.typeInto(tt.keys)
			h.wantEditor(tt.want)
		})
	}
}

// A command that cannot be reversed with a single undo is one people stop
// trusting, so each operator has to be exactly one edit.
func TestVimOperatorsUndoInOneStep(t *testing.T) {
	for _, keys := range []string{"dd", "dw", "cw", "x", "D"} {
		t.Run(keys, func(t *testing.T) {
			const text = "SELECT id\nFROM t"

			h := newVimHarness(t)
			h.buffer(text, 7)

			h.typeInto(keys)
			h.waitFor("the edit to happen", func(a *App) bool {
				return a.editor.GetText() != text
			})

			// Escape first: cw left us in insert mode, where u is a letter.
			h.press(tcell.KeyEscape)
			h.typeInto("u")
			h.wantEditor(text)
		})
	}
}

func TestYankAndPut(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		caret int
		keys  string
		want  string
	}{
		{
			name: "yy then p puts the line below",
			text: "SELECT id\nFROM t", caret: 0,
			keys: "yyp", want: "SELECT id\nSELECT id\nFROM t",
		},
		{
			name: "yy then P puts the line above",
			text: "SELECT id\nFROM t", caret: 12,
			keys: "yyP", want: "SELECT id\nFROM t\nFROM t",
		},
		{
			name: "yw then p puts the word after the caret",
			text: "ab cd", caret: 0,
			keys: "ywp", want: "aab b cd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newVimHarness(t)
			h.buffer(tt.text, tt.caret)

			h.typeInto(tt.keys)
			h.wantEditor(tt.want)
		})
	}
}

func TestVisualSelectionExtendsAndDeletes(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("SELECT id FROM t", 7)

	h.typeInto("v")
	h.waitFor("visual mode", func(a *App) bool { return a.vim.Mode() == vim.ModeVisual })

	// Extend over "id " and delete it.
	h.typeInto("w")
	h.typeInto("d")
	h.wantEditor("SELECT FROM t")

	h.waitFor("normal mode", func(a *App) bool { return a.vim.Mode() == vim.ModeNormal })
}

func TestVisualLineSelection(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("a\nb\nc", 2)

	h.typeInto("Vd")
	h.wantEditor("a\nc")
}

// Escape has to work from every mode, including out of a half-typed
// sequence, or a mistyped key leaves the user stuck.
func TestEscapeReturnsToNormalMode(t *testing.T) {
	for _, keys := range []string{"i", "v", "V", "d"} {
		t.Run(keys, func(t *testing.T) {
			h := newVimHarness(t)
			h.buffer("SELECT id", 0)

			h.typeInto(keys)
			h.press(tcell.KeyEscape)

			h.waitFor("normal mode with nothing pending", func(a *App) bool {
				return a.vim.Mode() == vim.ModeNormal && a.vim.Pending() == ""
			})

			// And the buffer is untouched by any of it.
			h.wantEditor("SELECT id")
		})
	}
}

// Opening a line carries the indentation, which is what makes o usable in a
// formatted statement.
func TestOpenLine(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("SELECT id\n  FROM t", 13)

	h.typeInto("oWHERE x")
	h.wantEditor("SELECT id\n  FROM t\n  WHERE x")
}

func TestOpenLineAbove(t *testing.T) {
	h := newVimHarness(t)
	h.buffer("  FROM t", 3)

	h.typeInto("OSELECT id")
	h.wantEditor("  SELECT id\n  FROM t")
}

// The DataGrip keyboard must be unaffected: j is a letter there.
func TestNonModalPresetStillTypes(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.buffer("", 0)

	h.typeInto("jk")
	h.wantEditor("jk")
}
