package ui

import "strings"

// commentMarker is SQL's line comment. The trailing space is conventional and
// is what gets written; recognition accepts the marker without it.
const commentMarker = "--"

// edit is a replacement for a byte range of the buffer.
//
// Edits are expressed as ranges rather than as a whole new buffer because
// that is what TextArea.Replace consumes — and going through Replace is what
// keeps each of these operations a single step for Ctrl+Z. Rebuilding the
// buffer with SetText would work visually but silently discard the undo
// history, which is worse than not having the feature.
type edit struct {
	start, end int
	text       string
}

// lineSpan returns the byte range of the whole lines covering [from, to].
//
// Positions outside the buffer are clamped: the caret can legitimately be
// reported past the end while an edit is in flight, and a panic there costs
// the user everything they had typed.
func lineSpan(text string, from, to int) (int, int) {
	if from > to {
		from, to = to, from
	}
	from = clamp(from, 0, len(text))
	to = clamp(to, 0, len(text))

	start := strings.LastIndexByte(text[:from], '\n') + 1

	end := to
	if nl := strings.IndexByte(text[to:], '\n'); nl >= 0 {
		end = to + nl
	} else {
		end = len(text)
	}
	return start, end
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// toggleComment comments or uncomments the lines covering [from, to].
//
// The rule matches VS Code and DataGrip: if every non-blank line in the range
// is already commented, uncomment; otherwise comment all of them. Deciding
// per line instead would let markers stack up on lines that were already
// commented.
func toggleComment(text string, from, to int) edit {
	start, end := lineSpan(text, from, to)
	block := text[start:end]
	lines := strings.Split(block, "\n")

	if allCommented(lines) {
		return edit{start: start, end: end, text: uncommentLines(lines)}
	}
	return edit{start: start, end: end, text: commentLines(lines)}
}

// allCommented reports whether every line that has content is a comment.
// Blank lines do not count either way; a selection of only blank lines is
// treated as uncommented so the toggle adds markers.
func allCommented(lines []string) bool {
	sawContent := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		sawContent = true
		if !strings.HasPrefix(trimmed, commentMarker) {
			return false
		}
	}
	return sawContent
}

// commentLines inserts the marker after each line's indentation, so the code
// keeps its shape.
func commentLines(lines []string) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			// Commenting blank lines adds noise and breaks the round trip.
			out[i] = line
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		out[i] = indent + commentMarker + " " + line[len(indent):]
	}
	return strings.Join(out, "\n")
}

// uncommentLines removes one marker per line, along with the single space
// commentLines writes after it.
func uncommentLines(lines []string) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, commentMarker) {
			out[i] = line
			continue
		}

		indent := line[:len(line)-len(trimmed)]
		rest := trimmed[len(commentMarker):]
		// Remove exactly the one space commentLines adds, so a comment that
		// was written by hand as "--   note" keeps its own spacing.
		rest = strings.TrimPrefix(rest, " ")
		out[i] = indent + rest
	}
	return strings.Join(out, "\n")
}

// duplicateLines copies the lines covering [from, to] below themselves.
func duplicateLines(text string, from, to int) edit {
	start, end := lineSpan(text, from, to)
	block := text[start:end]

	return edit{start: start, end: end, text: block + "\n" + block}
}

// deleteLines removes the lines covering [from, to].
//
// The trailing newline goes with the block, except on the last line, where
// the preceding newline is taken instead — otherwise deleting the final line
// would leave a blank one behind.
func deleteLines(text string, from, to int) edit {
	start, end := lineSpan(text, from, to)

	switch {
	case end < len(text):
		// Not the last line: take the newline that follows.
		end++
	case start > 0:
		// Last line: take the newline that precedes.
		start--
	}
	return edit{start: start, end: end}
}
