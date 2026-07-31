// Package match answers "does what the user typed match this?" for the two
// places that ask.
//
// The finder dialogs rank a list and want a forgiving answer: initials should
// reach a command nobody spelled out. Text search walks a buffer and wants an
// exact one: a caret that lands somewhere the pattern is not is worse than no
// jump at all. Both live here so the two never drift into disagreeing about
// what case-insensitive means.
//
// It knows nothing about screens, files or SQL — the inputs are strings and
// the outputs are numbers.
package match

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Scoring weights. Only their order matters, not their absolute size.
const (
	// runeMatched is what any matched rune is worth on its own.
	runeMatched = 1
	// runAdjacent rewards a rune that carries on from the previous match, so
	// "abc" prefers "abcdef" to "axbxcx".
	runAdjacent = 2
	// wordStart rewards a rune that opens a word, so "dl" prefers
	// "delete line" to "dreadfully".
	wordStart = 3
)

// lengthTiebreak scales the score so a candidate's length can only separate
// two equally good matches, never outrank a better one. Without it "run all"
// and "run" are indistinguishable and Enter runs whichever happened to sort
// first.
const lengthTiebreak = 1000

// unreachable marks a placement no run of matches can arrive at.
const unreachable = -1 << 30

// Fuzzy scores candidate against term, matching term's runes in order but not
// necessarily together.
//
// An empty term matches everything with no opinion about order, which is what
// a freshly opened dialog wants. Case is always ignored: a finder is a filter,
// and a capital typed into one is a typo more often than an intention.
//
// It finds the best-scoring placement, not the leftmost one. Taking the first
// occurrence of each rune is cheaper but ranks "dreadfully" above
// "delete line" for "dl", because the leading "l" of "line" is never reached —
// the "l" inside "delete" is claimed first.
func Fuzzy(term, candidate string) (score int, ok bool) {
	if term == "" {
		return 0, true
	}

	want := []rune(strings.ToLower(term))
	have := []rune(strings.ToLower(candidate))
	if len(want) > len(have) {
		return 0, false
	}

	// best[j] is the best score for placing the term rune under consideration
	// at candidate rune j, having placed every earlier one before it.
	best := make([]int, len(have))
	previous := make([]int, len(have))

	for i := range want {
		// carried is the best score any earlier placement of the previous term
		// rune reached, over every position left of j. Tracking it as j
		// advances is what keeps this linear.
		carried := unreachable

		for j := range have {
			if i > 0 {
				if j > 0 && previous[j-1] > carried {
					carried = previous[j-1]
				}
			}

			if have[j] != want[i] {
				best[j] = unreachable
				continue
			}

			gain := runeMatched
			if opensWord(have, j) {
				gain += wordStart
			}

			switch {
			case i == 0:
				// The first rune of the term may start anywhere.
				best[j] = gain
			case carried == unreachable:
				best[j] = unreachable
			default:
				from := carried
				// Carrying straight on from the rune next door is worth more
				// than reaching the same place across a gap.
				if j > 0 && previous[j-1] != unreachable && previous[j-1]+runAdjacent > from {
					from = previous[j-1] + runAdjacent
				}
				best[j] = from + gain
			}
		}
		best, previous = previous, best
	}

	raw := unreachable
	for _, s := range previous {
		if s > raw {
			raw = s
		}
	}
	if raw == unreachable {
		return 0, false
	}

	return raw*lengthTiebreak - min(len(candidate), lengthTiebreak-1), true
}

// opensWord reports whether the rune at i starts a word.
//
// Separators only, not capitals: the things being matched are command names,
// schema names and paths, where the break is a space, a slash or an
// underscore.
func opensWord(runes []rune, i int) bool {
	if i == 0 {
		return true
	}
	switch runes[i-1] {
	case ' ', '\t', '/', '\\', '_', '-', '.', ':':
		return true
	}
	return false
}

// Next finds pattern at or after the byte offset from.
//
// The offset is inclusive so that repeating a search from a match that is
// still under the caret does not require the caller to nudge past it first.
// A miss is a miss, not a wrap: whether searching should start over from the
// top is the caller's decision, and the operator that would delete everything
// in between must be able to say no.
func Next(text, pattern string, from int) (offset int, ok bool) {
	if pattern == "" {
		return 0, false
	}
	if from < 0 {
		from = 0
	}

	fold := ignoresCase(pattern)
	for i := from; i < len(text); {
		if matchesAt(text[i:], pattern, fold) {
			return i, true
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}
	return 0, false
}

// Contains reports whether pattern appears anywhere in text, under the same
// smartcase rule as Next.
func Contains(text, pattern string) bool {
	_, ok := Next(text, pattern, 0)
	return ok
}

// Prev finds the last match strictly before the byte offset before.
func Prev(text, pattern string, before int) (offset int, ok bool) {
	if pattern == "" {
		return 0, false
	}
	if before > len(text) {
		before = len(text)
	}

	fold := ignoresCase(pattern)
	last, found := 0, false
	for i := 0; i < before; {
		if matchesAt(text[i:], pattern, fold) {
			last, found = i, true
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}
	return last, found
}

// ignoresCase implements smartcase: a pattern typed in lower case matches
// anything, and one capital in it means the user meant that capital.
func ignoresCase(pattern string) bool {
	return strings.ToLower(pattern) == pattern
}

// matchesAt compares rune by rune rather than lower-casing and comparing
// bytes, because folding can change a string's length and every offset this
// package returns is a byte offset into the original text.
func matchesAt(text, pattern string, fold bool) bool {
	for _, want := range pattern {
		have, size := utf8.DecodeRuneInString(text)
		if size == 0 {
			return false
		}
		if have != want && !(fold && unicode.ToLower(have) == unicode.ToLower(want)) {
			return false
		}
		text = text[size:]
	}
	return true
}
