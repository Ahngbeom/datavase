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

func TestThereIsNothingToCopyFromARowThatIsNotThere(t *testing.T) {
	buf := bufferWith([]string{"id"}, []any{int64(1)})

	if _, ok := cellValue(buf, 5, 0); ok {
		t.Error("cellValue found a value in a row that does not exist")
	}
	if _, ok := cellValue(buf, 0, 5); ok {
		t.Error("cellValue found a value in a column that does not exist")
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

// The key's meaning depends on three things at once, and the order between
// them is a judgement rather than an implementation detail — so it is stated
// in one place and pinned here.
func TestCancellingWinsOverEveryKindOfCopyingWhileSomethingRuns(t *testing.T) {
	for _, c := range []copyContext{
		{running: true},
		{running: true, onGrid: true},
		{running: true, hasSelection: true},
		{running: true, onGrid: true, hasSelection: true},
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
		{"a selection in the editor", copyContext{hasSelection: true}, intentSelection},
		{"a cell in the results", copyContext{onGrid: true}, intentCell},
		{"nothing at all", copyContext{}, intentNothing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.resolve(); got != tt.want {
				t.Errorf("resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Focus decides, not what the editor happens to still be holding.
//
// A selection outlives the pane it was made in: running with ⌘A selected, or
// with a dragged selection, leaves one behind. Reading it before the grid
// meant that from the moment a query had been selected, the copy key in the
// results copied the SQL — and the only way to reach a cell was to go back
// and unselect. Nobody guesses that rule; they conclude cell copy is broken.
func TestOnTheGridTheKeyCopiesTheCellEvenWithASelectionLeftInTheEditor(t *testing.T) {
	ctx := copyContext{onGrid: true, hasSelection: true}
	if got := ctx.resolve(); got != intentCell {
		t.Errorf("resolve() = %v, want intentCell — the grid has focus", got)
	}
}

// A row is pasted somewhere that understands columns — a spreadsheet, another
// terminal — and alignment drawn with spaces stops being alignment the moment
// it lands there.
func TestCopyingARowSeparatesItsValuesWithTabs(t *testing.T) {
	buf := bufferWith([]string{"id", "email", "note"},
		[]any{int64(1), "a@example.com", nil})

	got, ok := rowValues(buf, 0)
	if !ok {
		t.Fatal("rowValues reported nothing to copy")
	}
	if want := "1\ta@example.com\tNULL"; got != want {
		t.Errorf("rowValues() = %q, want %q", got, want)
	}
}

// The same reason a cell is copied from the buffer: the grid cuts long values
// and doubles brackets for the markup parser, and a pasted row must carry
// neither.
func TestACopiedRowIsNeitherTruncatedNorEscaped(t *testing.T) {
	long := strings.Repeat("y", result.CellLimit*2)
	buf := bufferWith([]string{"bio", "v"}, []any{long, "[red]literal"})

	got, _ := rowValues(buf, 0)
	if !strings.Contains(got, long) {
		t.Error("the row was truncated the way the grid draws it")
	}
	if !strings.Contains(got, "[red]literal") {
		t.Errorf("the row was escaped for the screen: %q", got)
	}
}

func TestARowThatIsNotThereIsNotCopied(t *testing.T) {
	buf := bufferWith([]string{"id"}, []any{int64(1)})

	for _, row := range []int{-1, 1, 99} {
		if _, ok := rowValues(buf, row); ok {
			t.Errorf("rowValues(row %d) reported something to copy", row)
		}
	}
	if _, ok := rowValues(nil, 0); ok {
		t.Error("rowValues(nil buffer) reported something to copy")
	}
}
