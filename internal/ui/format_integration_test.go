//go:build integration

package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

func TestFormattingLaysTheWholeQueryOutByClause(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.buffer("select a, b from t where x = 1", len("select a, b "))

	h.do(keymap.ActionFormat)

	want := "SELECT a,\n    b\nFROM t\nWHERE x = 1"
	h.waitFor("the query to be formatted", func(a *App) bool { return a.editor.GetText() == want })
	if got := h.caret(); !strings.HasPrefix(want[got:], "FROM") {
		t.Errorf("the caret moved to %q, want it to stay on FROM", want[got:])
	}
}

// Formatting replaces text rather than resetting the buffer, so one undo
// brings back what was there — a reformat the user disagrees with is not a
// one-way door.
func TestAFormatCanBeUndone(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.buffer("select 1 from t", 0)

	h.do(keymap.ActionFormat)
	h.waitFor("the query to be formatted", func(a *App) bool { return a.editor.GetText() != "select 1 from t" })

	h.undo()
	if got := h.editorText(); got != "select 1 from t" {
		t.Errorf("after undo the editor holds %q, want the original", got)
	}
}

func TestFormattingASelectionLeavesTheRestAlone(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	first := "select 1 from a;\n"
	h.buffer(first+"select 2 from b;", 0)
	h.inspect(func(a *App) bool {
		a.editor.Select(len(first), len(a.editor.GetText()))
		return true
	})

	h.do(keymap.ActionFormat)

	want := first + "SELECT 2\nFROM b;"
	h.waitFor("the selection to be formatted", func(a *App) bool { return a.editor.GetText() == want })
}

func TestAQueryThatCannotBeFormattedSaysWhy(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	sql := "select /*!40001 SQL_NO_CACHE */ * from t"
	h.buffer(sql, 0)

	h.do(keymap.ActionFormat)

	h.waitFor("a notice saying why", func(a *App) bool { return strings.Contains(a.status.message, "/*!") })
	if got := h.editorText(); got != sql {
		t.Errorf("the editor was changed to %q", got)
	}
}

// A query copied on macOS arrives in a bracketed paste with LF line ends,
// which tcell reports as Ctrl+J; the query used to land on one line.
func TestAPastedQueryKeepsItsLines(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.buffer("", 0)

	events := []tcell.Event{tcell.NewEventPaste(true)}
	for i, line := range []string{"SELECT a", "FROM t"} {
		if i > 0 {
			events = append(events, tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl))
		}
		for _, r := range line {
			events = append(events, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		}
	}
	events = append(events, tcell.NewEventPaste(false))
	// The simulation's queue is short; a paste is longer than it.
	for _, ev := range events {
		for h.screen.PostEvent(ev) != nil {
			time.Sleep(time.Millisecond)
		}
	}

	h.waitFor("the pasted query, line breaks intact", func(a *App) bool {
		return a.editor.GetText() == "SELECT a\nFROM t"
	})
}

// The highlighter has to be what is on screen, not just a type that exists:
// left out of the layout, it draws nothing and every test of it still passes.
func TestTheEditorOnScreenColoursItsKeywords(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.buffer("select 1 from dual", 0)

	h.waitFor("FROM drawn in the keyword colour", func(a *App) bool {
		cells, width, height := h.screen.GetContents()
		for row := 0; row < height; row++ {
			var line strings.Builder
			for col := 0; col < width; col++ {
				line.Write(cells[row*width+col].Bytes)
			}
			col := strings.Index(line.String(), "select 1 from dual")
			if col < 0 {
				continue
			}
			fg, _, _ := cells[row*width+col+len("select 1 ")].Style.Decompose()
			return fg == colourKeyword
		}
		return false
	})
}
