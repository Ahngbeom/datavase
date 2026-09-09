package ui

import (
	"strings"

	"github.com/Ahngbeom/datavase/internal/result"
)

// Copying out of the grid reads the buffer rather than the screen, for the
// same reason searching it does: the grid truncates long values and doubles
// their brackets for the markup parser. Copying that copy would paste half a
// value with its punctuation mangled.

// copyIntent is what the copy-or-cancel key should do.
type copyIntent int

const (
	// intentNothing has nothing to offer, and says so.
	intentNothing copyIntent = iota
	intentCancel
	intentSelection
	intentCell
)

// copyContext is everything the key's meaning depends on.
type copyContext struct {
	running      bool
	onGrid       bool
	hasSelection bool
}

// resolve states the precedence in one place, because it is a judgement
// rather than an implementation detail.
//
// Cancelling wins over every kind of copying while a statement is running. A
// grid always has a cell under the cursor, so without this rule the key would
// quietly stop being the way to halt a runaway statement the moment the
// results had focus — and stopping one matters more than copying from it.
//
// Below that, focus decides. An editor selection outlives the pane it was
// made in: ⌘A before running leaves one behind, and reading it first meant
// the key in the results copied the SQL instead of the cell under the cursor,
// for as long as that selection stood.
//
// The rule is one sentence: while something is running the key cancels,
// otherwise it copies from wherever the keyboard is.
func (c copyContext) resolve() copyIntent {
	if c.running {
		return intentCancel
	}
	switch {
	case c.onGrid:
		return intentCell
	case c.hasSelection:
		return intentSelection
	}
	return intentNothing
}

// cellValue is the value under the grid's selection, as the server sent it.
func cellValue(buf *result.Buffer, row, col int) (string, bool) {
	if buf == nil || row < 0 || row >= buf.RowCount() || col < 0 || col >= buf.ColumnCount() {
		return "", false
	}
	return result.Format(buf.Raw(row, col)), true
}

// rowValues is one row, tab separated.
//
// Tabs rather than anything prettier because a copied row is pasted somewhere
// that understands columns — a spreadsheet, another terminal — and alignment
// drawn with spaces stops being alignment the moment it lands there.
func rowValues(buf *result.Buffer, row int) (string, bool) {
	if buf == nil || row < 0 || row >= buf.RowCount() {
		return "", false
	}

	values := make([]string, buf.ColumnCount())
	for col := range values {
		values[col] = result.Format(buf.Raw(row, col))
	}
	return strings.Join(values, "\t"), true
}

// copyCell puts the selected value on the clipboard, reporting whether there
// was one.
func (a *App) copyCell() bool {
	row, col := a.grid.GetSelection()

	value, ok := cellValue(a.buf, a.content.bufferRow(row), col)
	if !ok {
		return false
	}

	a.setClipboard(value)
	a.notice("value copied")
	return true
}

// copyRow puts the whole row under the grid's cursor on the clipboard.
//
// It answers only for the results: the key means one row, and there is no row
// to mean anywhere else.
func (a *App) copyRow() {
	if a.app.GetFocus() != a.grid {
		a.notice("select a result row first")
		return
	}

	row, _ := a.grid.GetSelection()
	values, ok := rowValues(a.buf, a.content.bufferRow(row))
	if !ok {
		a.notice("no row selected")
		return
	}

	a.setClipboard(values)
	a.notice(plural(a.buf.ColumnCount(), "value") + " copied, tab separated")
}
