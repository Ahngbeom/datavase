package result

import (
	"database/sql"
	"sync"
)

// CellLimit caps how many runes a grid cell shows. Long values stay in the
// buffer intact; only the rendering is shortened.
const CellLimit = 200

// Buffer holds the rows of one result set.
//
// It is written by the streaming goroutine and read by the UI goroutine, so
// every accessor takes the lock. Reads outside the current bounds return
// zero values rather than panicking: tview asks for cells while rendering,
// and a result that was just cleared would otherwise crash the application.
type Buffer struct {
	mu      sync.RWMutex
	columns []string
	types   []*sql.ColumnType
	rows    [][]any
	max     int
}

// NewBuffer returns an empty buffer. A max of zero means unbounded.
func NewBuffer(max int) *Buffer {
	return &Buffer{max: max}
}

// SetColumns replaces the result header and drops any existing rows.
func (b *Buffer) SetColumns(columns []string, types []*sql.ColumnType) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.columns = columns
	b.types = types
	b.rows = nil
}

// Append adds rows up to the cap and returns how many were accepted.
func (b *Buffer) Append(rows [][]any) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.max > 0 {
		room := b.max - len(b.rows)
		if room <= 0 {
			return 0
		}
		if len(rows) > room {
			rows = rows[:room]
		}
	}

	b.rows = append(b.rows, rows...)
	return len(rows)
}

// Reset clears the header and the rows.
func (b *Buffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.columns = nil
	b.types = nil
	b.rows = nil
}

// RowCount is the number of rows received so far. While a stream is running
// it grows, which is what lets the grid scroll through partial results.
func (b *Buffer) RowCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.rows)
}

// ColumnCount is the number of columns in the result.
func (b *Buffer) ColumnCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.columns)
}

// ColumnName returns the name of a column, or "" if it is out of range.
func (b *Buffer) ColumnName(col int) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if col < 0 || col >= len(b.columns) {
		return ""
	}
	return b.columns[col]
}

// Columns returns a copy of the column names.
func (b *Buffer) Columns() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]string, len(b.columns))
	copy(out, b.columns)
	return out
}

// AtCapacity reports whether the buffer has stopped accepting rows.
func (b *Buffer) AtCapacity() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.max > 0 && len(b.rows) >= b.max
}

// Cell returns display text for a cell, ready to hand to tview.
func (b *Buffer) Cell(row, col int) string {
	v, ok := b.rawAt(row, col)
	if !ok {
		return ""
	}
	return EscapeTags(Truncate(Format(v), CellLimit))
}

// ColumnType names the column's database type — BIGINT, VARCHAR — or "" when
// the driver did not say.
//
// A driver is allowed to report nothing, and several statements produce no
// type information at all, so the caller has to be able to show less rather
// than showing a blank where a type was promised.
func (b *Buffer) ColumnType(col int) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if col < 0 || col >= len(b.types) || b.types[col] == nil {
		return ""
	}
	return b.types[col].DatabaseTypeName()
}

// Raw returns the unformatted value, which export needs and display does not.
func (b *Buffer) Raw(row, col int) any {
	v, _ := b.rawAt(row, col)
	return v
}

// Row returns a copy of one row's raw values, or nil if out of range.
func (b *Buffer) Row(row int) []any {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if row < 0 || row >= len(b.rows) {
		return nil
	}
	out := make([]any, len(b.rows[row]))
	copy(out, b.rows[row])
	return out
}

func (b *Buffer) rawAt(row, col int) (any, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if row < 0 || row >= len(b.rows) {
		return nil, false
	}
	r := b.rows[row]
	if col < 0 || col >= len(r) {
		return nil, false
	}
	return r[col], true
}
