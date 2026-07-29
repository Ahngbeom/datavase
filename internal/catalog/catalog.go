// Package catalog reads schema metadata from the server.
//
// Every query here runs on the connection's control connection, so browsing
// the schema stays responsive while a large result is still streaming.
package catalog

import (
	"context"
	"database/sql"

	"github.com/Ahngbeom/datavase/internal/db"
)

// Table is one table or view in a schema.
type Table struct {
	Name   string
	IsView bool
	// Rows is the optimiser's estimate, not an exact count. Counting for
	// real would mean a full scan per table on every expand.
	Rows int64
}

// Column is one column of a table.
type Column struct {
	Name         string
	Type         string
	Nullable     bool
	IsPrimaryKey bool
	Default      string
	Comment      string
}

// internalSchemas are the server's own; they are noise in a tree meant for
// application data, and both DataGrip and DBeaver hide them by default.
var internalSchemas = []any{
	"information_schema",
	"performance_schema",
	"mysql",
	"sys",
}

// query runs a read on the control connection and scans every row.
//
// Going through WithControl is what serialises these against each other and
// against KILL QUERY; two of them at once would desynchronise the connection
// and the driver would then discard it.
func query(ctx context.Context, conn *db.Conn, sqlText string, args []any, scan func(*sql.Rows) error) error {
	return conn.WithControl(ctx, func(c *sql.Conn) error {
		rows, err := c.QueryContext(ctx, sqlText, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			if err := scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	})
}

// Schemas lists the user-visible databases on the server.
func Schemas(ctx context.Context, conn *db.Conn) ([]string, error) {
	const q = `SELECT SCHEMA_NAME
	           FROM information_schema.SCHEMATA
	           WHERE SCHEMA_NAME NOT IN (?, ?, ?, ?)
	           ORDER BY SCHEMA_NAME`

	var out []string
	err := query(ctx, conn, q, internalSchemas, func(rows *sql.Rows) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		out = append(out, name)
		return nil
	})
	return out, err
}

// Tables lists the tables and views in a schema.
func Tables(ctx context.Context, conn *db.Conn, schema string) ([]Table, error) {
	// TABLE_ROWS is NULL for views and an estimate for InnoDB tables.
	const q = `SELECT TABLE_NAME, TABLE_TYPE, COALESCE(TABLE_ROWS, 0)
	           FROM information_schema.TABLES
	           WHERE TABLE_SCHEMA = ?
	           ORDER BY TABLE_NAME`

	var out []Table
	err := query(ctx, conn, q, []any{schema}, func(rows *sql.Rows) error {
		var (
			t         Table
			tableType string
		)
		if err := rows.Scan(&t.Name, &tableType, &t.Rows); err != nil {
			return err
		}
		t.IsView = tableType == "VIEW"
		out = append(out, t)
		return nil
	})
	return out, err
}

// Columns lists a table's columns in declaration order, which is also the
// order "SELECT *" returns them in.
func Columns(ctx context.Context, conn *db.Conn, schema, table string) ([]Column, error) {
	const q = `SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY,
	                  COALESCE(COLUMN_DEFAULT, ''), COLUMN_COMMENT
	           FROM information_schema.COLUMNS
	           WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	           ORDER BY ORDINAL_POSITION`

	var out []Column
	err := query(ctx, conn, q, []any{schema, table}, func(rows *sql.Rows) error {
		var (
			c        Column
			nullable string
			key      string
		)
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &key, &c.Default, &c.Comment); err != nil {
			return err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimaryKey = key == "PRI"
		out = append(out, c)
		return nil
	})
	return out, err
}
