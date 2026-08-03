package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/vim"
)

// span names the text an object covers, which is far easier to read in a
// failure than a pair of offsets.
func span(t *testing.T, text string, caret int, o vim.Object, around bool) (string, bool) {
	t.Helper()

	start, end, ok := objectSpan(text, caret, o, around)
	if !ok {
		return "", false
	}
	return text[start:end], true
}

func TestAWordObjectIsTheIdentifierUnderTheCaret(t *testing.T) {
	// user_id is one identifier, not three — the same rule the word motions
	// follow, and the reason they exist here rather than being tview's.
	const text = "SELECT user_id FROM t"

	if got, _ := span(t, text, 9, vim.ObjectWord, false); got != "user_id" {
		t.Errorf("iw = %q, want %q", got, "user_id")
	}
	if got, _ := span(t, text, 7, vim.ObjectWord, true); got != "user_id " {
		t.Errorf("aw = %q, want the trailing space with it", got)
	}
}

func TestAWordObjectFindsNothingOffAWord(t *testing.T) {
	if _, ok := span(t, "a  b", 1, vim.ObjectWord, false); ok {
		t.Error("iw found a word in whitespace; leaving the caret alone is safer")
	}
}

// ci( over an IN list is the sequence this feature exists for.
func TestABracketObjectIsWhatThePairEncloses(t *testing.T) {
	const text = "WHERE id IN (1, 2, 3) AND x = 1"

	if got, _ := span(t, text, 15, vim.ObjectParen, false); got != "1, 2, 3" {
		t.Errorf("i( = %q, want %q", got, "1, 2, 3")
	}
	if got, _ := span(t, text, 15, vim.ObjectParen, true); got != "(1, 2, 3)" {
		t.Errorf("a( = %q, want the brackets with it", got)
	}
}

// A nested list has to find the pair the caret is actually inside, not the
// first bracket the search happens to meet.
func TestABracketObjectPicksThePairTheCaretIsIn(t *testing.T) {
	const text = "f(a, g(b, c), d)"

	if got, _ := span(t, text, 10, vim.ObjectParen, false); got != "b, c" {
		t.Errorf("inner i( = %q, want %q", got, "b, c")
	}
	if got, _ := span(t, text, 3, vim.ObjectParen, false); got != "a, g(b, c), d" {
		t.Errorf("outer i( = %q, want the whole argument list", got)
	}

	// Past a pair that has already closed. This is the case the backward
	// depth counting exists for: without it the search meets g's opening
	// bracket first and reports the wrong region entirely.
	if got, _ := span(t, text, 14, vim.ObjectParen, false); got != "a, g(b, c), d" {
		t.Errorf("i( after a closed nested pair = %q, want the outer list", got)
	}
}

func TestABracketObjectFindsNothingOutsideAnyPair(t *testing.T) {
	if _, ok := span(t, "SELECT 1", 3, vim.ObjectParen, false); ok {
		t.Error("i( found a pair where there is none")
	}
}

// ci' replaces a string literal, which is the other half of why this exists.
func TestAQuoteObjectIsTheStringTheCaretIsIn(t *testing.T) {
	const text = "WHERE name = 'ada' AND x = 'bob'"

	if got, _ := span(t, text, 15, vim.ObjectSingleQuote, false); got != "ada" {
		t.Errorf("i' = %q, want %q", got, "ada")
	}
	if got, _ := span(t, text, 15, vim.ObjectSingleQuote, true); got != "'ada'" {
		t.Errorf("a' = %q, want the quotes with it", got)
	}
	// The second literal, to prove the pairing counts rather than searching
	// outwards — both delimiters are the same character, so searching cannot
	// tell an opening quote from a closing one.
	if got, _ := span(t, text, 28, vim.ObjectSingleQuote, false); got != "bob" {
		t.Errorf("i' on the second literal = %q, want %q", got, "bob")
	}
}

// Between two literals is outside both, and taking the run between them would
// delete the operator sitting there.
func TestAQuoteObjectFindsNothingBetweenTwoLiterals(t *testing.T) {
	const text = "'a' + 'b'"

	if got, ok := span(t, text, 4, vim.ObjectSingleQuote, false); ok {
		t.Errorf("i' between two literals took %q, want nothing", got)
	}
}

func TestAQuoteObjectStaysOnItsOwnLine(t *testing.T) {
	const text = "SELECT 'a'\nFROM t"

	if _, ok := span(t, text, 14, vim.ObjectSingleQuote, false); ok {
		t.Error("i' reached a quote on the line above")
	}
}
