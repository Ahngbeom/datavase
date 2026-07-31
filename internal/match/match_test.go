package match

import (
	"strings"
	"testing"
)

// A term the user typed has to reach the thing they meant even when they typed
// only its initials, which is the whole reason a finder beats a menu.
func TestInitialsReachACommandTheUserDidNotSpellOut(t *testing.T) {
	for _, tc := range []struct {
		term, candidate string
		want            bool
	}{
		{"gt", "go to table", true},
		{"dl", "delete line", true},
		{"quit", "quit", true},
		{"", "anything at all", true},
		{"zzz", "go to table", false},
		{"tog", "go to table", false}, // the runes are there but not in order
	} {
		if _, ok := Fuzzy(tc.term, tc.candidate); ok != tc.want {
			t.Errorf("Fuzzy(%q, %q) matched = %v, want %v", tc.term, tc.candidate, ok, tc.want)
		}
	}
}

// Ranking is what makes Enter usable: the search box runs the first row, so a
// worse match sorting above a better one runs the wrong command.
func TestABetterMatchOutranksAWorseOne(t *testing.T) {
	for _, tc := range []struct {
		name          string
		term          string
		better, worse string
	}{
		{"a name that starts with the term beats one that merely contains it",
			"hi", "history", "this is it"},
		{"letters typed together beat letters scattered through the name",
			"abc", "abcdef", "axbxcxd"},
		{"initials landing on word starts beat letters caught mid-word",
			"dl", "delete line", "dreadfully"},
		{"the shorter of two equally good matches wins",
			"run", "run", "run all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			better, ok := Fuzzy(tc.term, tc.better)
			if !ok {
				t.Fatalf("Fuzzy(%q, %q) did not match at all", tc.term, tc.better)
			}
			worse, ok := Fuzzy(tc.term, tc.worse)
			if !ok {
				t.Fatalf("Fuzzy(%q, %q) did not match at all", tc.term, tc.worse)
			}
			if better <= worse {
				t.Errorf("Fuzzy(%q, %q) = %d, want more than Fuzzy(%q, %q) = %d",
					tc.term, tc.better, better, tc.term, tc.worse, worse)
			}
		})
	}
}

func TestTheFinderIgnoresCase(t *testing.T) {
	if _, ok := Fuzzy("GT", "go to table"); !ok {
		t.Error("an upper-case term did not match a lower-case candidate")
	}
	if _, ok := Fuzzy("gt", "GO TO TABLE"); !ok {
		t.Error("a lower-case term did not match an upper-case candidate")
	}
}

// Smartcase is the difference between "/id" finding every id in the file and
// "/ID" finding the one column that is actually spelled that way.
func TestAnUpperCaseLetterMakesTheSearchCaseSensitive(t *testing.T) {
	const text = "select id, ID_CARD from t"

	if got, ok := Next(text, "id", 0); !ok || got != 7 {
		t.Errorf("Next(%q, 0) = %d, %v; want 7, true — a lower-case pattern ignores case", "id", got, ok)
	}
	if got, ok := Next(text, "ID", 0); !ok || got != 11 {
		t.Errorf("Next(%q, 0) = %d, %v; want 11, true — a capital makes it exact", "ID", got, ok)
	}
}

func TestSearchWalksForwardAndBackFromWhereTheCursorIs(t *testing.T) {
	const text = "one two one two one"
	//            0123456789...
	//            "one" at 0, 8, 16

	if got, ok := Next(text, "one", 1); !ok || got != 8 {
		t.Errorf("Next from 1 = %d, %v; want 8, true", got, ok)
	}
	if got, ok := Next(text, "one", 8); !ok || got != 8 {
		t.Errorf("Next from 8 = %d, %v; want 8, true — from is inclusive", got, ok)
	}
	if _, ok := Next(text, "one", 17); ok {
		t.Error("Next past the last match reported one; the caller needs the miss to wrap")
	}

	if got, ok := Prev(text, "one", 16); !ok || got != 8 {
		t.Errorf("Prev before 16 = %d, %v; want 8, true — before is exclusive", got, ok)
	}
	if _, ok := Prev(text, "one", 0); ok {
		t.Error("Prev before the first match reported one; the caller needs the miss to wrap")
	}
}

// The editor addresses its buffer in bytes, so an offset measured against
// anything but the original text lands the caret in the middle of a character
// and the next edit corrupts the file.
//
// The obvious implementation — lower-case the text, then Index — is wrong for
// exactly this reason: folding "İ" shortens it by a byte, so every offset
// after one is reported two bytes early here.
func TestMatchesAreOffsetsIntoTheOriginalTextNotAFoldedCopy(t *testing.T) {
	for _, text := range []string{
		"주석 -- note\nselect note",
		"İİ note",
	} {
		want := strings.Index(text, "note")

		got, ok := Next(text, "note", 0)
		if !ok {
			t.Errorf("Next(%q) found nothing", text)
			continue
		}
		if got != want {
			t.Errorf("Next(%q) = %d, want %d (byte offset into the original)", text, got, want)
			continue
		}
		if text[got:got+len("note")] != "note" {
			t.Errorf("offset %d points at %q, not at the match", got, text[got:got+len("note")])
		}
	}
}

func TestContainsFollowsTheSameCaseRuleAsTheRest(t *testing.T) {
	if !Contains("SELECT id", "select") {
		t.Error("a lower-case pattern did not match upper-case text")
	}
	if Contains("select id", "ID") {
		t.Error("a capital in the pattern was ignored")
	}
	if Contains("anything", "") {
		t.Error("an empty pattern reported a match")
	}
}

func TestAnEmptyPatternNeverMatches(t *testing.T) {
	if _, ok := Next("anything", "", 0); ok {
		t.Error("an empty pattern reported a match; nothing would be found to jump to")
	}
	if _, ok := Prev("anything", "", 8); ok {
		t.Error("an empty pattern reported a match")
	}
}

// A caret at the very end of the buffer is an ordinary place to search from.
func TestSearchingFromTheEndsOfTheBufferDoesNotPanic(t *testing.T) {
	const text = "select 1"
	if _, ok := Next(text, "select", len(text)); ok {
		t.Error("found a match starting past the end of the text")
	}
	if _, ok := Next(text, "select", -1); !ok {
		t.Error("a negative offset should clamp to the start rather than find nothing")
	}
	if _, ok := Prev(text, "select", len(text)+100); !ok {
		t.Error("an offset past the end should clamp rather than find nothing")
	}
}
