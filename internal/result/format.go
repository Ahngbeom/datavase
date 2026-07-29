// Package result formats query values for display and export.
//
// Formatting lives apart from the UI so the grid, CSV export and JSON export
// all agree on what a value looks like — and so the rules can be tested
// without a terminal.
package result

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// NullText is how a SQL NULL is shown. It has to be visually distinct from
// an empty string: treating the two alike misleads people into thinking a
// row failed an IS NULL test for no reason.
const NullText = "NULL"

// TimeLayout is the timestamp rendering, chosen to match MySQL's own.
const TimeLayout = "2006-01-02 15:04:05"

// Format renders a scanned value as display text.
func Format(v any) string {
	switch value := v.(type) {
	case nil:
		return NullText
	case []byte:
		return formatBytes(value)
	case string:
		return escapeControl(value)
	case bool:
		// MySQL has no boolean type; TINYINT(1) reads back as 0 or 1, and
		// showing "true" here would not match what a query returns.
		if value {
			return "1"
		}
		return "0"
	case int64:
		return strconv.FormatInt(value, 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case time.Time:
		return value.Format(TimeLayout)
	default:
		return escapeControl(fmt.Sprint(v))
	}
}

// formatBytes renders text as text and anything else as hex, so a BLOB
// cannot scramble the terminal.
func formatBytes(b []byte) string {
	if !utf8.Valid(b) {
		return "0x" + hex.EncodeToString(b)
	}
	s := string(b)
	if hasBinaryControl(s) {
		return "0x" + hex.EncodeToString(b)
	}
	return escapeControl(s)
}

// hasBinaryControl reports control characters that are not the whitespace
// escapeControl already handles.
func hasBinaryControl(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

// escapeControl makes newlines and tabs visible instead of letting them
// break the grid's row alignment.
var controlEscaper = strings.NewReplacer(
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
)

func escapeControl(s string) string {
	return controlEscaper.Replace(s)
}

// EscapeTags neutralises tview's colour-tag syntax.
//
// tview reads "[" as the start of a tag, so a cell containing "[red]" would
// recolour the grid instead of showing its own text. tview's documented
// escape is to write "[" as "[[".
func EscapeTags(s string) string {
	if !strings.ContainsRune(s, '[') {
		return s
	}
	return strings.ReplaceAll(s, "[", "[[")
}

// Truncate shortens s to limit runes, marking the cut with an ellipsis.
// A limit of zero or less leaves s unchanged.
func Truncate(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}

	// Counting runes rather than bytes keeps multi-byte text from being cut
	// mid-character, which would render as a replacement glyph.
	var (
		b     strings.Builder
		count int
	)
	for _, r := range s {
		if count >= limit-1 {
			break
		}
		b.WriteRune(r)
		count++
	}
	b.WriteRune('…')
	return b.String()
}
