package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// catalogTimeout bounds a metadata read. information_schema can be slow on a
// server with tens of thousands of tables, and a hung read must not leave
// the tree looking permanently empty.
const catalogTimeout = 20 * time.Second

// nodeRef is what a tree node stands for. Nodes carry their identity rather
// than deriving it from their position, so a node knows what to load when it
// is expanded regardless of how the tree was built.
type nodeRef struct {
	kind   nodeKind
	schema string
	table  string
	loaded bool
}

type nodeKind int

const (
	nodeSchema nodeKind = iota
	nodeTable
	nodeColumn
)

// loadSchemas fills the top level of the tree in the background, then starts
// the cache refresh that completion depends on.
func (a *App) loadSchemas() {
	ds := a.conn.DataSource()
	label := rootLabel(ds, sidebarWidth-4)

	root := a.tree.GetRoot()
	root.ClearChildren()
	root.SetText(label + "  (loading…)")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
		defer cancel()

		schemas, err := catalog.Schemas(ctx, a.conn)
		a.app.QueueUpdateDraw(func() {
			root.SetText(label)
			if err != nil {
				a.status.phase = phaseFailed
				a.status.err = fmt.Errorf("reading schemas: %w", err)
				return
			}
			for _, s := range schemas {
				root.AddChild(schemaNode(s, ds.Database))
			}
		})
	}()

	a.refreshCache()
}

// snapshotTimeout bounds the whole-schema read, which is far larger than a
// single catalog query.
const snapshotTimeout = 5 * time.Minute

// refreshCache reloads the completion cache in the background.
//
// Nothing waits on it: the previous snapshot keeps serving completion while
// this runs, which is the reason the cache is written in WAL mode. A failure
// is reported but is not fatal — stale completion beats none.
func (a *App) refreshCache() {
	if a.cache == nil {
		return
	}

	name := a.conn.DataSource().Name
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), snapshotTimeout)
		defer cancel()

		snap, err := catalog.FetchSnapshot(ctx, a.conn)
		if err == nil {
			err = a.cache.Save(ctx, name, snap)
		}

		if err == nil {
			// Success is silent. This runs unasked, and announcing it would
			// wipe out the feedback for whatever the user actually just did.
			// That the cache filled is evident from completion working.
			return
		}
		a.app.QueueUpdateDraw(func() {
			a.notice(fmt.Sprintf("schema cache not updated: %v", err))
		})
	}()
}

func schemaNode(name, current string) *tview.TreeNode {
	colour := tcell.ColorTeal
	if current != "" && strings.EqualFold(name, current) {
		// The schema an unqualified query will hit deserves to stand out.
		colour = tcell.ColorAqua
	}

	return tview.NewTreeNode(schemaLabel(name, current)).
		SetReference(&nodeRef{kind: nodeSchema, schema: name}).
		SetColor(colour).
		SetSelectable(true).
		SetExpanded(false)
}

// onTreeSelect expands a node, loading its children the first time.
//
// Loading lazily is what keeps a database with tens of thousands of tables
// usable: only the branch actually opened costs a metadata query.
func (a *App) onTreeSelect(node *tview.TreeNode) {
	ref, ok := node.GetReference().(*nodeRef)
	if !ok {
		// The root: just toggle.
		node.SetExpanded(!node.IsExpanded())
		return
	}

	switch ref.kind {
	case nodeSchema:
		// Choosing a schema here is also how the tables tab, completion and
		// the status bar learn which one to work against.
		a.setSchema(ref.schema)
		if !ref.loaded {
			ref.loaded = true
			a.loadTables(node, ref.schema)
		}
		node.SetExpanded(!node.IsExpanded())

	case nodeTable:
		if !ref.loaded {
			ref.loaded = true
			a.loadColumns(node, ref.schema, ref.table)
		}
		node.SetExpanded(!node.IsExpanded())

	case nodeColumn:
		// Leaf: insert its name at the cursor, which is the quickest way to
		// build a column list without typing.
		a.insertAtCursor(ref.table)
	}
}

func (a *App) loadTables(node *tview.TreeNode, schema string) {
	node.SetText(node.GetText() + "  (loading…)")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
		defer cancel()

		tables, err := catalog.Tables(ctx, a.conn, schema)
		a.app.QueueUpdateDraw(func() {
			node.SetText(schemaLabel(schema, a.conn.DataSource().Database))
			if err != nil {
				a.status.phase = phaseFailed
				a.status.err = fmt.Errorf("reading tables in %s: %w", schema, err)
				return
			}
			for _, t := range tables {
				node.AddChild(tableNode(schema, t))
			}
		})
	}()
}

func tableNode(schema string, t catalog.Table) *tview.TreeNode {
	label := result.EscapeTags(t.Name)
	colour := tcell.ColorWhite
	if t.IsView {
		label += "  view"
		colour = tcell.ColorViolet
	} else if t.Rows > 0 {
		label += fmt.Sprintf("  ~%d", t.Rows)
	}

	return tview.NewTreeNode(label).
		SetReference(&nodeRef{kind: nodeTable, schema: schema, table: t.Name}).
		SetColor(colour).
		SetSelectable(true).
		SetExpanded(false)
}

func (a *App) loadColumns(node *tview.TreeNode, schema, table string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
		defer cancel()

		columns, err := catalog.Columns(ctx, a.conn, schema, table)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.status.phase = phaseFailed
				a.status.err = fmt.Errorf("reading columns of %s.%s: %w", schema, table, err)
				return
			}
			for _, c := range columns {
				node.AddChild(columnNode(schema, c))
			}
		})
	}()
}

func columnNode(schema string, c catalog.Column) *tview.TreeNode {
	label := fmt.Sprintf("%s  %s", result.EscapeTags(c.Name), result.EscapeTags(c.Type))
	if c.IsPrimaryKey {
		label += "  PK"
	}
	if !c.Nullable {
		label += "  NOT NULL"
	}

	colour := colourDim
	if c.IsPrimaryKey {
		colour = colourNotice
	}

	// The reference carries the bare column name so selecting the node can
	// paste it into the editor.
	return tview.NewTreeNode(label).
		SetReference(&nodeRef{kind: nodeColumn, schema: schema, table: c.Name}).
		SetColor(colour).
		SetSelectable(true)
}

// insertAtCursor pastes text into the editor at the caret.
//
// Replace rather than SetText: rebuilding the whole buffer looks the same on
// screen but discards the undo history, so a pasted column name could not be
// taken back.
func (a *App) insertAtCursor(text string) {
	current := a.editor.GetText()
	row, column, _, _ := a.editor.GetCursor()
	at := offsetAt(current, row, column)

	a.editor.Replace(at, at, text)
	a.editor.Select(at+len(text), at+len(text))
	a.app.SetFocus(a.editor)
}
