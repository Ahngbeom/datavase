package complete

import (
	"context"
	"strings"
	"testing"
)

// fakeCatalog stands in for the SQLite cache. The engine is pure logic on top
// of a lookup, so this keeps its tests free of a database.
type fakeCatalog struct {
	schemas map[string][]string // datasource -> schemas
	tables  map[string][]string // "ds.schema" -> tables
	columns map[string][]string // "ds.schema.table" -> columns
	calls   int                 // how many lookups were made
	err     error               // when set, every lookup fails
}

func (f *fakeCatalog) Schemas(_ context.Context, ds string) ([]string, error) {
	f.calls++
	return f.schemas[ds], f.err
}

func (f *fakeCatalog) Tables(_ context.Context, ds, schema string) ([]string, error) {
	f.calls++
	return f.tables[ds+"."+schema], f.err
}

func (f *fakeCatalog) Columns(_ context.Context, ds, schema, table string) ([]string, error) {
	f.calls++
	return f.columns[ds+"."+schema+"."+table], f.err
}

func testCatalog() *fakeCatalog {
	return &fakeCatalog{
		schemas: map[string][]string{"ds": {"app_db", "log_db"}},
		tables: map[string][]string{
			"ds.app_db": {"users", "user_roles", "orders"},
			"ds.log_db": {"events"},
		},
		columns: map[string][]string{
			"ds.app_db.users":      {"id", "email", "created_at"},
			"ds.app_db.orders":     {"id", "user_id", "total"},
			"ds.app_db.user_roles": {"user_id", "role"},
		},
	}
}

func newEngine(t *testing.T) *Engine {
	t.Helper()
	return New(testCatalog(), "ds", "app_db")
}

// suggest runs completion with "|" marking the caret.
func suggest(t *testing.T, e *Engine, marked string) []Candidate {
	t.Helper()

	offset := strings.Index(marked, "|")
	if offset < 0 {
		t.Fatalf("test SQL %q has no caret marker", marked)
	}

	got, err := e.Suggest(context.Background(), strings.Replace(marked, "|", "", 1), offset)
	if err != nil {
		t.Fatalf("Suggest(%q) error = %v, want nil", marked, err)
	}
	return got
}

func names(candidates []Candidate) []string {
	out := make([]string, len(candidates))
	for i, c := range candidates {
		out[i] = c.Text
	}
	return out
}

func TestSuggestsTablesAfterFrom(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT * FROM |"))

	for _, want := range []string{"users", "user_roles", "orders"} {
		if !contains(got, want) {
			t.Errorf("Suggest() = %v, want it to include %q", got, want)
		}
	}
}

func TestSuggestsColumnsOfTablesInScope(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT | FROM users"))

	for _, want := range []string{"id", "email", "created_at"} {
		if !contains(got, want) {
			t.Errorf("Suggest() = %v, want it to include %q", got, want)
		}
	}
	// A column of a table that is not in the statement must not appear.
	if contains(got, "role") {
		t.Errorf("Suggest() = %v, want no columns from tables outside the statement", got)
	}
}

func TestSuggestsColumnsOfEveryJoinedTable(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT | FROM users u JOIN orders o ON o.user_id = u.id"))

	if !contains(got, "email") {
		t.Errorf("Suggest() = %v, want a column of users", got)
	}
	if !contains(got, "total") {
		t.Errorf("Suggest() = %v, want a column of orders", got)
	}
}

// The point of alias resolution: "o." offers only orders' columns.
func TestQualifiedCompletionUsesTheAlias(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT o.| FROM users u JOIN orders o ON o.user_id = u.id"))

	if !contains(got, "total") {
		t.Errorf("Suggest() = %v, want orders' columns", got)
	}
	if contains(got, "email") {
		t.Errorf("Suggest() = %v, want no columns from users behind the o. qualifier", got)
	}
}

func TestQualifiedCompletionUsesTheTableName(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT users.| FROM users"))

	if !contains(got, "email") {
		t.Errorf("Suggest() = %v, want users' columns", got)
	}
}

// A schema name before the dot lists that schema's tables instead.
func TestQualifiedCompletionFallsBackToSchema(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT * FROM log_db.|"))

	if !contains(got, "events") {
		t.Errorf("Suggest() = %v, want log_db's tables", got)
	}
}

func TestPrefixFilters(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT * FROM user|"))

	for _, want := range []string{"users", "user_roles"} {
		if !contains(got, want) {
			t.Errorf("Suggest() = %v, want it to include %q", got, want)
		}
	}
	if contains(got, "orders") {
		t.Errorf("Suggest() = %v, want the prefix to exclude orders", got)
	}
}

// Typing in either case must find the name; SQL identifiers are not
// case-sensitive on most MySQL installations.
func TestPrefixIsCaseInsensitive(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT * FROM USER|"))

	if !contains(got, "users") {
		t.Errorf("Suggest() = %v, want a case-insensitive prefix match", got)
	}
}

// An exact prefix match should come before a match further in.
func TestPrefixMatchesRankAheadOfSubstringMatches(t *testing.T) {
	e := New(&fakeCatalog{
		schemas: map[string][]string{"ds": {"app_db"}},
		tables:  map[string][]string{"ds.app_db": {"my_user", "users"}},
	}, "ds", "app_db")

	got := names(suggest(t, e, "SELECT * FROM user|"))
	if len(got) < 2 {
		t.Fatalf("Suggest() = %v, want both tables", got)
	}
	if got[0] != "users" {
		t.Errorf("Suggest() = %v, want the prefix match %q first", got, "users")
	}
}

// Keywords help far more than they hurt when nothing has been typed yet.
func TestSuggestsKeywords(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT * FROM users WHERE id = 1 ORD|"))

	if !contains(got, "ORDER BY") {
		t.Errorf("Suggest() = %v, want the ORDER BY keyword", got)
	}
}

// Kinds drive the popup's labels. Keywords are offered alongside names, so
// the check is on the named entries rather than on every candidate.
func TestCandidateKindsAreLabelled(t *testing.T) {
	if got := kindOf(t, suggest(t, newEngine(t), "SELECT * FROM |"), "users"); got != KindTable {
		t.Errorf("users has kind %v, want KindTable", got)
	}
	if got := kindOf(t, suggest(t, newEngine(t), "SELECT | FROM users"), "email"); got != KindColumn {
		t.Errorf("email has kind %v, want KindColumn", got)
	}

	// Behind a qualifier only columns make sense, so nothing else may appear.
	for _, c := range suggest(t, newEngine(t), "SELECT u.| FROM users u") {
		if c.Kind != KindColumn {
			t.Errorf("candidate %q has kind %v, want only columns behind a qualifier", c.Text, c.Kind)
		}
	}
}

func kindOf(t *testing.T, candidates []Candidate, text string) Kind {
	t.Helper()

	for _, c := range candidates {
		if c.Text == text {
			return c.Kind
		}
	}
	t.Fatalf("no candidate named %q in %v", text, names(candidates))
	return KindTable
}

// Candidates carry the range they replace so the editor can apply one.
func TestCandidatesCarryTheReplacementRange(t *testing.T) {
	const marked = "SELECT * FROM user|"
	sql := strings.Replace(marked, "|", "", 1)

	got, err := newEngine(t).Suggest(context.Background(), sql, strings.Index(marked, "|"))
	if err != nil {
		t.Fatalf("Suggest() error = %v", err)
	}
	if len(got) == 0 {
		t.Fatal("Suggest() returned nothing")
	}

	c := got[0]
	if sql[c.ReplaceFrom:c.ReplaceTo] != "user" {
		t.Errorf("replacement range covers %q, want %q", sql[c.ReplaceFrom:c.ReplaceTo], "user")
	}
}

// Nothing sensible to offer must be an empty list, not an error.
func TestNoContextYieldsNothing(t *testing.T) {
	for _, marked := range []string{"|", "SELECT 'abc|'", "-- comment |"} {
		t.Run(marked, func(t *testing.T) {
			if got := suggest(t, newEngine(t), marked); len(got) != 0 {
				t.Errorf("Suggest(%q) = %v, want nothing", marked, names(got))
			}
		})
	}
}

// A cold or broken cache must not break typing. Names disappear, but
// keywords do not depend on the catalog and stay useful — the first run,
// before any schema has been fetched, is exactly when that matters.
func TestCatalogFailureStillOffersKeywords(t *testing.T) {
	broken := testCatalog()
	broken.err = context.DeadlineExceeded
	e := New(broken, "ds", "app_db")

	got, err := e.Suggest(context.Background(), "SELECT * FROM ", 14)
	if err != nil {
		t.Fatalf("Suggest() error = %v, want nil even when the catalog fails", err)
	}

	for _, c := range got {
		if c.Kind != KindKeyword {
			t.Errorf("candidate %q survived a catalog failure, want keywords only", c.Text)
		}
	}
	if len(got) == 0 {
		t.Error("Suggest() = nothing, want the keywords that need no catalog")
	}
}

// Behind a qualifier there is nothing sensible to offer without the catalog,
// so a failure there really does mean an empty list.
func TestCatalogFailureBehindAQualifierYieldsNothing(t *testing.T) {
	broken := testCatalog()
	broken.err = context.DeadlineExceeded
	e := New(broken, "ds", "app_db")

	got := suggest(t, e, "SELECT u.| FROM users u")
	if len(got) != 0 {
		t.Errorf("Suggest() = %v, want nothing", names(got))
	}
}

// Duplicate column names across joined tables should appear once.
func TestDuplicateColumnNamesAreCollapsed(t *testing.T) {
	got := names(suggest(t, newEngine(t), "SELECT | FROM users u JOIN orders o ON o.user_id = u.id"))

	seen := 0
	for _, name := range got {
		if name == "id" {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("the column %q appears %d times, want once: %v", "id", seen, got)
	}
}

// A limit keeps the popup usable on a schema with thousands of tables.
func TestResultsAreLimited(t *testing.T) {
	many := make([]string, 500)
	for i := range many {
		many[i] = "table_" + string(rune('a'+i%26)) + itoa(i)
	}
	e := New(&fakeCatalog{
		schemas: map[string][]string{"ds": {"app_db"}},
		tables:  map[string][]string{"ds.app_db": many},
	}, "ds", "app_db")

	got := suggest(t, e, "SELECT * FROM |")
	if len(got) > MaxCandidates {
		t.Errorf("Suggest() returned %d candidates, want at most %d", len(got), MaxCandidates)
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
