package ui

import (
	"strings"
	"testing"
)

// stripTags removes tview colour tags so assertions see what a user does.
func stripTags(s string) string {
	var (
		b      strings.Builder
		inTag  bool
		runes  = []rune(s)
		length = len(runes)
	)
	for i := 0; i < length; i++ {
		switch {
		case runes[i] == '[' && i+1 < length && runes[i+1] == '[':
			b.WriteRune('[')
			i++
		case runes[i] == '[':
			inTag = true
		case runes[i] == ']' && inTag:
			inTag = false
		case !inTag:
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}

func TestTabHeaderShowsEveryTab(t *testing.T) {
	got := stripTags(tabHeader([]string{"tree", "tables"}, 0, 40))

	for _, want := range []string{"tree", "tables"} {
		if !strings.Contains(got, want) {
			t.Errorf("tabHeader() = %q, want it to list %q", got, want)
		}
	}
}

// Which tab is active has to be visible; a header that only lists names is
// no better than none.
func TestTabHeaderDistinguishesTheActiveTab(t *testing.T) {
	first := tabHeader([]string{"tree", "tables"}, 0, 40)
	second := tabHeader([]string{"tree", "tables"}, 1, 40)

	if first == second {
		t.Errorf("both tabs render identically: %q", first)
	}
}

// A narrow pane may abbreviate the names, but never at the cost of showing
// which tab is active — that is the whole job of the header.
func TestTabHeaderFitsNarrowPanes(t *testing.T) {
	for _, width := range []int{40, 30, 20, 14, 10, 6} {
		first := tabHeader([]string{"tree", "tables"}, 0, width)
		second := tabHeader([]string{"tree", "tables"}, 1, width)

		if got := len([]rune(stripTags(first))); got > width {
			t.Errorf("width %d: header is %d runes: %q", width, got, stripTags(first))
		}
		if first == second {
			t.Errorf("width %d: the active tab is indistinguishable: %q", width, stripTags(first))
		}
	}
}

func TestTabHeaderWithASingleTab(t *testing.T) {
	got := stripTags(tabHeader([]string{"results"}, 0, 40))

	if !strings.Contains(got, "results") {
		t.Errorf("tabHeader() = %q, want the single tab named", got)
	}
}

func TestTabHeaderWithNoTabs(t *testing.T) {
	if got := tabHeader(nil, 0, 40); got != "" {
		t.Errorf("tabHeader(nil) = %q, want empty", got)
	}
}

// An out-of-range index must not panic; the pane may be redrawn while tabs
// are being rebuilt.
func TestTabHeaderToleratesABadIndex(t *testing.T) {
	for _, active := range []int{-1, 2, 99} {
		got := tabHeader([]string{"tree", "tables"}, active, 40)
		if got == "" {
			t.Errorf("active %d: header is empty", active)
		}
	}
}

// Tab names come from the code, but escaping them keeps the header honest if
// one ever carries a bracket.
func TestTabHeaderEscapesNames(t *testing.T) {
	got := tabHeader([]string{"a[1]"}, 0, 40)

	if strings.Contains(got, "[1]") && !strings.Contains(got, "[[1]") {
		t.Errorf("tabHeader() = %q, want the name escaped", got)
	}
}
