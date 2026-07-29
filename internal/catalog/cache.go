package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go driver: keeps the binary CGO-free
)

// Snapshot is a datasource's schema as read from the server.
//
// Tables are keyed by schema name; columns by "schema.table". Maps keep Save
// a single call rather than an interface the caller has to drive.
type Snapshot struct {
	Schemas []string
	Tables  map[string][]Table
	Columns map[string][]Column
}

// Cache stores schema metadata locally.
//
// Completion reads from here rather than from information_schema: on a server
// with tens of thousands of tables that query takes hundreds of milliseconds,
// which is far too slow to run on a keystroke.
type Cache struct {
	db *sql.DB
}

// DefaultCachePath returns the on-disk location, honouring XDG_STATE_HOME.
func DefaultCachePath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "datavase", "datavase.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "datavase", "datavase.db"), nil
}

// OpenCache opens or creates the cache at path.
func OpenCache(path string) (*Cache, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	// WAL is what lets the completion popup read while a background refresh
	// writes. Under the default journal mode the writer would lock readers
	// out, and completion would stall exactly when the schema is being
	// loaded — the moment it is most needed.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening the catalog cache: %w", err)
	}

	c := &Cache{db: db}
	if err := c.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return c, nil
}

// Close releases the database handle.
func (c *Cache) Close() error { return c.db.Close() }

const schemaSQL = `
CREATE TABLE IF NOT EXISTS cached_schema (
	datasource TEXT NOT NULL,
	name       TEXT NOT NULL,
	position   INTEGER NOT NULL,
	PRIMARY KEY (datasource, name)
);

CREATE TABLE IF NOT EXISTS cached_table (
	datasource TEXT NOT NULL,
	schema     TEXT NOT NULL,
	name       TEXT NOT NULL,
	is_view    INTEGER NOT NULL DEFAULT 0,
	rows_est   INTEGER NOT NULL DEFAULT 0,
	position   INTEGER NOT NULL,
	PRIMARY KEY (datasource, schema, name)
);

CREATE TABLE IF NOT EXISTS cached_column (
	datasource  TEXT NOT NULL,
	schema      TEXT NOT NULL,
	tbl         TEXT NOT NULL,
	name        TEXT NOT NULL,
	type        TEXT NOT NULL DEFAULT '',
	nullable    INTEGER NOT NULL DEFAULT 0,
	primary_key INTEGER NOT NULL DEFAULT 0,
	comment     TEXT NOT NULL DEFAULT '',
	position    INTEGER NOT NULL,
	PRIMARY KEY (datasource, schema, tbl, name)
);

-- Completion filters by prefix within one table, so these are the shapes
-- the lookups actually take.
CREATE INDEX IF NOT EXISTS cached_table_lookup
	ON cached_table (datasource, schema, position);
CREATE INDEX IF NOT EXISTS cached_column_lookup
	ON cached_column (datasource, schema, tbl, position);
`

func (c *Cache) migrate() error {
	if _, err := c.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("preparing the catalog cache: %w", err)
	}
	return nil
}

// Save replaces everything stored for a datasource.
//
// Replacing rather than merging is deliberate: a table dropped on the server
// has to stop being offered, and a merge would keep suggesting it forever.
// The whole write is one transaction, so a failure part-way leaves the
// previous snapshot intact rather than a half-updated one.
func (c *Cache) Save(ctx context.Context, datasource string, snap Snapshot) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting the cache update: %w", err)
	}
	defer tx.Rollback()

	for _, table := range []string{"cached_schema", "cached_table", "cached_column"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE datasource = ?", datasource); err != nil {
			return fmt.Errorf("clearing %s: %w", table, err)
		}
	}

	if err := insertSchemas(ctx, tx, datasource, snap); err != nil {
		return err
	}
	if err := insertTables(ctx, tx, datasource, snap); err != nil {
		return err
	}
	if err := insertColumns(ctx, tx, datasource, snap); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing the cache update: %w", err)
	}
	return nil
}

func insertSchemas(ctx context.Context, tx *sql.Tx, datasource string, snap Snapshot) error {
	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO cached_schema (datasource, name, position) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, name := range snap.Schemas {
		if _, err := stmt.ExecContext(ctx, datasource, name, i); err != nil {
			return fmt.Errorf("caching schema %q: %w", name, err)
		}
	}
	return nil
}

func insertTables(ctx context.Context, tx *sql.Tx, datasource string, snap Snapshot) error {
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO cached_table (datasource, schema, name, is_view, rows_est, position)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for schema, tables := range snap.Tables {
		for i, t := range tables {
			if _, err := stmt.ExecContext(ctx, datasource, schema, t.Name, t.IsView, t.Rows, i); err != nil {
				return fmt.Errorf("caching table %s.%s: %w", schema, t.Name, err)
			}
		}
	}
	return nil
}

func insertColumns(ctx context.Context, tx *sql.Tx, datasource string, snap Snapshot) error {
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO cached_column
		   (datasource, schema, tbl, name, type, nullable, primary_key, comment, position)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for key, columns := range snap.Columns {
		schema, table, ok := splitKey(key)
		if !ok {
			return fmt.Errorf("malformed column key %q, want schema.table", key)
		}
		for i, col := range columns {
			if _, err := stmt.ExecContext(ctx, datasource, schema, table,
				col.Name, col.Type, col.Nullable, col.IsPrimaryKey, col.Comment, i); err != nil {
				return fmt.Errorf("caching column %s.%s: %w", key, col.Name, err)
			}
		}
	}
	return nil
}

// ColumnKey builds the map key Snapshot.Columns uses.
func ColumnKey(schema, table string) string { return schema + "." + table }

func splitKey(key string) (schema, table string, ok bool) {
	for i := 0; i < len(key); i++ {
		if key[i] == '.' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

// Schemas lists the cached schema names for a datasource.
func (c *Cache) Schemas(ctx context.Context, datasource string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx,
		"SELECT name FROM cached_schema WHERE datasource = ? ORDER BY position", datasource)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// Tables lists the cached tables of a schema.
func (c *Cache) Tables(ctx context.Context, datasource, schema string) ([]Table, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT name, is_view, rows_est FROM cached_table
		 WHERE datasource = ? AND schema = ? ORDER BY position`, datasource, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Name, &t.IsView, &t.Rows); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Columns lists the cached columns of a table, in declaration order.
func (c *Cache) Columns(ctx context.Context, datasource, schema, table string) ([]Column, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT name, type, nullable, primary_key, comment FROM cached_column
		 WHERE datasource = ? AND schema = ? AND tbl = ? ORDER BY position`,
		datasource, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.IsPrimaryKey, &c.Comment); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SearchTables finds tables whose name contains the given text, across every
// schema of a datasource. It backs the go-to-table jump.
func (c *Cache) SearchTables(ctx context.Context, datasource, needle string, limit int) ([]QualifiedTable, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := c.db.QueryContext(ctx,
		`SELECT schema, name, is_view FROM cached_table
		 WHERE datasource = ? AND name LIKE ? ESCAPE '\'
		 ORDER BY LENGTH(name), schema, name
		 LIMIT ?`,
		datasource, "%"+escapeLike(needle)+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QualifiedTable
	for rows.Next() {
		var q QualifiedTable
		if err := rows.Scan(&q.Schema, &q.Name, &q.IsView); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// QualifiedTable is a table together with the schema holding it.
type QualifiedTable struct {
	Schema string
	Name   string
	IsView bool
}

// escapeLike neutralises the wildcards in user-typed text, so searching for
// "user_id" does not match "userXid".
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
