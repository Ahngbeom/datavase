package ui

import (
	"bytes"
	"fmt"

	"github.com/Ahngbeom/datavase/internal/export"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/rivo/tview"
)

// copyFormat is how the whole result is written to the clipboard.
type copyFormat int

const (
	formatMarkdown copyFormat = iota
	formatJSON
	formatCSV
)

func (f copyFormat) String() string {
	switch f {
	case formatJSON:
		return "JSON"
	case formatCSV:
		return "CSV"
	}
	return "Markdown"
}

const pageCopyFormat = "copy-format"

// showCopyFormats asks which format, then copies.
func (a *App) showCopyFormats() {
	if a.buf.ColumnCount() == 0 {
		a.notice("no result to copy")
		return
	}

	list := tview.NewList().ShowSecondaryText(true)
	list.AddItem("Markdown", "a table, for a document or a chat", 'm', func() {
		a.pages.RemovePage(pageCopyFormat)
		a.app.SetFocus(a.grid)
		a.copyResult(formatMarkdown)
	})
	list.AddItem("JSON", "an array of objects, for a script", 'j', func() {
		a.pages.RemovePage(pageCopyFormat)
		a.app.SetFocus(a.grid)
		a.copyResult(formatJSON)
	})
	list.AddItem("CSV file", "written to disk, for a spreadsheet", 'c', func() {
		a.pages.RemovePage(pageCopyFormat)
		a.promptSavePath()
	})
	list.SetDoneFunc(func() {
		a.pages.RemovePage(pageCopyFormat)
		a.app.SetFocus(a.grid)
	})
	list.SetBorder(true).SetTitle(" the result as ")

	a.pages.AddPage(pageCopyFormat, centred(list, 44, 10), true, true)
	a.app.SetFocus(list)
}

// copyResult writes the whole buffer to the clipboard.
func (a *App) copyResult(f copyFormat) {
	text, err := resultText(a.buf, a.content, f)
	if err != nil {
		a.notice("copy failed: " + err.Error())
		return
	}
	a.setClipboard(text)
	a.notice(copySummary(a.buf.RowCount(), f, len(text), a.buf.AtCapacity()))
}

// resultText renders the buffer in the order the grid shows it.
//
// The buffer rather than the screen: the grid truncates long values and
// doubles brackets for the markup parser, and neither belongs in a paste.
// Rows go through the grid's order so that a sorted column copies sorted.
func resultText(buf *result.Buffer, content *gridContent, f copyFormat) (string, error) {
	rows := make([][]any, 0, buf.RowCount())
	for grid := 1; grid <= buf.RowCount(); grid++ {
		rows = append(rows, buf.Row(content.bufferRow(grid)))
	}

	var out bytes.Buffer
	var err error
	switch f {
	case formatJSON:
		err = export.JSON(&out, buf.Columns(), rows)
	case formatCSV:
		err = export.CSV(&out, buf.Columns(), rows)
	default:
		err = export.Markdown(&out, buf.Columns(), rows)
	}
	return out.String(), err
}

// copySummary is the notice after a copy.
//
// The size is stated because OSC 52 has a per-terminal ceiling that is
// exceeded without a word; a reader who pastes half a table can at least see
// that the whole was larger than their terminal passes on.
func copySummary(rows int, f copyFormat, size int, truncated bool) string {
	s := fmt.Sprintf("%s copied as %s (%s)", plural(rows, "row"), f, humanBytes(size))
	if truncated {
		s += " · truncated at buffer_max"
	}
	return s
}

func humanBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
