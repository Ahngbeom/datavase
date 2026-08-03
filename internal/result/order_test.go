package result

import (
	"math"
	"testing"
	"time"
)

func TestOrderForReadsTheColumnsDeclaredType(t *testing.T) {
	for _, tt := range []struct {
		declared string
		want     Order
	}{
		{"BIGINT", OrderNumber},
		{"INT", OrderNumber},
		{"UNSIGNED BIGINT", OrderNumber},
		{"DECIMAL", OrderNumber},
		{"DOUBLE", OrderNumber},
		{"DATETIME", OrderTime},
		{"TIMESTAMP", OrderTime},
		{"DATE", OrderTime},
		{"VARCHAR", OrderText},
		{"TEXT", OrderText},
		{"JSON", OrderText},
		{"BLOB", OrderText},

		// A driver is allowed to say nothing, and several statements carry no
		// type information at all. Text is the answer that claims least: an
		// ordering the column cannot justify is worse than a coarse one.
		{"", OrderText},
		{"SOMETHING NEW", OrderText},

		// Case is the driver's business, not the caller's.
		{"bigint", OrderNumber},
	} {
		if got := OrderFor(tt.declared); got != tt.want {
			t.Errorf("OrderFor(%q) = %v, want %v", tt.declared, got, tt.want)
		}
	}
}

// The whole reason this exists. Values arrive as bytes over the text protocol,
// so an ordering that compared them as they arrive would put 10 before 9 in a
// column of integers.
func TestANumericColumnDoesNotSortItsValuesAsText(t *testing.T) {
	if Compare([]byte("9"), []byte("10"), OrderNumber) >= 0 {
		t.Error("9 did not sort before 10 in a numeric column")
	}
	// And the same values in a column the server sorts as text keep the
	// server's answer, rather than being quietly improved on.
	if Compare([]byte("9"), []byte("10"), OrderText) <= 0 {
		t.Error("\"9\" did not sort after \"10\" in a text column")
	}
}

func TestCompareOrdersValues(t *testing.T) {
	when := func(s string) time.Time {
		at, err := time.Parse("2006-01-02 15:04:05", s)
		if err != nil {
			t.Fatal(err)
		}
		return at
	}

	for _, tt := range []struct {
		name  string
		a, b  any
		order Order
		want  int
	}{
		// NULL sorts first ascending, which is what the server does and so
		// what a DBA reading the column expects to see.
		{"null before a number", nil, []byte("0"), OrderNumber, -1},
		{"null before text", nil, []byte(""), OrderText, -1},
		{"two nulls are equal", nil, nil, OrderText, 0},
		{"a number after null", []byte("-5"), nil, OrderNumber, 1},

		{"negatives sort below", []byte("-5"), []byte("3"), OrderNumber, -1},
		{"decimals compare as numbers", []byte("2.5"), []byte("10.1"), OrderNumber, -1},
		{"equal numbers, differently written", []byte("1.0"), []byte("1"), OrderNumber, 0},
		{"typed integers work too", int64(2), int64(11), OrderNumber, -1},

		// A numeric column that turns out to hold something unparseable must
		// not decide the order by accident. Whatever cannot be read as a
		// number sorts after everything that can, and ties break on the text.
		{"unparseable sorts after a number", []byte("n/a"), []byte("99999"), OrderNumber, 1},
		{"two unparseable fall back to text", []byte("abc"), []byte("abd"), OrderNumber, -1},

		{"text compares as text", []byte("alpha"), []byte("beta"), OrderText, -1},
		{"identical text is equal", []byte("same"), []byte("same"), OrderText, 0},

		{"times compare as times", when("2026-01-02 03:04:05"), when("2026-06-01 00:00:00"), OrderTime, -1},
		{"time strings compare as times", []byte("2026-01-02 03:04:05"), []byte("2026-06-01 00:00:00"), OrderTime, -1},
		{"a date and a datetime compare", []byte("2026-01-02"), []byte("2026-01-02 00:00:01"), OrderTime, -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.a, tt.b, tt.order)
			if sign(got) != tt.want {
				t.Errorf("Compare(%v, %v, %v) = %d, want sign %d", tt.a, tt.b, tt.order, got, tt.want)
			}
			// Reversing the arguments has to reverse the answer, or a sort
			// built on this can order a list differently depending on where it
			// started.
			if back := Compare(tt.b, tt.a, tt.order); sign(back) != -tt.want {
				t.Errorf("Compare(%v, %v, %v) = %d, which does not mirror %d", tt.b, tt.a, tt.order, back, got)
			}
		})
	}
}

// A NaN in a numeric column must not make the ordering inconsistent: sort
// panics on a comparator that says a < b and b < a at once.
func TestCompareStaysConsistentAroundNaN(t *testing.T) {
	nan := math.NaN()

	if a, b := Compare(nan, 1.0, OrderNumber), Compare(1.0, nan, OrderNumber); sign(a) != -sign(b) {
		t.Errorf("NaN compares %d one way and %d the other", a, b)
	}
	if got := Compare(nan, nan, OrderNumber); got != 0 {
		t.Errorf("Compare(NaN, NaN) = %d, want 0", got)
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
