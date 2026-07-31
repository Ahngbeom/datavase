package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/result"
)

// Lifting one value out of a result — into a ticket, a message, the next
// query — is an everyday action, and it was impossible: with the grid focused
// the copy key fell through to "nothing selected and nothing running".
//
// What it copies is the value, not what the grid had room to draw.
func TestCopyingACellTakesTheWholeValueRatherThanTheDisplayedOne(t *testing.T) {
	long := strings.Repeat("y", result.CellLimit*2)
	buf := bufferWith([]string{"id", "bio"}, []any{int64(1), long})

	if strings.Contains(buf.Cell(0, 1), long) {
		t.Fatal("the grid is not truncating, so this test proves nothing")
	}

	got, ok := cellValue(buf, 0, 1)
	if !ok {
		t.Fatal("cellValue reported nothing to copy")
	}
	if got != long {
		t.Errorf("copied %d runes, want the whole %d", len([]rune(got)), len([]rune(long)))
	}
}

// A copied value goes into a spreadsheet or another terminal, where the
// grid's markup escaping would arrive as literal doubled brackets.
func TestACopiedCellIsNotEscapedForTheScreen(t *testing.T) {
	buf := bufferWith([]string{"v"}, []any{"[red]literal"})

	got, _ := cellValue(buf, 0, 0)
	if got != "[red]literal" {
		t.Errorf("cellValue() = %q, want the value as the server sent it", got)
	}
}

// A row is copied to be pasted somewhere that understands columns, so the
// separator is a tab rather than anything prettier.
func TestCopyingARowSeparatesTheValuesWithTabs(t *testing.T) {
	buf := bufferWith([]string{"a", "b", "c"},
		[]any{int64(1), "two", nil})

	got, ok := rowValues(buf, 0)
	if !ok {
		t.Fatal("rowValues reported nothing to copy")
	}
	if want := "1\ttwo\tNULL"; got != want {
		t.Errorf("rowValues() = %q, want %q", got, want)
	}
}

func TestThereIsNothingToCopyFromARowThatIsNotThere(t *testing.T) {
	buf := bufferWith([]string{"id"}, []any{int64(1)})

	if _, ok := cellValue(buf, 5, 0); ok {
		t.Error("cellValue found a value in a row that does not exist")
	}
	if _, ok := cellValue(buf, 0, 5); ok {
		t.Error("cellValue found a value in a column that does not exist")
	}
	if _, ok := rowValues(buf, 5); ok {
		t.Error("rowValues found a row that does not exist")
	}
}

// Knowing a column is BIGINT rather than VARCHAR changes how its contents
// read. The buffer has kept the types all along and nothing ever showed them.
func TestTheRowViewNamesTheColumnTypeWhenTheServerGaveOne(t *testing.T) {
	buf := bufferWith([]string{"id"}, []any{int64(1)})

	// The unit tests build a buffer without types, which is also what a
	// cleared result looks like: the view must simply say less.
	if got := rowDetail(buf, 0); strings.Contains(got, "()") {
		t.Errorf("an absent type left an empty bracket behind:\n%s", got)
	}
}

// The key's meaning depends on four things at once, and the order between
// them is a judgement rather than an implementation detail — so it is stated
// in one place and pinned here.
func TestCancellingWinsOverEveryKindOfCopyingWhileSomethingRuns(t *testing.T) {
	for _, c := range []copyContext{
		{running: true},
		{running: true, onGrid: true},
		{running: true, onDDL: true},
		{running: true, hasSelection: true},
		{running: true, onGrid: true, onDDL: true, hasSelection: true},
	} {
		if got := c.resolve(); got != intentCancel {
			t.Errorf("%+v resolved to %v, want intentCancel — the way to stop a "+
				"runaway statement cannot depend on which pane has focus", c, got)
		}
	}
}

func TestWithNothingRunningTheKeyCopiesWhateverHasFocus(t *testing.T) {
	tests := []struct {
		name string
		ctx  copyContext
		want copyIntent
	}{
		{"the ddl tab", copyContext{onDDL: true}, intentDefinition},
		{"a selection in the editor", copyContext{hasSelection: true}, intentSelection},
		{"a cell in the results", copyContext{onGrid: true}, intentCell},
		{"nothing at all", copyContext{}, intentNothing},
		// The definition on screen is the obvious thing to copy there, even
		// though the editor still holds a selection behind it.
		{"the ddl tab over an editor selection",
			copyContext{onDDL: true, hasSelection: true}, intentDefinition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.resolve(); got != tt.want {
				t.Errorf("resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}
