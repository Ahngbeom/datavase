package history

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	s, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func record(sql string) Entry {
	return Entry{
		DataSource: "prod-app",
		SQL:        sql,
		Rows:       3,
		Elapsed:    14 * time.Millisecond,
		At:         time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC),
	}
}

func TestAddAndSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, sql := range []string{
		"SELECT * FROM users",
		"SELECT id FROM orders",
		"UPDATE users SET x = 1 WHERE id = 2",
	} {
		if err := s.Add(ctx, record(sql)); err != nil {
			t.Fatalf("Add(%q) error = %v", sql, err)
		}
	}

	got, err := s.Search(ctx, "users", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Search(users) returned %d entries, want 2: %v", len(got), sqlOf(got))
	}
}

// The newest entry is the one most likely wanted again.
func TestSearchReturnsNewestFirst(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	base := record("SELECT 1")
	for i, sql := range []string{"SELECT 1", "SELECT 2", "SELECT 3"} {
		e := base
		e.SQL = sql
		e.At = base.At.Add(time.Duration(i) * time.Minute)
		if err := s.Add(ctx, e); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	got, err := s.Search(ctx, "SELECT", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 3 || got[0].SQL != "SELECT 3" {
		t.Errorf("Search() = %v, want the newest first", sqlOf(got))
	}
}

// An empty search is the plain history list.
func TestSearchWithNoTermReturnsEverything(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Add(ctx, record("SELECT 1"))
	s.Add(ctx, record("SELECT 2"))

	got, err := s.Search(ctx, "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Search(\"\") returned %d entries, want 2", len(got))
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.Add(ctx, record("SELECT * FROM Customers"))

	got, err := s.Search(ctx, "customers", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Search() returned %d entries, want a case-insensitive match", len(got))
	}
}

// Wildcards typed by the user are text, not pattern syntax.
func TestSearchTreatsWildcardsAsLiteralText(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Add(ctx, record("SELECT * FROM user_roles"))
	s.Add(ctx, record("SELECT * FROM userXroles"))

	got, err := s.Search(ctx, "user_roles", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Search(user_roles) = %v, want only the literal match", sqlOf(got))
	}
}

// Running the same query repeatedly should not fill the list with copies.
func TestRepeatedQueriesAreCollapsed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		e := record("SELECT * FROM users")
		e.At = e.At.Add(time.Duration(i) * time.Minute)
		if err := s.Add(ctx, e); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	got, err := s.Search(ctx, "users", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Search() returned %d entries, want repeats collapsed: %v", len(got), sqlOf(got))
	}
	// The surviving entry carries the most recent run.
	if !got[0].At.Equal(record("").At.Add(2 * time.Minute)) {
		t.Errorf("At = %v, want the latest run", got[0].At)
	}
}

// A statement carrying a password must never be written to disk. Storing it
// would defeat the point of keeping credentials in the keychain.
func TestStatementsWithCredentialsAreNotStored(t *testing.T) {
	sensitive := []string{
		"CREATE USER 'app'@'%' IDENTIFIED BY 'hunter2'",
		"ALTER USER 'app'@'%' IDENTIFIED BY 'hunter2'",
		"SET PASSWORD FOR 'app'@'%' = 'hunter2'",
		"GRANT ALL ON *.* TO 'app'@'%' IDENTIFIED BY 'hunter2'",
		"create user x identified by 'secret'",
	}

	for _, sql := range sensitive {
		t.Run(sql, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()

			if err := s.Add(ctx, record(sql)); err != nil {
				t.Fatalf("Add() error = %v, want nil", err)
			}

			got, err := s.Search(ctx, "", 10)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(got) != 0 {
				t.Errorf("a statement containing a password was stored: %v", sqlOf(got))
			}
		})
	}
}

func TestOrdinaryStatementsAreStored(t *testing.T) {
	ordinary := []string{
		"SELECT * FROM users WHERE password_hash IS NOT NULL",
		"UPDATE settings SET identified_by = 'x'",
		"SELECT 'identified by' AS note",
	}

	for _, sql := range ordinary {
		t.Run(sql, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()

			if err := s.Add(ctx, record(sql)); err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			got, _ := s.Search(ctx, "", 10)
			if len(got) != 1 {
				t.Errorf("an ordinary statement was dropped: %q", sql)
			}
		})
	}
}

func TestLimitIsRespected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		e := record("SELECT " + strings.Repeat("x", i+1))
		e.At = e.At.Add(time.Duration(i) * time.Minute)
		s.Add(ctx, e)
	}

	got, err := s.Search(ctx, "", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got) != 5 {
		t.Errorf("Search(limit 5) returned %d entries", len(got))
	}
}

// Entries carry enough context to be useful when read back.
func TestEntriesKeepTheirMetadata(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Add(ctx, record("SELECT 1")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got, _ := s.Search(ctx, "", 10)
	if got[0].DataSource != "prod-app" {
		t.Errorf("DataSource = %q, want %q", got[0].DataSource, "prod-app")
	}
	if got[0].Rows != 3 {
		t.Errorf("Rows = %d, want 3", got[0].Rows)
	}
	if got[0].Elapsed != 14*time.Millisecond {
		t.Errorf("Elapsed = %v, want 14ms", got[0].Elapsed)
	}
}

// Whitespace-only statements are noise.
func TestBlankStatementsAreIgnored(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, sql := range []string{"", "   ", "\n\t"} {
		if err := s.Add(ctx, record(sql)); err != nil {
			t.Fatalf("Add(%q) error = %v", sql, err)
		}
	}

	got, _ := s.Search(ctx, "", 10)
	if len(got) != 0 {
		t.Errorf("blank statements were stored: %v", sqlOf(got))
	}
}

// History outlives the process.
func TestHistorySurvivesReopening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	ctx := context.Background()

	first, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	first.Add(ctx, record("SELECT * FROM users"))
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer second.Close()

	got, _ := second.Search(ctx, "users", 10)
	if len(got) != 1 {
		t.Errorf("history did not survive reopening: %v", sqlOf(got))
	}
}

func sqlOf(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.SQL
	}
	return out
}
