package catalog

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Ahngbeom/datavase/internal/db"
)

// FetchSnapshot reads a datasource's whole schema from the server.
//
// Columns are fetched one query per schema rather than one per table. A
// server with a few thousand tables would otherwise need a few thousand round
// trips — minutes through a bastion — and the point of the cache is that
// completion is ready shortly after connecting, not eventually.
func FetchSnapshot(ctx context.Context, conn *db.Conn) (Snapshot, error) {
	snap := Snapshot{
		Tables:  make(map[string][]Table),
		Columns: make(map[string][]Column),
	}

	schemas, err := Schemas(ctx, conn)
	if err != nil {
		return snap, fmt.Errorf("reading schemas: %w", err)
	}
	snap.Schemas = schemas

	for _, schema := range schemas {
		tables, err := Tables(ctx, conn, schema)
		if err != nil {
			return snap, fmt.Errorf("reading tables in %s: %w", schema, err)
		}
		snap.Tables[schema] = tables

		columns, err := columnsBySchema(ctx, conn, schema)
		if err != nil {
			return snap, fmt.Errorf("reading columns in %s: %w", schema, err)
		}
		for table, cols := range columns {
			snap.Columns[ColumnKey(schema, table)] = cols
		}
	}
	return snap, nil
}

// columnsBySchema reads every column of a schema in one query, grouped by
// table.
func columnsBySchema(ctx context.Context, conn *db.Conn, schema string) (map[string][]Column, error) {
	const q = `SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY,
	                  COALESCE(COLUMN_DEFAULT, ''), COLUMN_COMMENT
	           FROM information_schema.COLUMNS
	           WHERE TABLE_SCHEMA = ?
	           ORDER BY TABLE_NAME, ORDINAL_POSITION`

	out := make(map[string][]Column)
	err := query(ctx, conn, q, []any{schema}, func(rows *sql.Rows) error {
		var (
			table    string
			c        Column
			nullable string
			key      string
		)
		if err := rows.Scan(&table, &c.Name, &c.Type, &nullable, &key,
			&c.Default, &c.Comment); err != nil {
			return err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimaryKey = key == "PRI"
		out[table] = append(out[table], c)
		return nil
	})
	return out, err
}
