package result

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Order is how a column's values are put in sequence.
type Order int

const (
	// OrderText compares the formatted values, which is what the server does
	// for a character column and the only thing that can be claimed about a
	// column whose type is unknown.
	OrderText Order = iota
	OrderNumber
	OrderTime
)

// OrderFor decides from the column's declared type.
//
// It is the type that decides and never the values, because values reach here
// as bytes: everything a query returns over the text protocol is a []byte,
// so "9" and "10" look alike whether the column is a BIGINT or a VARCHAR.
// Guessing from the bytes would sort a VARCHAR numerically and disagree with
// the server about its own column.
//
// An unknown type is text. A driver is allowed to report nothing, and an
// ordering a column cannot justify is worse than a coarse one.
func OrderFor(databaseType string) Order {
	switch upper := strings.ToUpper(databaseType); {
	case strings.Contains(upper, "INT"),
		strings.Contains(upper, "DECIMAL"),
		strings.Contains(upper, "NUMERIC"),
		strings.Contains(upper, "FLOAT"),
		strings.Contains(upper, "DOUBLE"),
		strings.Contains(upper, "YEAR"):
		return OrderNumber

	case strings.Contains(upper, "DATE"),
		strings.Contains(upper, "TIME"):
		return OrderTime

	default:
		return OrderText
	}
}

// timeLayouts are what MySQL sends for its date and time types, longest
// first so that a DATETIME is not read as the DATE it starts with.
var timeLayouts = []string{
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"15:04:05",
}

// Compare orders two values of one column, returning the usual negative, zero
// or positive.
//
// NULL sorts first, as it does in an ascending ORDER BY. Anything that cannot
// be read as the column's kind sorts after everything that can and then ties
// on its text, so a stray value in a numeric column lands somewhere it can be
// seen rather than somewhere arbitrary.
func Compare(a, b any, order Order) int {
	if a == nil || b == nil {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		default:
			return 1
		}
	}

	switch order {
	case OrderNumber:
		return compareParsed(a, b, asNumber, func(x, y float64) int {
			// NaN is not less than, greater than or equal to anything, and a
			// comparator that says so makes sort panic. Treating them as
			// equal to each other and above everything else keeps the
			// ordering consistent, which is what sort actually requires.
			switch {
			case math.IsNaN(x) && math.IsNaN(y):
				return 0
			case math.IsNaN(x):
				return 1
			case math.IsNaN(y):
				return -1
			case x < y:
				return -1
			case x > y:
				return 1
			}
			return 0
		})

	case OrderTime:
		return compareParsed(a, b, asTime, func(x, y time.Time) int {
			return x.Compare(y)
		})
	}
	return strings.Compare(Format(a), Format(b))
}

// compareParsed applies a typed comparison where both values could be read as
// that type, and falls back to text where they could not.
func compareParsed[T any](a, b any, parse func(any) (T, bool), cmp func(T, T) int) int {
	x, okA := parse(a)
	y, okB := parse(b)

	switch {
	case okA && okB:
		if c := cmp(x, y); c != 0 {
			return c
		}
		return 0
	case okA:
		return -1
	case okB:
		return 1
	}
	return strings.Compare(Format(a), Format(b))
}

func asNumber(v any) (float64, bool) {
	switch value := v.(type) {
	case int64:
		return float64(value), true
	case uint64:
		return float64(value), true
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case bool:
		if value {
			return 1, true
		}
		return 0, true
	}

	n, err := strconv.ParseFloat(strings.TrimSpace(Format(v)), 64)
	return n, err == nil
}

func asTime(v any) (time.Time, bool) {
	if at, ok := v.(time.Time); ok {
		return at, true
	}

	text := strings.TrimSpace(Format(v))
	for _, layout := range timeLayouts {
		if at, err := time.Parse(layout, text); err == nil {
			return at, true
		}
	}
	return time.Time{}, false
}
