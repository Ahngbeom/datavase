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
	intentDefinition
	intentSelection
	intentCell
)

// copyContext is everything the key's meaning depends on.
type copyContext struct {
	running      bool
	onDDL        bool
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
// It also makes the rule sayable in a sentence: while something is running
// the key cancels, otherwise it copies whatever has focus. It used to depend
// on which pane you were in, which is not a rule anyone could hold in mind.
func (c copyContext) resolve() copyIntent {
	if c.running {
		return intentCancel
	}
	switch {
	case c.onDDL:
		return intentDefinition
	case c.hasSelection:
		return intentSelection
	case c.onGrid:
		return intentCell
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

	// Row zero is the header, which the buffer does not count.
	value, ok := cellValue(a.buf, row-1, col)
	if !ok {
		return false
	}

	a.setClipboard(value)
	a.notice("value copied")
	return true
}

// copyRow puts the whole selected row on the clipboard.
func (a *App) copyRow() {
	row, _ := a.grid.GetSelection()

	values, ok := rowValues(a.buf, row-1)
	if !ok {
		a.notice("no row selected")
		return
	}

	a.setClipboard(values)
	a.notice("row copied")
}
