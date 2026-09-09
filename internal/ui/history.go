package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/result"
)

// showHistory opens a searchable list of previously run statements.
func (a *App) showHistory() {
	if a.history == nil {
		a.notice("query history is unavailable")
		return
	}

	box := a.newSearchBox("search: ", " history ", pageHistory, func(term string) []searchItem {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()

		entries, err := a.history.Search(ctx, term, 100)
		if err != nil {
			return []searchItem{message("search failed", err.Error())}
		}
		if len(entries) == 0 {
			if term == "" {
				return []searchItem{nothingHere("nothing has been run yet",
					"statements are remembered once they finish")}
			}
			return []searchItem{noMatch("statement", term)}
		}

		items := make([]searchItem, len(entries))
		for i, e := range entries {
			entry := e
			items[i] = searchItem{
				primary: oneLineSQL(entry.SQL),
				secondary: fmt.Sprintf("%s · %d rows · %s",
					entry.At.Local().Format("2006-01-02 15:04"), entry.Rows, entry.DataSource),
				accept: func() {
					a.closeSearchBox(pageHistory)
					a.editor.SetText(entry.SQL, true)
				},
			}
		}
		return items
	})

	a.pages.AddPage(pageHistory, centred(box, 90, 24), true, true)
}

// oneLineSQL flattens a statement so each history entry occupies one row.
func oneLineSQL(sql string) string {
	return result.Truncate(strings.Join(strings.Fields(sql), " "), 110)
}
