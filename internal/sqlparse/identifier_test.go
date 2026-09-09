package sqlparse

import "testing"

func TestQuoteIdentifierWrapsAPlainNameInBackticks(t *testing.T) {
	if got, want := QuoteIdentifier("orders"), "`orders`"; got != want {
		t.Errorf("QuoteIdentifier(%q) = %q, want %q", "orders", got, want)
	}
}

// MySQL ends a quoted identifier at the first unescaped backtick, so one
// inside the name has to be doubled or the rest of the name becomes syntax.
func TestQuoteIdentifierDoublesAnEmbeddedBacktick(t *testing.T) {
	if got, want := QuoteIdentifier("we`ird"), "`we``ird`"; got != want {
		t.Errorf("QuoteIdentifier(%q) = %q, want %q", "we`ird", got, want)
	}
}

func TestQuoteIdentifierOfAnEmptyStringIsAnEmptyPairOfBackticks(t *testing.T) {
	if got, want := QuoteIdentifier(""), "``"; got != want {
		t.Errorf("QuoteIdentifier(\"\") = %q, want %q", got, want)
	}
}

// QuoteIdentifier quotes exactly the string it is given; a caller that wants
// "schema"."table" has to quote each part itself and join them, because a
// dot inside one call is just a character in the name, not a separator.
func TestQuoteIdentifierTreatsADotAsPartOfOneName(t *testing.T) {
	if got, want := QuoteIdentifier("schema.table"), "`schema.table`"; got != want {
		t.Errorf("QuoteIdentifier(%q) = %q, want %q", "schema.table", got, want)
	}
}
