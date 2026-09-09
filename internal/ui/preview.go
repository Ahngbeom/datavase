package ui

import (
	"fmt"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// previewLimit is how many rows a click on a table shows. A hundred fits a
// screen or two, which is what "let me see what is in here" asks for.
const previewLimit = 100

func previewSQL(schema, table string) string {
	return fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d",
		sqlparse.QuoteIdentifier(schema), sqlparse.QuoteIdentifier(table), previewLimit)
}

// previewTable runs the preview without touching the editor.
//
// The editor holds the user's own text, and a click that replaced it would
// lose work to a gesture nobody reads as destructive. The SQL that ran is
// shown in the status bar instead.
func (a *App) previewTable(schema, table string) {
	if a.running != nil {
		a.notice("a statement is already running; ^C cancels it")
		return
	}
	sql := previewSQL(schema, table)
	a.runStatement(sqlparse.Parse(sql))
	a.notice(sql)
}
