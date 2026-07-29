package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"strings"

	"github.com/Ahngbeom/datavase/internal/db"
)

// TableDDL returns the CREATE statement for a table or view.
//
// It runs on the control connection, alongside the other catalog reads, and
// deliberately bypasses guard: the statement is read-only and its text is
// built here rather than typed by the user.
func TableDDL(ctx context.Context, conn *db.Conn, schema, table string, isView bool) (string, error) {
	object := "TABLE"
	if isView {
		object = "VIEW"
	}

	// Identifiers cannot be bound as parameters — they are syntax, not
	// values — so the name is quoted and escaped by hand. The names come
	// from our own cache, but that cache holds whatever the server reported,
	// and a backtick in a name is legal in MySQL.
	statement := fmt.Sprintf("SHOW CREATE %s %s.%s",
		object, QuoteIdentifier(schema), QuoteIdentifier(table))

	var ddl string
	err := conn.WithControl(ctx, func(c *sql.Conn) error {
		rows, err := c.QueryContext(ctx, statement)
		if err != nil {
			return err
		}
		defer rows.Close()

		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return fmt.Errorf("%s.%s does not exist", schema, table)
		}

		// SHOW CREATE returns a different column layout for tables and
		// views, and the trailing columns differ again by server version.
		// Scanning by position into a slice sized to what arrived is the
		// only version-proof way to reach the definition.
		columns, err := rows.Columns()
		if err != nil {
			return err
		}

		cells := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range cells {
			targets[i] = &cells[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return err
		}

		ddl = definitionColumn(columns, cells)
		if ddl == "" {
			return fmt.Errorf("the server returned no definition for %s.%s", schema, table)
		}
		return rows.Err()
	})

	return ddl, err
}

// definitionColumn picks the CREATE statement out of a SHOW CREATE row.
//
// The column is named "Create Table" for tables and "Create View" for views,
// so the prefix is matched rather than the exact name.
func definitionColumn(columns []string, cells []any) string {
	for i, name := range columns {
		if !strings.HasPrefix(name, "Create ") {
			continue
		}
		switch v := cells[i].(type) {
		case []byte:
			return string(v)
		case string:
			return v
		}
	}
	return ""
}

// QuoteIdentifier renders a name as a backtick-quoted MySQL identifier.
func QuoteIdentifier(name string) string {
	return sqlparse.QuoteIdentifier(name)
}
