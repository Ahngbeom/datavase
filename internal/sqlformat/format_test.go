package sqlformat

import (
	"strings"
	"testing"
)

func format(t *testing.T, sql string) string {
	t.Helper()
	out, err := Format(sql)
	if err != nil {
		t.Fatalf("Format(%q) error = %v", sql, err)
	}
	return out
}

func lines(s ...string) string { return strings.Join(s, "\n") }

func TestAOneLineQueryIsLaidOutByClause(t *testing.T) {
	got := format(t, "select a, b from t left join u on u.id = t.uid where x = 1 and y between 1 and 2 group by a order by b desc limit 10")
	want := lines(
		"SELECT a,",
		"    b",
		"FROM t",
		"LEFT JOIN u ON u.id = t.uid",
		"WHERE x = 1",
		"    AND y BETWEEN 1 AND 2",
		"GROUP BY a",
		"ORDER BY b DESC",
		"LIMIT 10",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

func TestASubqueryIsIndentedInsideItsParentheses(t *testing.T) {
	got := format(t, "SELECT * FROM a WHERE id IN (SELECT id FROM b WHERE c = 1 AND d = 2)")
	want := lines(
		"SELECT *",
		"FROM a",
		"WHERE id IN (",
		"    SELECT id",
		"    FROM b",
		"    WHERE c = 1",
		"        AND d = 2",
		")",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

// Parentheses that are not a subquery — function calls, window clauses — hold
// keywords that are not clauses. Breaking there would split COUNT(DISTINCT x)
// or GROUP_CONCAT(x ORDER BY y) across lines.
func TestKeywordsInsideAFunctionCallStayOnTheLine(t *testing.T) {
	got := format(t, "SELECT group_concat(x ORDER BY y SEPARATOR ','), count(DISTINCT z), left(name, 3) FROM t")
	want := lines(
		"SELECT group_concat(x ORDER BY y SEPARATOR ','),",
		"    count(DISTINCT z),",
		"    LEFT(name, 3)",
		"FROM t",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

// Literals and quoted identifiers are copied byte for byte: a reformat that
// touched the inside of a string would change the data the query writes.
func TestLiteralsAndQuotedNamesAreNeverRewritten(t *testing.T) {
	got := format(t, "select 'from  where\nselect', `select` from t")
	want := lines(
		"SELECT 'from  where\nselect',",
		"    `select`",
		"FROM t",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

// After a dot a reserved word is a column name; upper-casing it is harmless,
// but breaking the line there would make t.from read as a clause.
func TestAReservedWordAfterADotIsAName(t *testing.T) {
	got := format(t, "select t.from from t")
	want := lines(
		"SELECT t.from",
		"FROM t",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

// A line comment runs to the end of its line, so whatever followed it must
// still start a new line — otherwise the formatter comments the query out.
func TestALineCommentStillEndsItsLine(t *testing.T) {
	got := format(t, "select a, -- the id\n b from t")
	want := lines(
		"SELECT a, -- the id",
		"    b",
		"FROM t",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

func TestStatementsAreSeparatedByABlankLine(t *testing.T) {
	got := format(t, "select 1; select 2;")
	want := lines(
		"SELECT 1;",
		"",
		"SELECT 2;",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

// Spacing the author chose inside an expression is kept: the tokenizer splits
// >= and 1e5 into two tokens, and adding a space between them would change
// what the server reads.
func TestTokensWrittenTogetherStayTogether(t *testing.T) {
	got := format(t, "select 1e5 from t where a>=1 and b - -1 = 0")
	want := lines(
		"SELECT 1e5",
		"FROM t",
		"WHERE a>=1",
		"    AND b - -1 = 0",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

func TestUpdateAndInsertAreLaidOutToo(t *testing.T) {
	got := format(t, "update t set a = 1, b = 2 where id = 3")
	want := lines(
		"UPDATE t",
		"SET a = 1, b = 2",
		"WHERE id = 3",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}

	got = format(t, "insert into t (a, b) values (1, 2) on duplicate key update a = values(a)")
	want = lines(
		"INSERT INTO t (a, b)",
		"VALUES (1, 2)",
		"ON DUPLICATE KEY UPDATE a = VALUES(a)",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

func TestCaseEndIsUpperCasedOnlyAsTheEndOfACase(t *testing.T) {
	got := format(t, "select case when a then 1 else 2 end, end from t")
	want := lines(
		"SELECT CASE WHEN a THEN 1 ELSE 2 END,",
		"    end",
		"FROM t",
	)
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

// MySQL executes the inside of /*! ... */, and the tokenizer unwraps it to
// say so. Rebuilding text from those tokens would delete the comment markers
// and run version-gated code on every server.
func TestAVersionCommentIsRefusedRatherThanUnwrapped(t *testing.T) {
	if _, err := Format("select /*!40001 SQL_NO_CACHE */ * from t"); err == nil {
		t.Error("Format() reformatted a /*! comment instead of refusing")
	}
}

func TestSurroundingWhitespaceIsKept(t *testing.T) {
	got := format(t, "\n  select 1 from t\n")
	want := "\n  SELECT 1\nFROM t\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormattingIsStable(t *testing.T) {
	once := format(t, "select a, b from t where x in (select y from u) and z = 1 order by a")
	if twice := format(t, once); twice != once {
		t.Errorf("formatting twice changed the result:\n%s\nthen\n%s", once, twice)
	}
}

// The caret is carried to the same character, so reformatting does not throw
// the user somewhere else in the query.
func TestOffsetFollowsTheSameCharacter(t *testing.T) {
	before := "select a, b from t"
	after := format(t, before)

	at := strings.Index(before, "from")
	got := Offset(before, after, at)
	if !strings.HasPrefix(after[got:], "FROM") {
		t.Errorf("Offset() landed on %q, want the start of FROM", after[got:])
	}
}

// The self-check is the last line of defence against a layout bug: joining
// two words, or touching a literal, must be refused rather than returned.
func TestTheSelfCheckRejectsAnythingButWhitespaceAndCase(t *testing.T) {
	tests := []struct{ name, before, after string }{
		{name: "two words joined", before: "select a from t", after: "SELECT a FROMt"},
		{name: "a literal re-cased", before: "select 'a' from t", after: "SELECT 'A' FROM t"},
		{name: "a version comment unwrapped", before: "select /*! 1 */", after: "SELECT 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if sameStatement(tt.before, tt.after) {
				t.Errorf("sameStatement(%q, %q) = true", tt.before, tt.after)
			}
		})
	}
	if !sameStatement("select a\nfrom t", "SELECT a FROM t") {
		t.Error("sameStatement() rejected a change of whitespace and case")
	}
}
