package ui

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// filterTables narrows a schema's tables by a typed term.
//
// The filter runs in memory rather than through the cache's LIKE query: one
// schema's worth of names is small, and filtering locally keeps every
// keystroke instant regardless of how the cache is behaving.
func filterTables(tables []catalog.Table, term string) []catalog.Table {
	needle := strings.ToLower(strings.TrimSpace(term))

	out := make([]catalog.Table, 0, len(tables))
	for _, t := range tables {
		if needle == "" || strings.Contains(strings.ToLower(t.Name), needle) {
			out = append(out, t)
		}
	}

	// A name that starts with the term is far more likely the one being
	// reached for than one that merely contains it.
	sort.SliceStable(out, func(i, j int) bool {
		return prefixRank(out[i].Name, needle) < prefixRank(out[j].Name, needle)
	})
	return out
}

func prefixRank(name, needle string) int {
	if needle == "" || strings.HasPrefix(strings.ToLower(name), needle) {
		return 0
	}
	return 1
}

// tableDetail is the secondary line: how big the table is, or that it is a
// view.
func tableDetail(t catalog.Table) string {
	if t.IsView {
		return "view"
	}
	if t.Rows <= 0 {
		// InnoDB reports zero for tables it has not analysed, which is not
		// the same as an empty table — saying "0" would be a lie.
		return "table"
	}
	return approximateRows(t.Rows)
}

// approximateRows renders an estimate compactly. The number is the
// optimiser's guess, so extra digits would suggest a precision it lacks.
func approximateRows(n int64) string {
	switch {
	case n >= 1_000_000:
		return strconv.FormatFloat(float64(n)/1_000_000, 'f', 1, 64) + "M"
	case n >= 1_000:
		return strconv.FormatFloat(float64(n)/1_000, 'f', 1, 64) + "k"
	default:
		return strconv.FormatInt(n, 10)
	}
}

// buildTablesTab creates the pane listing the current schema's tables.
func (a *App) buildTablesTab() tview.Primitive {
	a.tableFilter = tview.NewInputField().SetLabel("filter: ")
	a.tableList = tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)

	a.tableFilter.SetChangedFunc(func(string) { a.renderTables() })

	a.tableFilter.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyDown, tcell.KeyEnter:
			if a.tableList.GetItemCount() > 0 {
				a.app.SetFocus(a.tableList)
			}
			return nil
		}
		return ev
	})

	a.tableList.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		ev = a.vimListKey(ev)
		if ev == nil {
			return nil
		}
		if ev.Key() == tcell.KeyUp && a.tableList.GetCurrentItem() == 0 {
			a.app.SetFocus(a.tableFilter)
			return nil
		}
		return ev
	})

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.tableFilter, 1, 0, true).
		AddItem(a.tableList, 0, 1, false)
	return layout
}

// renderTables refills the list from the cache.
//
// Reading from the cache rather than the server is what makes this instant;
// it is also why the list can be empty on a first run, which the pane says
// out loud rather than looking broken.
func (a *App) renderTables() {
	a.tableList.Clear()
	a.listedTables = nil

	schema := a.currentSchema()
	if schema == "" {
		a.tableList.AddItem("no schema selected", "", 0, nil)
		return
	}
	if a.cache == nil {
		a.tableList.AddItem("the schema cache is unavailable", "", 0, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()

	tables, err := a.cache.Tables(ctx, a.conn.DataSource().Name, schema)
	if err != nil {
		a.tableList.AddItem("reading the cache failed: "+err.Error(), "", 0, nil)
		return
	}
	if len(tables) == 0 {
		a.tableList.AddItem("still loading the schema…", "", 0, nil)
		return
	}

	matches := filterTables(tables, a.tableFilter.GetText())
	if len(matches) == 0 {
		a.tableList.AddItem("no matching tables", "", 0, nil)
		return
	}

	a.listedTables = matches
	for _, t := range matches {
		table := t
		a.tableList.AddItem(tableListLabel(table, a.tablesPaneWidth()), "", 0, func() {
			a.openTable(schema, table.Name)
		})
	}
}

// tableListLabel puts the name on the left and its size after it.
//
// The name is never abbreviated to make room for the size. A list of
// "custome…" tells the reader nothing, whereas a list of full names without
// row counts is still perfectly usable — so when space runs short the size is
// what goes.
func tableListLabel(t catalog.Table, width int) string {
	const gap = "  "

	name := result.EscapeTags(t.Name)
	detail := tableDetail(t)

	needed := utf8.RuneCountInString(t.Name) + utf8.RuneCountInString(gap) +
		utf8.RuneCountInString(detail)
	if width > 0 && needed > width {
		return name
	}
	return name + gap + detail
}

// tablesPaneWidth is the room the list has, used to right-align the detail.
func (a *App) tablesPaneWidth() int {
	if _, _, width, _ := a.tableList.GetInnerRect(); width > 0 {
		return width
	}
	return sidebarWidth - 4
}

// currentSchema is the schema the tables tab and unqualified queries use.
func (a *App) currentSchema() string {
	if a.selectedSchema != "" {
		return a.selectedSchema
	}
	return a.conn.DataSource().Database
}
