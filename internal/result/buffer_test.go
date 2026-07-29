package result

import (
	"sync"
	"testing"
)

func TestEmptyBuffer(t *testing.T) {
	b := NewBuffer(0)

	if got := b.RowCount(); got != 0 {
		t.Errorf("RowCount() = %d, want 0", got)
	}
	if got := b.ColumnCount(); got != 0 {
		t.Errorf("ColumnCount() = %d, want 0", got)
	}
}

func TestBufferHoldsColumnsAndRows(t *testing.T) {
	b := NewBuffer(0)
	b.SetColumns([]string{"id", "email"}, nil)
	b.Append([][]any{
		{int64(1), []byte("a@example.com")},
		{int64(2), nil},
	})

	if got := b.ColumnCount(); got != 2 {
		t.Errorf("ColumnCount() = %d, want 2", got)
	}
	if got := b.RowCount(); got != 2 {
		t.Errorf("RowCount() = %d, want 2", got)
	}
	if got := b.ColumnName(1); got != "email" {
		t.Errorf("ColumnName(1) = %q, want %q", got, "email")
	}
	if got := b.Cell(0, 1); got != "a@example.com" {
		t.Errorf("Cell(0,1) = %q, want %q", got, "a@example.com")
	}
	if got := b.Cell(1, 1); got != NullText {
		t.Errorf("Cell(1,1) = %q, want %q", got, NullText)
	}
}

// tview calls GetCell while rendering, and a concurrent Reset can shrink the
// buffer underneath it. Returning empty text is right; panicking would take
// the whole application down and lose the user's unsaved SQL.
func TestBufferReadsOutOfRangeSafely(t *testing.T) {
	b := NewBuffer(0)
	b.SetColumns([]string{"id"}, nil)
	b.Append([][]any{{int64(1)}})

	cases := []struct{ row, col int }{
		{row: -1, col: 0},
		{row: 0, col: -1},
		{row: 5, col: 0},
		{row: 0, col: 5},
		{row: 99, col: 99},
	}

	for _, c := range cases {
		if got := b.Cell(c.row, c.col); got != "" {
			t.Errorf("Cell(%d,%d) = %q, want an empty string", c.row, c.col, got)
		}
	}
	if got := b.ColumnName(9); got != "" {
		t.Errorf("ColumnName(9) = %q, want an empty string", got)
	}
}

func TestBufferResetClearsEverything(t *testing.T) {
	b := NewBuffer(0)
	b.SetColumns([]string{"id"}, nil)
	b.Append([][]any{{int64(1)}})

	b.Reset()

	if got := b.RowCount(); got != 0 {
		t.Errorf("RowCount() after Reset = %d, want 0", got)
	}
	if got := b.ColumnCount(); got != 0 {
		t.Errorf("ColumnCount() after Reset = %d, want 0", got)
	}
}

// The cap keeps a runaway result from exhausting memory; the buffer reports
// that it stopped accepting rows so the status bar can say so.
func TestBufferStopsAtItsCap(t *testing.T) {
	b := NewBuffer(3)
	b.SetColumns([]string{"n"}, nil)

	accepted := b.Append([][]any{{int64(1)}, {int64(2)}})
	if accepted != 2 {
		t.Errorf("Append() = %d, want 2", accepted)
	}
	if b.AtCapacity() {
		t.Error("AtCapacity() = true before the cap was reached")
	}

	accepted = b.Append([][]any{{int64(3)}, {int64(4)}, {int64(5)}})
	if accepted != 1 {
		t.Errorf("Append() = %d, want 1; only one row fits under the cap", accepted)
	}
	if got := b.RowCount(); got != 3 {
		t.Errorf("RowCount() = %d, want 3", got)
	}
	if !b.AtCapacity() {
		t.Error("AtCapacity() = false after the cap was reached")
	}
}

func TestBufferWithoutACapAcceptsEverything(t *testing.T) {
	b := NewBuffer(0)
	b.SetColumns([]string{"n"}, nil)

	rows := make([][]any, 5000)
	for i := range rows {
		rows[i] = []any{int64(i)}
	}
	if accepted := b.Append(rows); accepted != 5000 {
		t.Errorf("Append() = %d, want 5000", accepted)
	}
	if b.AtCapacity() {
		t.Error("AtCapacity() = true for an uncapped buffer")
	}
}

// Cell output goes straight into tview, so tag characters must already be
// neutralised by the time the UI sees them.
func TestCellEscapesTviewTags(t *testing.T) {
	b := NewBuffer(0)
	b.SetColumns([]string{"v"}, nil)
	b.Append([][]any{{[]byte("[red]alert")}})

	if got := b.Cell(0, 0); got == "[red]alert" {
		t.Errorf("Cell(0,0) = %q, want the tag escaped", got)
	}
}

// Raw returns the unformatted value, which export needs and display does not.
func TestRawReturnsTheOriginalValue(t *testing.T) {
	b := NewBuffer(0)
	b.SetColumns([]string{"v"}, nil)
	b.Append([][]any{{nil}, {int64(7)}})

	if got := b.Raw(0, 0); got != nil {
		t.Errorf("Raw(0,0) = %v, want nil", got)
	}
	if got := b.Raw(1, 0); got != int64(7) {
		t.Errorf("Raw(1,0) = %v, want int64(7)", got)
	}
	if got := b.Raw(9, 9); got != nil {
		t.Errorf("Raw(9,9) = %v, want nil for an out-of-range read", got)
	}
}

// The streaming goroutine appends while the UI goroutine renders; this must
// hold under the race detector.
func TestBufferIsSafeForConcurrentUse(t *testing.T) {
	b := NewBuffer(0)
	b.SetColumns([]string{"n"}, nil)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			b.Append([][]any{{int64(i)}})
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = b.RowCount()
			_ = b.Cell(i/2, 0)
			_ = b.ColumnCount()
		}
	}()

	wg.Wait()

	if got := b.RowCount(); got != 1000 {
		t.Errorf("RowCount() = %d, want 1000", got)
	}
}
