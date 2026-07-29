package ui

import "strings"

// offsetAt converts the editor's (row, column) caret into a byte offset.
//
// tview reports the column as a rune index while sqlparse works in bytes, so
// the two disagree on every line containing multi-byte text. Out-of-range
// positions are clamped rather than rejected: the caret can legitimately sit
// past the end of the text while an edit is being applied, and a panic there
// would take down the application.
func offsetAt(text string, row, column int) int {
	if row < 0 {
		row = 0
	}
	if column < 0 {
		column = 0
	}

	offset := 0
	for r := 0; r < row; r++ {
		nl := strings.IndexByte(text[offset:], '\n')
		if nl < 0 {
			// Fewer lines than requested; the caret is at the end.
			return len(text)
		}
		offset += nl + 1
	}

	line := text[offset:]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}

	// Walk runes so the column index lands on a character boundary.
	count := 0
	for i := range line {
		if count == column {
			return offset + i
		}
		count++
	}
	return offset + len(line)
}
