package ui

import (
	"strings"
	"testing"
)

// The row view steps between rows without closing — comparing two rows is most
// of what it is opened for — and nothing on screen said so. The key reference
// two dialogs over puts its own controls in its title bar for exactly this
// reason; this one carried only "row 1 of 5", so the stepping was a feature
// you had to already know about to find.
func TestTheRowViewNamesItsControls(t *testing.T) {
	title := rowViewTitle(2, 40)

	if !strings.Contains(title, "2 of 40") {
		t.Errorf("the title does not say which row is showing: %q", title)
	}
	for _, want := range []string{"j", "k", "Esc"} {
		if !strings.Contains(title, want) {
			t.Errorf("the title does not name %q: %q", want, title)
		}
	}
}

// The marker beside a schema in the tree is the only thing that says which one
// an unqualified statement will reach, and it is a dot. A dot explains
// nothing, and the pane it sits in had no room used for anything else.
func TestTheSchemaPaneExplainsItsMarker(t *testing.T) {
	legend := schemaPaneDetail(tabTree, "app_db")

	if !strings.Contains(legend, currentSchemaMarker) {
		t.Errorf("the legend does not show the marker it explains: %q", legend)
	}
	if !strings.Contains(legend, "current") {
		t.Errorf("the legend does not say what the marker means: %q", legend)
	}
}

// With no schema chosen nothing is marked, and a legend for a marker that is
// not on screen is one more thing to work out rather than one fewer.
func TestTheSchemaPaneExplainsNothingWithNoCurrentSchema(t *testing.T) {
	if got := schemaPaneDetail(tabTree, ""); got != "" {
		t.Errorf("schemaPaneDetail() = %q, want nothing", got)
	}
}

// The tables tab draws no markers, so the legend belongs to the tree alone.
func TestTheTablesTabCarriesNoMarkerLegend(t *testing.T) {
	if got := schemaPaneDetail(tabTables, "app_db"); got != "" {
		t.Errorf("schemaPaneDetail() = %q, want nothing", got)
	}
}

// The legend has to survive the pane it lives in, which is narrow and fixed.
func TestTheMarkerLegendFitsTheSchemaPane(t *testing.T) {
	legend := schemaPaneDetail(tabTree, "app_db")

	// The marker, the region marker, the tab strip and the gap before the
	// detail all come out of the pane's width first.
	room := sidebarWidth - 2 - visibleCost("▸tree tables") - 2
	if got := visibleCost(legend); got > room {
		t.Errorf("the legend is %d cells and the pane leaves %d: %q", got, room, legend)
	}
}
