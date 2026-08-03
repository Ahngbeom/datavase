package ui

import "github.com/Ahngbeom/datavase/internal/vim"

// Text objects reach a region the caret is inside rather than one a motion
// travels to, which is why they are what a SQL editor is used for: the thing
// worth changing is almost always between a pair of brackets or quotes.

// objectPair is the delimiters an object is bounded by, empty for a word.
func objectPair(o vim.Object) (open, close byte, ok bool) {
	switch o {
	case vim.ObjectParen:
		return '(', ')', true
	case vim.ObjectBracket:
		return '[', ']', true
	case vim.ObjectBrace:
		return '{', '}', true
	case vim.ObjectAngle:
		return '<', '>', true
	case vim.ObjectSingleQuote:
		return '\'', '\'', true
	case vim.ObjectDoubleQuote:
		return '"', '"', true
	case vim.ObjectBacktick:
		return '`', '`', true
	}
	return 0, 0, false
}

// objectSpan is the range a text object covers, and whether there was one.
//
// Not finding the object leaves the caret alone rather than guessing at a
// region: an operator that deleted something arbitrary because the caret was
// outside every bracket would be worse than one that did nothing.
func objectSpan(text string, caret int, o vim.Object, around bool) (start, end int, ok bool) {
	if o == vim.ObjectWord {
		return wordSpan(text, caret, around)
	}

	open, close, isPair := objectPair(o)
	if !isPair {
		return 0, 0, false
	}
	if open == close {
		return quoteSpan(text, caret, open, around)
	}
	return bracketSpan(text, caret, open, close, around)
}

// wordSpan is the identifier under the caret. Around takes the whitespace
// after it as well, which is what makes "daw" leave a tidy line.
func wordSpan(text string, caret int, around bool) (int, int, bool) {
	caret = clamp(caret, 0, len(text))
	if !isWordAt(text, caret) {
		// vim would take the run of whitespace instead; leaving the caret
		// alone is the conservative reading and never deletes a neighbour.
		return 0, 0, false
	}

	start := caret
	for start > 0 && isWordAt(text, start-1) {
		start--
	}
	end := caret
	for end < len(text) && isWordAt(text, end) {
		end++
	}

	// "a" takes the whitespace after the word. vim falls back to the
	// whitespace *before* it when there is none after — "daw" on the "a" of
	// "a, b" — and that fallback is not here: it would delete the space
	// separating the word from whatever precedes it, and in a column list the
	// key for that job is "df," which already does it exactly.
	if around {
		for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
			end++
		}
	}
	return start, end, true
}

// bracketSpan is the pair enclosing the caret.
//
// The search counts depth outwards in both directions, so a nested list finds
// the pair it is actually inside rather than the first bracket it meets.
func bracketSpan(text string, caret int, open, close byte, around bool) (int, int, bool) {
	caret = clamp(caret, 0, len(text))

	start := -1
	for i, depth := caret, 0; i >= 0; i-- {
		if i < len(text) && text[i] == close && i != caret {
			depth++
			continue
		}
		if i < len(text) && text[i] == open {
			if depth == 0 {
				start = i
				break
			}
			depth--
		}
	}
	if start < 0 {
		return 0, 0, false
	}

	end := -1
	for i, depth := start+1, 0; i < len(text); i++ {
		switch text[i] {
		case open:
			depth++
		case close:
			if depth == 0 {
				end = i
			}
			depth--
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return 0, 0, false
	}

	if around {
		return start, end + 1, true
	}
	return start + 1, end, true
}

// quoteSpan is the quoted run the caret is in.
//
// Quotes are counted from the start of the line rather than searched
// outwards: the two delimiters are the same character, so which one opens a
// string is only knowable by counting from somewhere that is definitely
// outside one.
func quoteSpan(text string, caret int, quote byte, around bool) (int, int, bool) {
	caret = clamp(caret, 0, len(text))
	lineStart := lineStartAt(text, caret)
	line := text[lineStart:lineEndAt(text, caret)]

	var opens []int
	for i := 0; i < len(line); i++ {
		if line[i] == quote {
			opens = append(opens, i)
		}
	}

	for i := 0; i+1 < len(opens); i += 2 {
		start, end := lineStart+opens[i], lineStart+opens[i+1]
		if caret < start || caret > end {
			continue
		}
		if around {
			return start, end + 1, true
		}
		return start + 1, end, true
	}
	return 0, 0, false
}
