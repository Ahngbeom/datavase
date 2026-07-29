// Package history records the statements a user has run, so they can be
// found again.
package history

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver: keeps the binary CGO-free
)

// Entry is one recorded statement.
type Entry struct {
	DataSource string
	SQL        string
	Rows       int
	Elapsed    time.Duration
	At         time.Time
}

// Store holds the history on disk.
type Store struct {
	db *sql.DB
}

// Open opens or creates the history at path.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	// WAL so that recording a statement never blocks a search, and vice
	// versa; both happen while the user is working.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening the query history: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

const schemaSQL = `
CREATE TABLE IF NOT EXISTS query_history (
	datasource TEXT NOT NULL,
	sql_text   TEXT NOT NULL,
	rows_count INTEGER NOT NULL DEFAULT 0,
	elapsed_ms INTEGER NOT NULL DEFAULT 0,
	ran_at     INTEGER NOT NULL,
	PRIMARY KEY (datasource, sql_text)
);

CREATE INDEX IF NOT EXISTS query_history_recent ON query_history (ran_at DESC);
`

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("preparing the query history: %w", err)
	}
	return nil
}

// Add records a statement.
//
// Re-running a statement updates the existing row rather than adding another,
// so a query run in a loop does not crowd out everything else.
func (s *Store) Add(ctx context.Context, e Entry) error {
	sqlText := strings.TrimSpace(e.SQL)
	if sqlText == "" {
		return nil
	}
	if carriesCredentials(sqlText) {
		// Recording this would put a plaintext password on disk, which is
		// exactly what keeping credentials in the keychain avoids.
		return nil
	}

	const q = `INSERT INTO query_history (datasource, sql_text, rows_count, elapsed_ms, ran_at)
	           VALUES (?, ?, ?, ?, ?)
	           ON CONFLICT (datasource, sql_text) DO UPDATE SET
	             rows_count = excluded.rows_count,
	             elapsed_ms = excluded.elapsed_ms,
	             ran_at     = excluded.ran_at`

	at := e.At
	if at.IsZero() {
		at = time.Now()
	}

	_, err := s.db.ExecContext(ctx, q,
		e.DataSource, sqlText, e.Rows, e.Elapsed.Milliseconds(), at.UnixMilli())
	if err != nil {
		return fmt.Errorf("recording the statement: %w", err)
	}
	return nil
}

// credentialStatements match statements that embed a password in their text.
//
// Each is anchored at the start so that a SELECT merely containing the words
// is not mistaken for one. SET PASSWORD needs no second condition: setting a
// password is the entire purpose of the statement.
var credentialStatements = []*regexp.Regexp{
	regexp.MustCompile(`(?is)^\s*set\s+password\b`),
	regexp.MustCompile(`(?is)^\s*(create|alter)\s+user\b.*\bidentified\s+by\b`),
	regexp.MustCompile(`(?is)^\s*grant\b.*\bidentified\s+by\b`),
}

// carriesCredentials reports whether a statement embeds a secret.
//
// It cannot be exhaustive, but the common shapes are worth catching: writing
// a plaintext password to disk would undo the reason the real ones live in
// the keychain.
func carriesCredentials(sqlText string) bool {
	for _, re := range credentialStatements {
		if re.MatchString(sqlText) {
			return true
		}
	}
	return false
}

// Search returns matching entries, newest first. An empty term lists
// everything.
func (s *Store) Search(ctx context.Context, term string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 50
	}

	// The term is text a user typed, so its wildcards are literal.
	const q = `SELECT datasource, sql_text, rows_count, elapsed_ms, ran_at
	           FROM query_history
	           WHERE sql_text LIKE ? ESCAPE '\'
	           ORDER BY ran_at DESC
	           LIMIT ?`

	rows, err := s.db.QueryContext(ctx, q, "%"+escapeLike(term)+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var (
			e         Entry
			elapsedMs int64
			ranAt     int64
		)
		if err := rows.Scan(&e.DataSource, &e.SQL, &e.Rows, &elapsedMs, &ranAt); err != nil {
			return nil, err
		}
		e.Elapsed = time.Duration(elapsedMs) * time.Millisecond
		e.At = time.UnixMilli(ranAt).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// DefaultPath returns the on-disk location, alongside the catalog cache.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "datavase", "history.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "datavase", "history.db"), nil
}

// escapeLike neutralises the wildcards in user-typed text, so searching for
// "user_roles" does not also match "userXroles".
func escapeLike(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '%', '_', '\\':
			out = append(out, '\\', s[i])
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
