package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
)

// showGoToTable offers a search across every table of the datasource.
//
// The schema tree answers "what is in this database"; this answers "where is
// the table whose name I already know", which on a server with thousands of
// tables is a different question and a much more common one.
func (a *App) showGoToTable() {
	if a.cache == nil {
		a.notice("the schema cache is unavailable")
		return
	}

	box := a.newSearchBox("table: ", " go to table ", pageGoTo, func(term string) []searchItem {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()

		tables, err := a.cache.SearchTables(ctx, a.conn.DataSource().Name, term, 100)
		if err != nil {
			return []searchItem{message("search failed", err.Error())}
		}
		if len(tables) == 0 {
			return []searchItem{message("no matching tables", "type part of a name")}
		}

		items := make([]searchItem, len(tables))
		for i, t := range tables {
			table := t
			kind := "table"
			if table.IsView {
				kind = "view"
			}
			items[i] = searchItem{
				primary:   table.Name,
				secondary: fmt.Sprintf("%s · %s", table.Schema, kind),
				accept: func() {
					a.closeSearchBox(pageGoTo)
					a.openTable(table.Schema, table.Name)
				},
			}
		}
		return items
	})

	a.pages.AddPage(pageGoTo, centred(box, 70, 22), true, true)
}

// openTable brings a chosen table into the editor.
//
// An empty editor gets a starter query; an editor with work in it gets only
// the qualified name, inserted at the caret. Replacing the buffer outright —
// which this used to do — silently destroyed whatever the user was writing,
// with no way back.
//
// Either way the statement is placed rather than run, so the guard and the
// user still decide when anything executes.
func (a *App) openTable(schema, table string) {
	qualified := fmt.Sprintf("%s.%s", schema, table)

	if strings.TrimSpace(a.editor.GetText()) == "" {
		a.editor.SetText(fmt.Sprintf("SELECT *\nFROM %s", qualified), true)
		a.app.SetFocus(a.editor)
		a.notice(fmt.Sprintf("%s — %s to run", qualified, a.runKeyLabel()))
		return
	}

	a.insertAtCursor(qualified)
	a.notice(fmt.Sprintf("inserted %s", qualified))
}

// runKeyLabel names the run key as this terminal can deliver it.
func (a *App) runKeyLabel() string { return a.keyLabel(keymap.ActionRun) }
