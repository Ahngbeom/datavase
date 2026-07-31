package ui

import (
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// gridContent adapts a result buffer to tview's virtual table.
//
// tview only asks for the cells it is about to draw, so a million-row result
// costs no more to render than a screenful. Crucially, GetRowCount reports
// the rows received *so far*: the grid stays scrollable while the stream is
// still filling the buffer, which is what makes a large result feel instant.
type gridContent struct {
	tview.TableContentReadOnly
	buf *result.Buffer
}

func newGridContent(buf *result.Buffer) *gridContent {
	return &gridContent{buf: buf}
}

// GetRowCount includes the header row, which occupies row zero.
func (c *gridContent) GetRowCount() int {
	n := c.buf.ColumnCount()
	if n == 0 {
		return 0
	}
	return c.buf.RowCount() + 1
}

func (c *gridContent) GetColumnCount() int {
	return c.buf.ColumnCount()
}

// GetCell never returns nil. tview asks for positions that may already be
// gone by the time the request arrives — a result cleared mid-render — and a
// nil dereference there would take down the whole application.
func (c *gridContent) GetCell(row, column int) *tview.TableCell {
	if row == 0 {
		return headerCell(c.buf.ColumnName(column))
	}
	return dataCell(c.buf.Cell(row-1, column))
}

func headerCell(name string) *tview.TableCell {
	return tview.NewTableCell(result.EscapeTags(name)).
		SetTextColor(colourAccent).
		SetAttributes(tcell.AttrBold).
		SetSelectable(false).
		SetExpansion(1)
}

func dataCell(text string) *tview.TableCell {
	cell := tview.NewTableCell(text).SetExpansion(1)

	// NULL is dimmed so it reads as an absence rather than as the literal
	// four-letter string a column might actually contain.
	if text == result.NullText {
		cell.SetTextColor(colourMuted)
	}
	return cell
}
