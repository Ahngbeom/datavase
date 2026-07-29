package sqlparse

import (
	"strings"
	"testing"
)

func statementTexts(stmts []Statement) []string {
	out := make([]string, len(stmts))
	for i, s := range stmts {
		out[i] = s.SQL
	}
	return out
}

func TestSplitSeparatesStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "two statements",
			sql:  "SELECT 1; SELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name: "trailing semicolon",
			sql:  "SELECT 1;",
			want: []string{"SELECT 1"},
		},
		{
			name: "blank statements are dropped",
			sql:  ";;SELECT 1;;;",
			want: []string{"SELECT 1"},
		},
		{
			name: "semicolon inside a string literal",
			sql:  `SELECT 'a;b'; SELECT 2`,
			want: []string{`SELECT 'a;b'`, "SELECT 2"},
		},
		{
			name: "semicolon inside a backtick identifier",
			sql:  "SELECT `a;b` FROM t; SELECT 2",
			want: []string{"SELECT `a;b` FROM t", "SELECT 2"},
		},
		{
			name: "semicolon inside a line comment",
			sql:  "SELECT 1 -- ; not a separator\n; SELECT 2",
			want: []string{"SELECT 1 -- ; not a separator", "SELECT 2"},
		},
		{
			name: "semicolon inside a block comment",
			sql:  "SELECT /* ; */ 1; SELECT 2",
			want: []string{"SELECT /* ; */ 1", "SELECT 2"},
		},
		{
			name: "multiline statement keeps its shape",
			sql:  "SELECT id\nFROM users\nWHERE id = 1;",
			want: []string{"SELECT id\nFROM users\nWHERE id = 1"},
		},
		{
			name: "comment only input yields nothing",
			sql:  "-- just a note\n",
			want: nil,
		},
		{
			name: "empty input",
			sql:  "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statementTexts(Split(tt.sql))
			if len(got) != len(tt.want) {
				t.Fatalf("Split(%q) = %q, want %q", tt.sql, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Split(%q)[%d] = %q, want %q", tt.sql, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSplitRecordsSpans(t *testing.T) {
	const sql = "SELECT 1; SELECT 2"
	stmts := Split(sql)

	if len(stmts) != 2 {
		t.Fatalf("len(Split()) = %d, want 2", len(stmts))
	}
	for i, s := range stmts {
		if sql[s.Pos:s.End] != s.SQL {
			t.Errorf("statement %d: sql[%d:%d] = %q, want %q", i, s.Pos, s.End, sql[s.Pos:s.End], s.SQL)
		}
	}
}

// Ctrl+Enter runs the statement under the cursor, so the mapping from a
// caret offset to a statement has to be exact at the boundaries.
func TestStatementAtCursor(t *testing.T) {
	const sql = "SELECT 1;\nSELECT 2;\n"

	tests := []struct {
		name   string
		offset int
		want   string
	}{
		{name: "start of first", offset: 0, want: "SELECT 1"},
		{name: "inside first", offset: 4, want: "SELECT 1"},
		{name: "end of first", offset: 8, want: "SELECT 1"},
		{name: "in the whitespace after the first", offset: 9, want: "SELECT 1"},
		{name: "at the start of the second", offset: 10, want: "SELECT 2"},
		{name: "inside second", offset: 12, want: "SELECT 2"},
		{name: "trailing newline falls back to the last", offset: 20, want: "SELECT 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := StatementAt(sql, tt.offset)
			if !ok {
				t.Fatalf("StatementAt(%d) not found", tt.offset)
			}
			if got.SQL != tt.want {
				t.Errorf("StatementAt(%d) = %q, want %q", tt.offset, got.SQL, tt.want)
			}
		})
	}
}

func TestStatementAtWithNoStatements(t *testing.T) {
	if _, ok := StatementAt("   \n-- nothing here\n", 3); ok {
		t.Error("StatementAt() found a statement in comment-only input")
	}
}

func TestStatementKind(t *testing.T) {
	tests := []struct {
		sql  string
		want StmtKind
	}{
		{"SELECT 1", StmtSelect},
		{"  \n select 1", StmtSelect},
		{"-- note\nSELECT 1", StmtSelect},
		{"/* note */ SELECT 1", StmtSelect},
		{"(SELECT 1) UNION (SELECT 2)", StmtSelect},
		{"WITH x AS (SELECT 1) SELECT * FROM x", StmtSelect},
		{"TABLE users", StmtSelect},
		{"INSERT INTO t VALUES (1)", StmtInsert},
		{"REPLACE INTO t VALUES (1)", StmtInsert},
		{"UPDATE t SET x = 1", StmtUpdate},
		{"DELETE FROM t", StmtDelete},
		{"TRUNCATE TABLE t", StmtTruncate},
		{"TRUNCATE t", StmtTruncate},
		{"DROP TABLE t", StmtDrop},
		{"DROP DATABASE d", StmtDrop},
		{"CREATE TABLE t (id INT)", StmtDDL},
		{"ALTER TABLE t ADD COLUMN x INT", StmtDDL},
		{"RENAME TABLE a TO b", StmtDDL},
		{"SHOW TABLES", StmtRead},
		{"DESCRIBE users", StmtRead},
		{"EXPLAIN SELECT 1", StmtRead},
		{"USE app_db", StmtSession},
		{"SET autocommit = 1", StmtSession},
		{"GRANT ALL ON *.* TO u", StmtOther},
		{"", StmtOther},
		{"/*! DELETE FROM t */", StmtDelete},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			stmt := Parse(tt.sql)
			if got := stmt.Kind(); got != tt.want {
				t.Errorf("Parse(%q).Kind() = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

// This is the check that decides whether a DELETE is a routine edit or a
// table wipe, so the subquery cases have to be exact.
func TestHasTopLevelWhere(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{
			name: "plain where",
			sql:  "DELETE FROM t WHERE id = 1",
			want: true,
		},
		{
			name: "no where at all",
			sql:  "DELETE FROM t",
			want: false,
		},
		{
			name: "where only inside a subquery predicate",
			sql:  "UPDATE t SET x = (SELECT y FROM u WHERE u.id = 1)",
			want: false,
		},
		{
			name: "top level where with a subquery after it",
			sql:  "DELETE FROM t WHERE id IN (SELECT id FROM u)",
			want: true,
		},
		{
			name: "where inside a nested subquery only",
			sql:  "DELETE FROM t USING (SELECT id FROM u WHERE x = 1) s",
			want: false,
		},
		{
			name: "where appearing in a string literal",
			sql:  "DELETE FROM t /* WHERE */ ",
			want: false,
		},
		{
			name: "where as a quoted column name",
			sql:  "DELETE FROM t ORDER BY `where`",
			want: false,
		},
		{
			name: "where in a string value",
			sql:  "INSERT INTO log VALUES ('WHERE')",
			want: false,
		},
		{
			name: "lowercase where",
			sql:  "delete from t where id = 1",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.sql).HasTopLevelWhere(); got != tt.want {
				t.Errorf("Parse(%q).HasTopLevelWhere() = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func TestHasTopLevelLimit(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{name: "plain limit", sql: "SELECT * FROM t LIMIT 10", want: true},
		{name: "no limit", sql: "SELECT * FROM t", want: false},
		{name: "limit only in a subquery", sql: "SELECT * FROM (SELECT 1 LIMIT 1) s", want: false},
		{name: "limit with offset", sql: "SELECT * FROM t LIMIT 10, 20", want: true},
		{name: "limit as a quoted column", sql: "SELECT `limit` FROM t", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.sql).HasTopLevelLimit(); got != tt.want {
				t.Errorf("Parse(%q).HasTopLevelLimit() = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func TestStatementIsEmpty(t *testing.T) {
	for _, sql := range []string{"", "   ", "-- note", "/* block */"} {
		t.Run(sql, func(t *testing.T) {
			if !Parse(sql).IsEmpty() {
				t.Errorf("Parse(%q).IsEmpty() = false, want true", sql)
			}
		})
	}
	if Parse("SELECT 1").IsEmpty() {
		t.Error("Parse(\"SELECT 1\").IsEmpty() = true, want false")
	}
}

func TestSplitHandlesLargeInputWithoutQuadraticBlowup(t *testing.T) {
	sql := strings.Repeat("SELECT 1;\n", 20000)

	if got := len(Split(sql)); got != 20000 {
		t.Fatalf("len(Split()) = %d, want 20000", got)
	}
}
