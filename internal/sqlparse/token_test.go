package sqlparse

import (
	"strings"
	"testing"
)

// summarise renders tokens as "kind:text" so tests read as a single line.
func summarise(toks []Token) string {
	parts := make([]string, len(toks))
	for i, tk := range toks {
		parts[i] = tk.Kind.String() + ":" + tk.Text
	}
	return strings.Join(parts, " ")
}

func TestTokenizeBasicStatement(t *testing.T) {
	got := summarise(Tokenize("SELECT id FROM users"))
	want := "word:SELECT word:id word:FROM word:users"
	if got != want {
		t.Errorf("Tokenize() = %q, want %q", got, want)
	}
}

func TestTokenizeClassifiesLiteralsAndIdentifiers(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "single quoted string",
			sql:  `WHERE name = 'bahn'`,
			want: `word:WHERE word:name punct:= string:'bahn'`,
		},
		{
			name: "double quoted string",
			sql:  `WHERE name = "bahn"`,
			want: `word:WHERE word:name punct:= string:"bahn"`,
		},
		{
			name: "backtick identifier",
			sql:  "SELECT `from` FROM t",
			want: "word:SELECT ident:`from` word:FROM word:t",
		},
		{
			name: "doubled quote escape",
			sql:  `'it''s'`,
			want: `string:'it''s'`,
		},
		{
			name: "backslash quote escape",
			sql:  `'it\'s'`,
			want: `string:'it\'s'`,
		},
		{
			name: "backtick doubled escape",
			sql:  "`we``ird`",
			want: "ident:`we``ird`",
		},
		{
			name: "number",
			sql:  "LIMIT 100",
			want: "word:LIMIT number:100",
		},
		{
			name: "decimal number",
			sql:  "1.5",
			want: "number:1.5",
		},
		{
			name: "qualified name",
			sql:  "app.users",
			want: "word:app punct:. word:users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summarise(Tokenize(tt.sql)); got != tt.want {
				t.Errorf("Tokenize(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

func TestTokenizeComments(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "double dash to end of line",
			sql:  "SELECT 1 -- trailing\nFROM t",
			want: "word:SELECT number:1 comment:-- trailing word:FROM word:t",
		},
		{
			name: "hash to end of line",
			sql:  "SELECT 1 # trailing\nFROM t",
			want: "word:SELECT number:1 comment:# trailing word:FROM word:t",
		},
		{
			name: "block comment",
			sql:  "SELECT /* hidden */ 1",
			want: "word:SELECT comment:/* hidden */ number:1",
		},
		{
			name: "unterminated block comment",
			sql:  "SELECT /* never closed",
			want: "word:SELECT comment:/* never closed",
		},
		{
			name: "minus operator is not a comment",
			sql:  "1-2",
			want: "number:1 punct:- number:2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summarise(Tokenize(tt.sql)); got != tt.want {
				t.Errorf("Tokenize(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

// MySQL executes the contents of /*! ... */ version-hint comments. Treating
// them as ordinary comments would let a destructive statement past guard
// while the server ran it.
func TestTokenizeExecutableCommentsExposeTheirContents(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "bare executable comment",
			sql:  "/*! DELETE FROM users */",
			want: "word:DELETE word:FROM word:users",
		},
		{
			name: "versioned executable comment",
			sql:  "/*!40001 DELETE FROM users */",
			want: "word:DELETE word:FROM word:users",
		},
		{
			name: "optimizer hint is a real comment",
			sql:  "SELECT /*+ MAX_EXECUTION_TIME(1) */ 1",
			want: "word:SELECT comment:/*+ MAX_EXECUTION_TIME(1) */ number:1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summarise(Tokenize(tt.sql)); got != tt.want {
				t.Errorf("Tokenize(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}

// Depth drives the top-level WHERE check, so it must count parentheses only
// outside literals.
func TestTokenizeTracksParenthesisDepth(t *testing.T) {
	toks := Tokenize("DELETE FROM t WHERE id IN (SELECT id FROM u WHERE x)")

	depthOf := func(text string, occurrence int) int {
		t.Helper()
		seen := 0
		for _, tk := range toks {
			if strings.EqualFold(tk.Text, text) {
				seen++
				if seen == occurrence {
					return tk.Depth
				}
			}
		}
		t.Fatalf("token %q occurrence %d not found in %q", text, occurrence, summarise(toks))
		return -1
	}

	if got := depthOf("WHERE", 1); got != 0 {
		t.Errorf("outer WHERE depth = %d, want 0", got)
	}
	if got := depthOf("WHERE", 2); got != 1 {
		t.Errorf("inner WHERE depth = %d, want 1", got)
	}
	if got := depthOf("SELECT", 1); got != 1 {
		t.Errorf("subquery SELECT depth = %d, want 1", got)
	}
}

func TestTokenizeIgnoresParenthesesInsideLiterals(t *testing.T) {
	toks := Tokenize("SELECT '(' FROM t WHERE x = 1")

	for _, tk := range toks {
		if strings.EqualFold(tk.Text, "WHERE") && tk.Depth != 0 {
			t.Errorf("WHERE depth = %d, want 0; a literal parenthesis was counted", tk.Depth)
		}
	}
}

// Positions let the editor map the cursor back to a statement.
func TestTokenizeRecordsPositions(t *testing.T) {
	const sql = "SELECT id"
	toks := Tokenize(sql)

	if len(toks) != 2 {
		t.Fatalf("len(tokens) = %d, want 2: %s", len(toks), summarise(toks))
	}
	if toks[1].Pos != 7 || toks[1].End != 9 {
		t.Errorf("second token span = [%d,%d), want [7,9)", toks[1].Pos, toks[1].End)
	}
	if sql[toks[1].Pos:toks[1].End] != "id" {
		t.Errorf("slice at recorded span = %q, want %q", sql[toks[1].Pos:toks[1].End], "id")
	}
}

func TestTokenizeEmptyInput(t *testing.T) {
	if toks := Tokenize("   \n\t  "); len(toks) != 0 {
		t.Errorf("Tokenize(whitespace) = %s, want no tokens", summarise(toks))
	}
}
