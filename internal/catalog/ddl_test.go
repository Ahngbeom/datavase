package catalog

import "testing"

// Identifiers cannot be bound as parameters, so this escaping is the only
// thing standing between a table name and the statement's syntax.
func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "ordinary", in: "orders", want: "`orders`"},
		{name: "underscore", in: "order_items", want: "`order_items`"},
		{name: "backtick is doubled", in: "we`ird", want: "`we``ird`"},
		{name: "several backticks", in: "a`b`c", want: "`a``b``c`"},
		{name: "multibyte", in: "테이블", want: "`테이블`"},
		{name: "empty", in: "", want: "``"},
		// A name that tries to close the quote and append SQL must end up
		// inert rather than executable.
		{name: "injection attempt", in: "t`; DROP TABLE users; --", want: "`t``; DROP TABLE users; --`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuoteIdentifier(tt.in); got != tt.want {
				t.Errorf("QuoteIdentifier(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The quoted form must always be balanced: an odd number of backticks would
// leave the statement open.
func TestQuoteIdentifierIsAlwaysBalanced(t *testing.T) {
	names := []string{"a", "a`", "`a", "``", "a``b", "`", "```"}

	for _, name := range names {
		quoted := QuoteIdentifier(name)

		count := 0
		for _, r := range quoted {
			if r == '`' {
				count++
			}
		}
		if count%2 != 0 {
			t.Errorf("QuoteIdentifier(%q) = %q, which has %d backticks", name, quoted, count)
		}
	}
}

// SHOW CREATE names its definition column differently for tables and views,
// and pads the row with version-dependent extras.
func TestDefinitionColumn(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		cells   []any
		want    string
	}{
		{
			name:    "table",
			columns: []string{"Table", "Create Table"},
			cells:   []any{[]byte("orders"), []byte("CREATE TABLE `orders` (…)")},
			want:    "CREATE TABLE `orders` (…)",
		},
		{
			name:    "view with the extra charset columns",
			columns: []string{"View", "Create View", "character_set_client", "collation_connection"},
			cells: []any{
				[]byte("v"), []byte("CREATE VIEW `v` AS SELECT 1"),
				[]byte("utf8mb4"), []byte("utf8mb4_general_ci"),
			},
			want: "CREATE VIEW `v` AS SELECT 1",
		},
		{
			name:    "string rather than bytes",
			columns: []string{"Table", "Create Table"},
			cells:   []any{"orders", "CREATE TABLE `orders` (…)"},
			want:    "CREATE TABLE `orders` (…)",
		},
		{
			name:    "no definition column",
			columns: []string{"Table"},
			cells:   []any{[]byte("orders")},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := definitionColumn(tt.columns, tt.cells); got != tt.want {
				t.Errorf("definitionColumn() = %q, want %q", got, tt.want)
			}
		})
	}
}
