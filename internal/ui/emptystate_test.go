package ui

import (
	"strings"
	"testing"
)

// An empty finder has two quite different things to say, and they were the
// same sentence.
//
// Before anything is typed nothing has been searched for, so "no matching
// tables" is not a result — it is a report on a search nobody ran, sitting
// above an invitation to run one. The two lines contradicted each other on the
// screen a user meets first.
//
// noMatch is the answer to a search that found nothing; nothingHere is the
// answer to a place with nothing in it.
func TestAnUnsearchedFinderDoesNotReportAFailedSearch(t *testing.T) {
	for _, tt := range []struct {
		name string
		item searchItem
	}{
		{"nothing here", nothingHere("no tables in the schema cache yet", "the schema is still loading")},
		{"no match", noMatch("table", "customers")},
	} {
		if tt.item.accept != nil {
			t.Errorf("%s: an empty state is offering itself as a choice", tt.name)
		}
		if tt.item.primary == "" {
			t.Errorf("%s: an empty state says nothing", tt.name)
		}
	}
}

// The failure names what was looked for, so a typo is visible in the answer
// rather than only in the field above it.
func TestAFailedSearchQuotesWhatWasLookedFor(t *testing.T) {
	item := noMatch("table", "custmoers")

	if !strings.Contains(item.primary, "custmoers") {
		t.Errorf("the failure does not repeat the term: %q", item.primary)
	}
	if !strings.Contains(item.primary, "table") {
		t.Errorf("the failure does not name what was searched: %q", item.primary)
	}
}

// An invitation is not a failure, so it must not be worded as one — that is
// the whole of the confusion being undone here.
func TestAnEmptyPlaceIsNotWordedAsAFailedSearch(t *testing.T) {
	item := nothingHere("no tables in the schema cache yet", "it is still loading")

	for _, forbidden := range []string{"no matching", "not found", "no match"} {
		if strings.Contains(strings.ToLower(item.primary), forbidden) {
			t.Errorf("an empty place is worded as a failed search: %q", item.primary)
		}
	}
}
