package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/complete"
	"github.com/rivo/uniseg"
)

// A Korean name is two cells a character. Measured in runes, it pushed the
// kind column out of line with every other row and ran past the menu's edge.
func TestTheMenuLinesUpKindsBehindWideNames(t *testing.T) {
	layout := menuLayout([]complete.Candidate{
		{Text: "고객", Kind: complete.KindTable},
		{Text: "orders", Kind: complete.KindTable},
	})

	if layout.nameWidth != 6 {
		t.Errorf("the name column is %d cells wide, want 6 (orders)", layout.nameWidth)
	}
	for i, name := range layout.names {
		if got := uniseg.StringWidth(name); got != layout.nameWidth {
			t.Errorf("row %d's name %q fills %d cells, want %d so the kinds line up",
				i, name, got, layout.nameWidth)
		}
	}

	wide := menuLayout([]complete.Candidate{{Text: "주문내역", Kind: complete.KindTable}})
	if wide.nameWidth != 8 {
		t.Errorf("a four-character Korean name is measured as %d cells, want 8", wide.nameWidth)
	}
	if want := 8 + uniseg.StringWidth(wide.trailers[0]) + 1; wide.width != want {
		t.Errorf("the menu is %d cells wide, want %d: it must fit what it draws", wide.width, want)
	}
}

// A column comment can hold line breaks. Drawn as they are, a newline or a
// tab in one cell leaves the rest of that row misplaced.
func TestTheMenuDrawsADetailsLineBreaksAsSpaces(t *testing.T) {
	layout := menuLayout([]complete.Candidate{
		{Text: "id", Kind: complete.KindColumn, Detail: "first\nsecond\tthird\r"},
	})

	if strings.ContainsAny(layout.trailers[0], "\n\t\r") {
		t.Errorf("the detail is drawn with its control characters: %q", layout.trailers[0])
	}
	if !strings.Contains(layout.trailers[0], "first second third") {
		t.Errorf("the detail lost its words: %q", layout.trailers[0])
	}
}
