package ui

import (
	"strings"
	"unicode/utf8"
)

// Motions that only vim needs.
//
// The DataGrip keyboard's word movement lives in motion.go and is shared; the
// functions here exist because vim's are subtly different in ways that matter
// once an operator is applied to them. "w" reaching the start of the next
// word rather than the end of this one is the whole reason "dw" removes the
// space after a word — which is what makes it useful.

// wordStartRight returns the offset of the next word's start.
//
// It stops at the end of the line. In vim "w" does cross lines, but "dw" on
// the last word of a line does not, and joining two lines is the more
// surprising of the two behaviours to get by accident.
func wordStartRight(text string, offset int) int {
	at := clamp(offset, 0, len(text))

	for at < len(text) && isWordAt(text, at) {
		at++
	}
	for at < len(text) && !isWordAt(text, at) && text[at] != '\n' {
		at++
	}
	return at
}

// firstNonBlankAt returns the offset of the first non-blank on the caret's
// line, or the line's end when it is blank.
func firstNonBlankAt(text string, offset int) int {
	start := lineStartAt(text, offset)
	end := lineEndAt(text, offset)

	at := start
	for at < end && (text[at] == ' ' || text[at] == '\t') {
		at++
	}
	return at
}

// charLeft returns the offset one character back, staying on the line.
func charLeft(text string, offset int) int {
	at := clamp(offset, 0, len(text))
	if start := lineStartAt(text, at); at <= start {
		return start
	}

	_, size := utf8.DecodeLastRuneInString(text[:at])
	return at - size
}

// charRight returns the offset one character on, staying on the line.
func charRight(text string, offset int) int {
	at := clamp(offset, 0, len(text))
	if end := lineEndAt(text, at); at >= end {
		return end
	}

	_, size := utf8.DecodeRuneInString(text[at:])
	return at + size
}

// lineDown returns the offset one line down, keeping the column.
func lineDown(text string, offset int) int {
	at := clamp(offset, 0, len(text))

	end := lineEndAt(text, at)
	if end >= len(text) {
		return at
	}
	return atColumn(text, end+1, at-lineStartAt(text, at))
}

// lineUp returns the offset one line up, keeping the column.
func lineUp(text string, offset int) int {
	at := clamp(offset, 0, len(text))

	start := lineStartAt(text, at)
	if start == 0 {
		return at
	}
	return atColumn(text, lineStartAt(text, start-1), at-start)
}

// atColumn returns the offset that many bytes into the line beginning at
// start, clamped to the line's end.
//
// Columns are counted in bytes to match every other offset in the editor. A
// line of CJK text moved through vertically therefore lands on a rune
// boundary only by luck, so the result is snapped back onto one.
func atColumn(text string, start, column int) int {
	end := lineEndAt(text, start)

	at := clamp(start+column, start, end)
	for at > start && at < len(text) && !utf8.RuneStart(text[at]) {
		at--
	}
	return at
}

// vimIndent returns the leading whitespace of the caret's line, which "o" and
// "O" carry onto the line they open.
func vimIndent(text string, offset int) string {
	start := lineStartAt(text, offset)
	return text[start:firstNonBlankAt(text, offset)]
}

// trimTrailingNewline drops one trailing newline, used when a linewise yank
// is put back charwise.
func trimTrailingNewline(s string) string {
	return strings.TrimSuffix(s, "\n")
}
