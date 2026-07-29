package sqlparse

import "testing"

func TestAppendLimit(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "plain select",
			sql:  "SELECT * FROM t",
			want: "SELECT * FROM t LIMIT 1000",
		},
		{
			name: "after order by",
			sql:  "SELECT * FROM t ORDER BY id",
			want: "SELECT * FROM t ORDER BY id LIMIT 1000",
		},
		{
			name: "newlines are preserved",
			sql:  "SELECT *\nFROM t\nWHERE x = 1",
			want: "SELECT *\nFROM t\nWHERE x = 1 LIMIT 1000",
		},
		// A trailing comment would swallow the appended clause.
		{
			name: "before a trailing line comment",
			sql:  "SELECT * FROM t -- check this",
			want: "SELECT * FROM t LIMIT 1000 -- check this",
		},
		{
			name: "before a trailing block comment",
			sql:  "SELECT * FROM t /* note */",
			want: "SELECT * FROM t LIMIT 1000 /* note */",
		},
		// LIMIT has to precede the locking clause to be valid SQL.
		{
			name: "before FOR UPDATE",
			sql:  "SELECT * FROM t FOR UPDATE",
			want: "SELECT * FROM t LIMIT 1000 FOR UPDATE",
		},
		{
			name: "before LOCK IN SHARE MODE",
			sql:  "SELECT * FROM t LOCK IN SHARE MODE",
			want: "SELECT * FROM t LIMIT 1000 LOCK IN SHARE MODE",
		},
		{
			name: "before FOR SHARE",
			sql:  "SELECT * FROM t FOR SHARE",
			want: "SELECT * FROM t LIMIT 1000 FOR SHARE",
		},
		{
			name: "union applies to the whole result",
			sql:  "SELECT 1 UNION SELECT 2",
			want: "SELECT 1 UNION SELECT 2 LIMIT 1000",
		},
		// "FOR UPDATE" inside a literal is data, not a clause.
		{
			name: "literal containing a clause keyword",
			sql:  "SELECT 'FOR UPDATE' FROM t",
			want: "SELECT 'FOR UPDATE' FROM t LIMIT 1000",
		},
		{
			name: "quoted identifier named for",
			sql:  "SELECT `for` FROM t",
			want: "SELECT `for` FROM t LIMIT 1000",
		},
		// A locking clause inside a subquery is not the outer statement's.
		{
			name: "for update inside a subquery",
			sql:  "SELECT * FROM (SELECT 1 FOR UPDATE) s",
			want: "SELECT * FROM (SELECT 1 FOR UPDATE) s LIMIT 1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendLimit(Parse(tt.sql), 1000); got != tt.want {
				t.Errorf("AppendLimit(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

// Rather than risk producing invalid SQL, an unhandled shape is left alone;
// too many rows is a nuisance, a broken query is a betrayal.
func TestAppendLimitLeavesRiskyShapesUntouched(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{name: "already limited", sql: "SELECT * FROM t LIMIT 5"},
		{name: "into outfile", sql: "SELECT * FROM t INTO OUTFILE '/tmp/x'"},
		{name: "into variable", sql: "SELECT id FROM t INTO @x"},
		{name: "empty", sql: ""},
		{name: "comment only", sql: "-- nothing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendLimit(Parse(tt.sql), 1000); got != tt.sql {
				t.Errorf("AppendLimit(%q) = %q, want it unchanged", tt.sql, got)
			}
		})
	}
}

func TestAppendLimitWithNonPositiveCountIsANoop(t *testing.T) {
	const sql = "SELECT * FROM t"

	for _, n := range []int{0, -1} {
		if got := AppendLimit(Parse(sql), n); got != sql {
			t.Errorf("AppendLimit(%q, %d) = %q, want it unchanged", sql, n, got)
		}
	}
}

// Whatever AppendLimit produces must still parse as one bounded statement.
func TestAppendLimitProducesAParsableBoundedStatement(t *testing.T) {
	sources := []string{
		"SELECT * FROM t",
		"SELECT * FROM t ORDER BY id",
		"SELECT * FROM t -- note",
		"SELECT * FROM t FOR UPDATE",
		"SELECT 1 UNION SELECT 2",
	}

	for _, sql := range sources {
		t.Run(sql, func(t *testing.T) {
			got := Parse(AppendLimit(Parse(sql), 1000))

			if !got.HasTopLevelLimit() {
				t.Errorf("AppendLimit(%q) = %q, which has no top-level LIMIT", sql, got.SQL)
			}
			if n := len(Split(got.SQL)); n != 1 {
				t.Errorf("AppendLimit(%q) = %q, which splits into %d statements", sql, got.SQL, n)
			}
		})
	}
}
