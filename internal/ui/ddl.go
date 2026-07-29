package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/rivo/tview"
)

// ddlTimeout bounds a SHOW CREATE. It is a single statement on the control
// connection, so anything slower than this means the server is in trouble.
const ddlTimeout = 20 * time.Second

// buildDDLTab creates the pane that shows a table definition.
func (a *App) buildDDLTab() tview.Primitive {
	a.ddlView = tview.NewTextView().SetDynamicColors(false)
	a.ddlView.SetText(ddlPlaceholder)
	return a.ddlView
}

const ddlPlaceholder = "Select a table in the schema pane, then press the inspect key."

// inspectTable fetches the definition of whichever table is selected.
//
// The lookup is explicit rather than automatic: SHOW CREATE is a server round
// trip, and firing one every time the tree selection moves would make simply
// browsing expensive.
func (a *App) inspectTable() {
	schema, table, ok := a.selectedTable()
	if !ok {
		a.notice("select a table in the schema pane first")
		return
	}

	isView := a.tableIsView(schema, table)
	a.notice(fmt.Sprintf("reading %s.%s…", schema, table))

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), ddlTimeout)
		defer cancel()

		ddl, err := catalog.TableDDL(ctx, a.conn, schema, table, isView)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(fmt.Sprintf("reading the definition of %s.%s: %v", schema, table, err))
				return
			}

			a.ddlText = ddl
			a.ddlView.SetText(ddl).ScrollToBeginning()
			a.resultTabs.show(tabDDL)
			a.notice(fmt.Sprintf("%s.%s — %s copies it", schema, table, a.copyKeyLabel()))
		})
	}()
}

// selectedTable finds the table the user is pointing at.
//
// Which pane has focus decides, because both the tree and the tables list can
// hold a selection at once and guessing between them would sometimes inspect
// the wrong table.
func (a *App) selectedTable() (schema, table string, ok bool) {
	if a.app.GetFocus() == a.tableList {
		if name, found := a.selectedListTable(); found {
			return a.currentSchema(), name, true
		}
		return "", "", false
	}

	node := a.tree.GetCurrentNode()
	if node == nil {
		return "", "", false
	}
	ref, isRef := node.GetReference().(*nodeRef)
	if !isRef || ref.kind != nodeTable {
		return "", "", false
	}
	return ref.schema, ref.table, true
}

// selectedListTable reads the highlighted entry of the tables tab.
func (a *App) selectedListTable() (string, bool) {
	if a.tableList == nil || a.tableList.GetItemCount() == 0 {
		return "", false
	}

	index := a.tableList.GetCurrentItem()
	if index < 0 || index >= len(a.listedTables) {
		return "", false
	}
	return a.listedTables[index].Name, true
}

// tableIsView reports whether a name refers to a view, so the right SHOW
// CREATE is issued.
func (a *App) tableIsView(schema, table string) bool {
	if a.cache == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()

	tables, err := a.cache.Tables(ctx, a.conn.DataSource().Name, schema)
	if err != nil {
		return false
	}
	for _, t := range tables {
		if t.Name == table {
			return t.IsView
		}
	}
	return false
}

// copyDDL puts the whole definition on the clipboard.
func (a *App) copyDDL() bool {
	if a.ddlText == "" {
		return false
	}
	a.setClipboard(a.ddlText)
	a.notice("definition copied")
	return true
}

// copyKeyLabel names the copy key as this terminal can deliver it.
func (a *App) copyKeyLabel() string {
	bindings := a.keys.DisplayBindings(keymap.ActionCopyOrCancel)
	if len(bindings) == 0 {
		return "the copy key"
	}
	return bindings[0].Label(onMac)
}
