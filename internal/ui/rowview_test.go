package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/result"
)

func bufferWith(columns []string, rows ...[]any) *result.Buffer {
	buf := result.NewBuffer(100)
	buf.SetColumns(columns, nil)
	buf.Append(rows)
	return buf
}

// The grid cuts a value at CellLimit, which is the whole reason this view
// exists: a JSON column or a TEXT blob is unreadable there and there was no
// other way to see the rest of it.
func TestTheRowViewShowsAValueTheGridHadToCutShort(t *testing.T) {
	long := strings.Repeat("x", result.CellLimit*2)
	buf := bufferWith([]string{"id", "bio"}, []any{int64(1), long})

	if strings.Contains(buf.Cell(0, 1), long) {
		t.Fatal("the grid is not truncating, so this test proves nothing")
	}

	got := rowDetail(buf, 0)
	if !strings.Contains(got, long) {
		t.Errorf("the row view cut the value short too:\n%s", got)
	}
}

// A row is read by looking down the names, so they have to line up. Without
// that the view is a wall of text with the answer somewhere in it.
func TestTheRowViewLinesTheColumnNamesUp(t *testing.T) {
	buf := bufferWith([]string{"id", "a_much_longer_name"},
		[]any{int64(1), "second"})

	lines := strings.Split(strings.TrimSpace(rowDetail(buf, 0)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want one per column:\n%s", len(lines), rowDetail(buf, 0))
	}

	first := strings.Index(lines[0], "1")
	second := strings.Index(lines[1], "second")
	if first != second {
		t.Errorf("values start at columns %d and %d, so they do not line up:\n%s",
			first, second, rowDetail(buf, 0))
	}
}

// NULL has to stay distinguishable from the four-letter string a column might
// actually hold, the same way it is in the grid.
func TestTheRowViewMarksNullApartFromTheWordNull(t *testing.T) {
	buf := bufferWith([]string{"absent", "literal"}, []any{nil, "NULL"})

	got := rowDetail(buf, 0)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), got)
	}
	if lines[0] == lines[1] {
		t.Errorf("a real NULL and the string \"NULL\" render identically:\n%s", got)
	}
}

// Markup in the data must not be read as markup. A value containing "[red]"
// would otherwise colour the rest of the view.
func TestTheRowViewDoesNotLetDataColourTheScreen(t *testing.T) {
	buf := bufferWith([]string{"v"}, []any{"[red]not a tag"})

	if got := rowDetail(buf, 0); !strings.Contains(got, "[[red]") {
		t.Errorf("the value was not escaped for the markup parser:\n%s", got)
	}
}

// Asking for a row that is not there happens: the buffer is cleared while the
// view is open, or a stream is still filling it.
func TestTheRowViewIsEmptyForARowThatIsNotThere(t *testing.T) {
	buf := bufferWith([]string{"id"}, []any{int64(1)})

	for _, row := range []int{-1, 1, 99} {
		if got := rowDetail(buf, row); got != "" {
			t.Errorf("rowDetail(%d) = %q, want empty", row, got)
		}
	}
}
