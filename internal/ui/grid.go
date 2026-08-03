package ui

import (
	"fmt"
	"sort"

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

	// sortCol is the column the rows are ordered by, or -1 for the order the
	// server sent — which is the answer to any ORDER BY that was in the
	// statement, and so worth being able to get back to.
	sortCol  int
	sortDesc bool

	// order maps a display row to its buffer row, and is nil while unsorted:
	// the common case costs nothing and every reader falls back to identity.
	order []int
	// builtFor is the row count order was built against. A result can be
	// sorted while it is still arriving, so an order built over the rows
	// received so far stops being one the moment more land.
	builtFor int
}

func newGridContent(buf *result.Buffer) *gridContent {
	return &gridContent{buf: buf, sortCol: -1}
}

// sortBy cycles a column: ascending, then descending, then back to the order
// the rows arrived in.
//
// The third state is the one grids usually leave out. Without it a statement's
// own ORDER BY is gone as soon as anyone glances at another column.
func (c *gridContent) sortBy(col int) {
	switch {
	case col < 0 || col >= c.buf.ColumnCount():
		return
	case c.sortCol != col:
		c.sortCol, c.sortDesc = col, false
	case !c.sortDesc:
		c.sortDesc = true
	default:
		c.sortCol, c.sortDesc = -1, false
	}
	c.order = nil
}

// unsort drops the ordering, for a result that has nothing to do with the one
// the column belonged to.
func (c *gridContent) unsort() {
	c.sortCol, c.sortDesc, c.order = -1, false, nil
}

func (c *gridContent) sorted() bool { return c.sortCol >= 0 }

// bufferRow turns a grid row into the buffer row it draws.
//
// This is the only place the two are allowed to differ. Copying a row,
// opening it, searching to it and saying which of how many it is all read the
// grid's number, and every one of them was the buffer's number until a column
// could be sorted.
func (c *gridContent) bufferRow(gridRow int) int {
	c.reorder()

	at := gridRow - 1
	if c.order == nil || at < 0 || at >= len(c.order) {
		return at
	}
	return c.order[at]
}

// gridRow is bufferRow's inverse: where a buffer row ended up on screen.
//
// It scans, because the reverse mapping is wanted once per sort rather than
// once per drawn cell, and a second slice to keep in step is a second thing
// that can fall out of step.
func (c *gridContent) gridRow(bufferRow int) int {
	c.reorder()

	if c.order != nil {
		for display, at := range c.order {
			if at == bufferRow {
				return display + 1
			}
		}
	}
	return bufferRow + 1
}

// reorder rebuilds the mapping when it no longer covers the buffer.
func (c *gridContent) reorder() {
	rows := c.buf.RowCount()
	if c.sortCol < 0 || c.sortCol >= c.buf.ColumnCount() {
		c.order, c.builtFor = nil, rows
		return
	}
	if c.order != nil && c.builtFor == rows {
		return
	}

	// The keys are read out once rather than inside the comparison. Buffer
	// takes its read lock per call, and sorting reads a value n·log n times
	// while the goroutine still filling the buffer waits behind every one.
	keys := make([]any, rows)
	for i := range keys {
		keys[i] = c.buf.Raw(i, c.sortCol)
	}

	order := make([]int, rows)
	for i := range order {
		order[i] = i
	}

	by := result.OrderFor(c.buf.ColumnType(c.sortCol))
	// Stable, so that rows equal on this column keep the order the server
	// sent them in rather than shuffling on every rebuild.
	sort.SliceStable(order, func(i, j int) bool {
		cmp := result.Compare(keys[order[i]], keys[order[j]], by)
		if c.sortDesc {
			return cmp > 0
		}
		return cmp < 0
	})

	c.order, c.builtFor = order, rows
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
		return headerCell(c.buf.ColumnName(column) + c.sortMarker(column))
	}
	return dataCell(c.buf.Cell(c.bufferRow(row), column))
}

// sortMarker says which column the order came from, and which way.
//
// Without it a sorted grid and an ORDER BY in the statement look identical,
// and the difference decides whether what is on screen is the server's answer
// or this application's rearrangement of it.
func (c *gridContent) sortMarker(column int) string {
	// The unsorted column is -1, and so is the column tview asks about when a
	// position has already gone. Comparing them alone would mark that cell.
	if !c.sorted() || column != c.sortCol {
		return ""
	}
	if c.sortDesc {
		return " ↓"
	}
	return " ↑"
}

// sortColumn orders the results by the column the selection is in.
//
// The selection follows the row it was on rather than staying at its screen
// position: the row under the cursor is the one being looked at, and having it
// replaced by whatever happens to land there is how a sort loses someone's
// place.
func (a *App) sortColumn() {
	if a.buf.ColumnCount() == 0 {
		a.notice("no results to sort")
		return
	}

	row, col := a.grid.GetSelection()
	was := a.content.bufferRow(row)

	a.content.sortBy(col)
	a.grid.Select(a.content.gridRow(was), col)
	a.notice(sortNotice(a.sortStateNow(col)))
}

// sortState is what the notice has to describe.
type sortState struct {
	column     string
	descending bool
	rows       int
	// atCapacity and arriving are the two ways the rows in hand are not the
	// query's answer.
	atCapacity bool
	arriving   bool
}

// sortNotice says what was sorted, and admits it when that was not the result.
//
// Sorting the rows in hand is not sorting the query's answer, and on screen the
// two are identical. A result cut at capacity is missing exactly the rows that
// might have sorted to the top; a result still arriving has not been told what
// they are yet. Saying "sorted" flat in either case is the quiet kind of wrong
// this refuses elsewhere.
//
// It takes a struct rather than reading the application so that all three
// answers can be checked without a server that can be made to truncate.
func sortNotice(s sortState) string {
	if s.column == "" {
		return "back to the order the rows arrived in"
	}

	direction := "ascending"
	if s.descending {
		direction = "descending"
	}

	switch {
	case s.atCapacity:
		return fmt.Sprintf("sorted the %d rows kept by %s, %s — the result was cut, so this is not the whole ordering",
			s.rows, s.column, direction)
	case s.arriving:
		return fmt.Sprintf("sorted the %d rows so far by %s, %s — more are still arriving",
			s.rows, s.column, direction)
	}
	return fmt.Sprintf("sorted by %s, %s", s.column, direction)
}

// sortStateNow reads the state the notice describes.
func (a *App) sortStateNow(col int) sortState {
	s := sortState{
		rows:       a.buf.RowCount(),
		atCapacity: a.buf.AtCapacity(),
		arriving:   a.running != nil,
	}
	if a.content.sorted() {
		s.column, s.descending = a.buf.ColumnName(col), a.content.sortDesc
	}
	return s
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
