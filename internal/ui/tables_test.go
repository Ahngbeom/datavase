package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/catalog"
)

func sampleTables() []catalog.Table {
	return []catalog.Table{
		{Name: "orders", Rows: 1200},
		{Name: "order_items", Rows: 45000},
		{Name: "customers", Rows: 300},
		{Name: "vw_order_summary", IsView: true},
	}
}

func tableNames(tables []catalog.Table) []string {
	out := make([]string, len(tables))
	for i, t := range tables {
		out[i] = t.Name
	}
	return out
}

func TestFilterTablesMatchesSubstrings(t *testing.T) {
	got := tableNames(filterTables(sampleTables(), "order"))

	for _, want := range []string{"orders", "order_items", "vw_order_summary"} {
		if !contains(got, want) {
			t.Errorf("filterTables(order) = %v, want it to include %q", got, want)
		}
	}
	if contains(got, "customers") {
		t.Errorf("filterTables(order) = %v, want customers excluded", got)
	}
}

func TestFilterTablesIsCaseInsensitive(t *testing.T) {
	if got := tableNames(filterTables(sampleTables(), "ORDER")); !contains(got, "orders") {
		t.Errorf("filterTables(ORDER) = %v, want a case-insensitive match", got)
	}
}

// An empty filter is the plain list, which is what the tab shows on open.
func TestFilterTablesWithNoTermReturnsEverything(t *testing.T) {
	if got := filterTables(sampleTables(), ""); len(got) != 4 {
		t.Errorf("filterTables(\"\") returned %d tables, want all 4", len(got))
	}
}

// A name starting with the term is more likely the one wanted than one that
// merely contains it.
func TestFilterTablesRanksPrefixMatchesFirst(t *testing.T) {
	got := tableNames(filterTables(sampleTables(), "order"))

	if len(got) < 2 {
		t.Fatalf("filterTables(order) = %v, want several matches", got)
	}
	if strings.HasPrefix(got[len(got)-1], "order") {
		t.Errorf("filterTables(order) = %v, want the substring match last", got)
	}
	if !strings.HasPrefix(got[0], "order") {
		t.Errorf("filterTables(order) = %v, want a prefix match first", got)
	}
}

// The filter is text a user typed, so its wildcards are literal.
func TestFilterTablesTreatsWildcardsAsText(t *testing.T) {
	tables := []catalog.Table{{Name: "order_items"}, {Name: "orderXitems"}}

	got := tableNames(filterTables(tables, "order_items"))
	if len(got) != 1 || got[0] != "order_items" {
		t.Errorf("filterTables(order_items) = %v, want only the literal match", got)
	}
}

func TestFilterTablesWithNoMatches(t *testing.T) {
	if got := filterTables(sampleTables(), "zzz"); len(got) != 0 {
		t.Errorf("filterTables(zzz) = %v, want nothing", tableNames(got))
	}
}

// The row estimate and the view flag are what make the list worth reading
// rather than just a column of names.
func TestTableRowDetail(t *testing.T) {
	tests := []struct {
		name  string
		table catalog.Table
		want  string
	}{
		{name: "table with rows", table: catalog.Table{Name: "orders", Rows: 1200}, want: "1.2k"},
		{name: "large table", table: catalog.Table{Name: "big", Rows: 45000}, want: "45.0k"},
		{name: "millions", table: catalog.Table{Name: "huge", Rows: 2500000}, want: "2.5M"},
		{name: "small table", table: catalog.Table{Name: "small", Rows: 42}, want: "42"},
		{name: "view", table: catalog.Table{Name: "v", IsView: true}, want: "view"},
		// A zero estimate is unknown, not empty: InnoDB reports it for
		// tables it has not analysed.
		{name: "unknown size", table: catalog.Table{Name: "t"}, want: "table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tableDetail(tt.table); got != tt.want {
				t.Errorf("tableDetail(%+v) = %q, want %q", tt.table, got, tt.want)
			}
		})
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
