package ui

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The row view is what `\G` is for in the mysql client: one row read down the
// page instead of across it.
//
// A grid is the wrong shape for a wide table. Forty columns share the
// terminal's width between them, so each gets a few characters, and the value
// you came to read was cut at CellLimit besides. Turning the row on its side
// gives every value the whole width and the space to be shown in full.

// rowDetail renders one row of a result as name-and-value lines.
//
// It is a plain function over the buffer so that what the user is shown can be
// tested without a terminal, in the same way the status bar is.
func rowDetail(buf *result.Buffer, row int) string {
	if buf == nil || row < 0 || row >= buf.RowCount() {
		return ""
	}

	columns := buf.Columns()
	width := 0
	for _, name := range columns {
		if n := len([]rune(name)); n > width {
			width = n
		}
	}

	var b strings.Builder
	for col, name := range columns {
		// Deliberately not buf.Cell: that truncates at CellLimit and is
		// exactly what this view exists to get past.
		value := result.Format(buf.Raw(row, col))

		b.WriteString(tag(colourAccent, pad(result.EscapeTags(name), width)))
		b.WriteString("  ")
		if value == result.NullText {
			// Dimmed for the same reason as in the grid: so an absent value
			// reads as an absence rather than as the four-letter string a
			// column might really hold.
			b.WriteString(tag(colourMuted, value))
		} else {
			b.WriteString(result.EscapeTags(value))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// pad right-fills a name to width so the values line up in a second column.
func pad(s string, width int) string {
	if n := len([]rune(s)); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// showRow opens the row under the grid's selection.
//
// It is an overlay rather than a third tab because it answers a question about
// one row — "what is actually in this" — and then gets out of the way. A tab
// would be a place you have to remember to leave.
func (a *App) showRow() {
	row, _ := a.grid.GetSelection()
	// Row zero is the header, and the buffer does not count it.
	index := row - 1

	detail := rowDetail(a.buf, index)
	if detail == "" {
		a.notice("no row selected")
		return
	}

	view := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetText(detail)
	view.SetBorder(true).
		SetTitle(fmt.Sprintf(" row %d of %d ", index+1, a.buf.RowCount())).
		SetTitleAlign(tview.AlignLeft)
	view.SetBackgroundColor(tcell.ColorBlack)

	view.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape, ev.Rune() == 'q':
			a.closeDialog()
			return nil

		// Stepping between rows without leaving: comparing two rows is most
		// of what this view is opened for, and closing and reopening for each
		// one loses the place in the grid.
		case ev.Rune() == 'j', ev.Key() == tcell.KeyRight:
			a.stepRow(1)
			return nil
		case ev.Rune() == 'k', ev.Key() == tcell.KeyLeft:
			a.stepRow(-1)
			return nil
		}
		return ev
	})

	a.openDialog(centred(view, 90, 30))
}

// stepRow moves the grid's selection and redraws the open row view, so the
// two cannot disagree about which row is being read.
func (a *App) stepRow(delta int) {
	row, col := a.grid.GetSelection()
	next := row + delta
	if next < 1 || next > a.buf.RowCount() {
		return
	}

	a.grid.Select(next, col)
	a.closeDialog()
	a.showRow()
}
