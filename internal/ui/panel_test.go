package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Ahngbeom/datavase/internal/result"
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
	header, _ := tabHeader([]string{"tree", "tables"}, 0, 40)
	got := stripTags(header)

	for _, want := range []string{"tree", "tables"} {
		if !strings.Contains(got, want) {
			t.Errorf("tabHeader() = %q, want it to list %q", got, want)
		}
	}
}

// Which tab is active has to be visible; a header that only lists names is
// no better than none.
func TestTabHeaderDistinguishesTheActiveTab(t *testing.T) {
	first, _ := tabHeader([]string{"tree", "tables"}, 0, 40)
	second, _ := tabHeader([]string{"tree", "tables"}, 1, 40)

	if first == second {
		t.Errorf("both tabs render identically: %q", first)
	}
}

// A narrow pane may abbreviate the names, but never at the cost of showing
// which tab is active — that is the whole job of the header.
func TestTabHeaderFitsNarrowPanes(t *testing.T) {
	for _, width := range []int{40, 30, 20, 14, 10, 6} {
		first, _ := tabHeader([]string{"tree", "tables"}, 0, width)
		second, _ := tabHeader([]string{"tree", "tables"}, 1, width)

		if got := len([]rune(stripTags(first))); got > width {
			t.Errorf("width %d: header is %d runes: %q", width, got, stripTags(first))
		}
		if first == second {
			t.Errorf("width %d: the active tab is indistinguishable: %q", width, stripTags(first))
		}
	}
}

func TestTabHeaderWithASingleTab(t *testing.T) {
	header, _ := tabHeader([]string{"results"}, 0, 40)
	got := stripTags(header)

	if !strings.Contains(got, "results") {
		t.Errorf("tabHeader() = %q, want the single tab named", got)
	}
}

func TestTabHeaderWithNoTabs(t *testing.T) {
	if got, _ := tabHeader(nil, 0, 40); got != "" {
		t.Errorf("tabHeader(nil) = %q, want empty", got)
	}
}

// An out-of-range index must not panic; the pane may be redrawn while tabs
// are being rebuilt.
func TestTabHeaderToleratesABadIndex(t *testing.T) {
	for _, active := range []int{-1, 2, 99} {
		got, _ := tabHeader([]string{"tree", "tables"}, active, 40)
		if got == "" {
			t.Errorf("active %d: header is empty", active)
		}
	}
}

// Tab names come from the code, but escaping them keeps the header honest if
// one ever carries a bracket.
func TestTabHeaderEscapesNames(t *testing.T) {
	got, _ := tabHeader([]string{"a[1]"}, 0, 40)

	if strings.Contains(got, "[1]") && !strings.Contains(got, "[[1]") {
		t.Errorf("tabHeader() = %q, want the name escaped", got)
	}
}

// Tab names abbreviate as the pane narrows. A zone measured against the full
// name would put the wrong tab under the pointer.
func TestTabZonesAgreeWithTheAbbreviatedNames(t *testing.T) {
	names := []string{"results", "ddl", "messages"}
	const active = 1

	for _, width := range []int{60, 30, 18, 12, 8} {
		header, zones := regionHeader(names, active, true, "", width)
		plain := []rune(visibleText(header))

		tabs := 0
		for _, z := range zones {
			if z.target != zoneTab {
				continue
			}
			tabs++
			if z.from < 0 || z.to > len(plain) || z.from >= z.to {
				t.Fatalf("width %d: tab zone %+v is outside %q", width, z, string(plain))
			}
			covered := string(plain[z.from:z.to])

			label := covered
			if z.index == active {
				if !strings.HasPrefix(covered, "▸") {
					t.Errorf("width %d: active tab %d's zone %q carries no marker", width, z.index, covered)
					continue
				}
				label = strings.TrimPrefix(covered, "▸")
			}

			// A zone one column short drops the label's last rune; one column
			// long pulls in the separator or the next tab's marker. Either
			// way, running the label back through Truncate at its own length
			// no longer reproduces it — Truncate is keyed on rune count, so
			// only the exact span round-trips.
			want := result.Truncate(names[z.index], utf8.RuneCountInString(label))
			if label != want {
				t.Errorf("width %d: zone for tab %d covers %q (label %q), want %q",
					width, z.index, covered, label, want)
			}
		}
		if width >= 30 && tabs != len(names) {
			t.Errorf("width %d: %d tab zones for %d tabs", width, tabs, len(names))
		}
	}
}

// A tab strip of one is what the editor has: no tab to switch to, so nothing
// there should answer a click as though there were.
func TestASingleUnnamedTabPublishesNoTabZone(t *testing.T) {
	_, zones := regionHeader([]string{""}, 0, false, "002_add_index.sql *", 60)

	for _, z := range zones {
		if z.target == zoneTab {
			t.Errorf("a region holding one unnamed thing published a tab zone: %+v", z)
		}
	}
}
