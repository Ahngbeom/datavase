package sqlparse

import (
	"strings"
	"testing"
)

// at builds a completion context from SQL with "|" marking the caret, which
// keeps the test cases readable.
func at(t *testing.T, marked string) CompletionContext {
	t.Helper()

	offset := strings.Index(marked, "|")
	if offset < 0 {
		t.Fatalf("test SQL %q has no caret marker", marked)
	}
	return CompletionAt(strings.Replace(marked, "|", "", 1), offset)
}

func TestCompletionKindFollowsTheClause(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want CompletionKind
	}{
		{name: "after SELECT", sql: "SELECT | FROM users", want: CompleteColumn},
		{name: "after FROM", sql: "SELECT * FROM |", want: CompleteTable},
		{name: "after JOIN", sql: "SELECT * FROM a JOIN |", want: CompleteTable},
		{name: "after LEFT JOIN", sql: "SELECT * FROM a LEFT JOIN |", want: CompleteTable},
		{name: "after WHERE", sql: "SELECT * FROM users WHERE |", want: CompleteColumn},
		{name: "after ORDER BY", sql: "SELECT * FROM users ORDER BY |", want: CompleteColumn},
		{name: "after GROUP BY", sql: "SELECT * FROM users GROUP BY |", want: CompleteColumn},
		{name: "after HAVING", sql: "SELECT * FROM users HAVING |", want: CompleteColumn},
		{name: "after ON", sql: "SELECT * FROM a JOIN b ON |", want: CompleteColumn},
		{name: "after SET in UPDATE", sql: "UPDATE users SET |", want: CompleteColumn},
		{name: "after UPDATE", sql: "UPDATE |", want: CompleteTable},
		{name: "after INSERT INTO", sql: "INSERT INTO |", want: CompleteTable},
		{name: "after DELETE FROM", sql: "DELETE FROM |", want: CompleteTable},
		{name: "at the very start", sql: "|", want: CompleteNone},
		{name: "inside a string literal", sql: "SELECT 'abc|' FROM t", want: CompleteNone},
		{name: "inside a comment", sql: "-- note |", want: CompleteNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := at(t, tt.sql).Kind; got != tt.want {
				t.Errorf("CompletionAt(%q).Kind = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

// The prefix is what has been typed so far, and it decides both what is
// offered and what gets replaced when a candidate is chosen.
func TestCompletionPrefix(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		want   string
		wantAt int // ReplaceFrom
	}{
		{name: "partial column", sql: "SELECT ema| FROM users", want: "ema", wantAt: 7},
		{name: "partial table", sql: "SELECT * FROM use|", want: "use", wantAt: 14},
		{name: "nothing typed", sql: "SELECT * FROM |", want: "", wantAt: 14},
		{name: "after a comma", sql: "SELECT a, | FROM t", want: "", wantAt: 10},
		{name: "uppercase is preserved", sql: "SELECT EMA| FROM users", want: "EMA", wantAt: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := at(t, tt.sql)
			if got.Prefix != tt.want {
				t.Errorf("Prefix = %q, want %q", got.Prefix, tt.want)
			}
			if got.ReplaceFrom != tt.wantAt {
				t.Errorf("ReplaceFrom = %d, want %d", got.ReplaceFrom, tt.wantAt)
			}
		})
	}
}

// "u." means "the columns of whatever u refers to", and that mapping lives
// after the caret — which is why the whole statement has to be parsed.
func TestQualifiedCompletion(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		wantQualifier string
		wantPrefix    string
	}{
		{
			name:          "alias defined later in the statement",
			sql:           "SELECT u.| FROM users u",
			wantQualifier: "u",
		},
		{
			name:          "alias with a partial column",
			sql:           "SELECT u.ema| FROM users u",
			wantQualifier: "u",
			wantPrefix:    "ema",
		},
		{
			name:          "table name used as the qualifier",
			sql:           "SELECT users.| FROM users",
			wantQualifier: "users",
		},
		{
			name:          "schema qualifier in a FROM clause",
			sql:           "SELECT * FROM app.|",
			wantQualifier: "app",
		},
		{
			name:          "backtick-quoted qualifier",
			sql:           "SELECT `u`.| FROM users u",
			wantQualifier: "u",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := at(t, tt.sql)
			if got.Kind != CompleteQualified {
				t.Fatalf("Kind = %v, want CompleteQualified", got.Kind)
			}
			if got.Qualifier != tt.wantQualifier {
				t.Errorf("Qualifier = %q, want %q", got.Qualifier, tt.wantQualifier)
			}
			if got.Prefix != tt.wantPrefix {
				t.Errorf("Prefix = %q, want %q", got.Prefix, tt.wantPrefix)
			}
		})
	}
}

// A qualified completion replaces only the part after the dot.
func TestQualifiedCompletionReplacesAfterTheDot(t *testing.T) {
	const marked = "SELECT u.ema| FROM users u"
	got := at(t, marked)

	sql := strings.Replace(marked, "|", "", 1)
	if sql[got.ReplaceFrom:got.ReplaceTo] != "ema" {
		t.Errorf("replacement range covers %q, want %q",
			sql[got.ReplaceFrom:got.ReplaceTo], "ema")
	}
}

func TestTableRefsExtractsNamesAndAliases(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []TableRef
	}{
		{
			name: "plain table",
			sql:  "SELECT * FROM users",
			want: []TableRef{{Name: "users"}},
		},
		{
			name: "table with an alias",
			sql:  "SELECT * FROM users u",
			want: []TableRef{{Name: "users", Alias: "u"}},
		},
		{
			name: "explicit AS",
			sql:  "SELECT * FROM users AS u",
			want: []TableRef{{Name: "users", Alias: "u"}},
		},
		{
			name: "schema qualified",
			sql:  "SELECT * FROM app.users u",
			want: []TableRef{{Schema: "app", Name: "users", Alias: "u"}},
		},
		{
			name: "join",
			sql:  "SELECT * FROM users u JOIN orders o ON o.user_id = u.id",
			want: []TableRef{{Name: "users", Alias: "u"}, {Name: "orders", Alias: "o"}},
		},
		{
			name: "left join",
			sql:  "SELECT * FROM a LEFT OUTER JOIN b ON a.id = b.id",
			want: []TableRef{{Name: "a"}, {Name: "b"}},
		},
		{
			name: "comma separated",
			sql:  "SELECT * FROM a, b",
			want: []TableRef{{Name: "a"}, {Name: "b"}},
		},
		{
			name: "backtick quoted",
			sql:  "SELECT * FROM `users` `u`",
			want: []TableRef{{Name: "users", Alias: "u"}},
		},
		{
			name: "update statement",
			sql:  "UPDATE users u SET u.x = 1",
			want: []TableRef{{Name: "users", Alias: "u"}},
		},
		{
			name: "delete statement",
			sql:  "DELETE FROM users WHERE id = 1",
			want: []TableRef{{Name: "users"}},
		},
		{
			name: "insert statement",
			sql:  "INSERT INTO users (id) VALUES (1)",
			want: []TableRef{{Name: "users"}},
		},
		// A keyword that follows the table must not be taken for an alias.
		{
			name: "where is not an alias",
			sql:  "SELECT * FROM users WHERE id = 1",
			want: []TableRef{{Name: "users"}},
		},
		{
			name: "order by is not an alias",
			sql:  "SELECT * FROM users ORDER BY id",
			want: []TableRef{{Name: "users"}},
		},
		{
			name: "join is not an alias",
			sql:  "SELECT * FROM a JOIN b ON a.id = b.id",
			want: []TableRef{{Name: "a"}, {Name: "b"}},
		},
		{
			name: "group by is not an alias",
			sql:  "SELECT * FROM users GROUP BY id",
			want: []TableRef{{Name: "users"}},
		},
		{
			name: "limit is not an alias",
			sql:  "SELECT * FROM users LIMIT 10",
			want: []TableRef{{Name: "users"}},
		},
		// Subqueries contribute their alias, not their contents.
		{
			name: "derived table",
			sql:  "SELECT * FROM (SELECT id FROM users) s",
			want: []TableRef{{Alias: "s", Derived: true}},
		},
		{
			name: "no tables at all",
			sql:  "SELECT 1",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TableRefs(Parse(tt.sql))

			if len(got) != len(tt.want) {
				t.Fatalf("TableRefs(%q) = %+v, want %+v", tt.sql, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("TableRefs(%q)[%d] = %+v, want %+v", tt.sql, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// The completion context carries the statement's tables so the engine can
// resolve a qualifier without parsing again.
func TestCompletionContextCarriesTheStatementTables(t *testing.T) {
	got := at(t, "SELECT u.| FROM users u JOIN orders o ON o.user_id = u.id")

	if len(got.Tables) != 2 {
		t.Fatalf("Tables = %+v, want two entries", got.Tables)
	}
	if got.Tables[0].Alias != "u" || got.Tables[1].Alias != "o" {
		t.Errorf("Tables = %+v, want aliases u and o", got.Tables)
	}
}

// Resolve maps a qualifier onto the table it stands for.
func TestResolveQualifier(t *testing.T) {
	refs := []TableRef{
		{Schema: "app", Name: "users", Alias: "u"},
		{Name: "orders", Alias: "o"},
		{Name: "audit"},
	}

	tests := []struct {
		qualifier string
		want      TableRef
		wantOK    bool
	}{
		{qualifier: "u", want: refs[0], wantOK: true},
		{qualifier: "o", want: refs[1], wantOK: true},
		// A table with no alias answers to its own name.
		{qualifier: "audit", want: refs[2], wantOK: true},
		// Case-insensitive, as MySQL identifiers are on most systems.
		{qualifier: "U", want: refs[0], wantOK: true},
		{qualifier: "nope", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.qualifier, func(t *testing.T) {
			got, ok := ResolveQualifier(refs, tt.qualifier)
			if ok != tt.wantOK {
				t.Fatalf("ResolveQualifier(%q) ok = %v, want %v", tt.qualifier, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("ResolveQualifier(%q) = %+v, want %+v", tt.qualifier, got, tt.want)
			}
		})
	}
}

// Completion runs on every keystroke, so it must never panic on a caret in
// an odd place.
func TestCompletionAtToleratesAnyOffset(t *testing.T) {
	sources := []string{
		"",
		"SELECT",
		"SELECT * FROM users u WHERE u.id = 1",
		"SELECT '한글' FROM 테이블",
		"SELECT * FROM (SELECT 1) x",
		"-- only a comment",
	}

	for _, sql := range sources {
		for offset := -2; offset <= len(sql)+2; offset++ {
			ctx := CompletionAt(sql, offset)
			if ctx.ReplaceFrom < 0 || ctx.ReplaceTo < ctx.ReplaceFrom || ctx.ReplaceTo > len(sql) {
				t.Fatalf("CompletionAt(%q, %d) produced range [%d,%d), outside [0,%d]",
					sql, offset, ctx.ReplaceFrom, ctx.ReplaceTo, len(sql))
			}
		}
	}
}
