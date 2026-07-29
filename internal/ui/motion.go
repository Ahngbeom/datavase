package ui

import (
	"strings"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// Cursor movement by word and line.
//
// tview's TextArea has its own word movement, but it splits on Unicode word
// boundaries: "user_id" becomes three words. SQL identifiers have to move as
// one, so these use the tokenizer's notion of an identifier byte — the same
// one completion uses, so the two cannot disagree.
//
// The asymmetry is deliberate and matches every other editor: moving left
// seeks the *start* of a word, moving right seeks its *end*. A left-then-right
// round trip therefore spans the word rather than returning to where it began.

// wordLeft returns the offset of the previous word start.
func wordLeft(text string, offset int) int {
	at := clamp(offset, 0, len(text))

	// Step back over anything that is not part of an identifier.
	for at > 0 && !isWordAt(text, at-1) {
		at--
	}
	// Then back over the identifier itself.
	for at > 0 && isWordAt(text, at-1) {
		at--
	}
	return at
}

// wordRight returns the offset of the next word end.
func wordRight(text string, offset int) int {
	at := clamp(offset, 0, len(text))

	for at < len(text) && !isWordAt(text, at) {
		at++
	}
	for at < len(text) && isWordAt(text, at) {
		at++
	}
	return at
}

// isWordAt reports whether the byte at an offset belongs to an identifier.
//
// Continuation bytes of a multi-byte rune count as part of the word, which is
// what keeps movement from landing inside a character.
func isWordAt(text string, offset int) bool {
	if offset < 0 || offset >= len(text) {
		return false
	}
	return sqlparse.IsIdentifierByte(text[offset])
}

// lineStartAt returns the offset of the first character of the caret's line.
func lineStartAt(text string, offset int) int {
	at := clamp(offset, 0, len(text))
	return strings.LastIndexByte(text[:at], '\n') + 1
}

// lineEndAt returns the offset just past the last character of the line.
func lineEndAt(text string, offset int) int {
	at := clamp(offset, 0, len(text))

	if nl := strings.IndexByte(text[at:], '\n'); nl >= 0 {
		return at + nl
	}
	return len(text)
}

// deleteWordLeft removes the word before the caret.
func deleteWordLeft(text string, offset int) edit {
	at := clamp(offset, 0, len(text))
	return edit{start: wordLeft(text, at), end: at}
}

// deleteToLineStart removes everything from the line's start to the caret.
func deleteToLineStart(text string, offset int) edit {
	at := clamp(offset, 0, len(text))
	return edit{start: lineStartAt(text, at), end: at}
}
