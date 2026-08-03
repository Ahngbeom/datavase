package ui

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/explain"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// planTimeout bounds an EXPLAIN. The server does not run the statement to
// produce one, so anything slower than this is the server in trouble rather
// than the query being expensive.
const planTimeout = 20 * time.Second

const planPlaceholder = "Put the cursor in a statement and ask for its plan."

// planPane draws a plan at whatever width it currently has.
//
// It holds the parsed tree rather than the rendered text and lays it out at
// draw, for the reason the top bar is read at draw time too: a plan rendered
// once is rendered for the pane's width at that moment, and the first thing
// anyone does with a plan that looks cramped is make the window bigger.
//
// It is also the only way the first render is right at all. A tab that has
// never been shown has no width to ask for, so rendering when the plan
// arrives folds a join into a column ten characters wide.
type planPane struct {
	*tview.TextView
	plan *explain.Node
	// width is what the tree was last laid out for, so an unchanged size
	// costs nothing.
	width int
	// text is the rendered tree, kept so copying does not have to read it
	// back off a widget it was escaped into.
	text string
}

func newPlanPane() *planPane {
	view := tview.NewTextView().SetDynamicColors(false)
	// Deliberately not wrapped by the widget: the tree is fitted before it
	// gets here, and a second wrap would break its indentation mid-line.
	view.SetWrap(false)
	view.SetText(planPlaceholder)

	return &planPane{TextView: view}
}

func (p *planPane) show(plan *explain.Node) {
	p.plan, p.width = plan, 0
	p.ScrollToBeginning()
}

func (p *planPane) Draw(screen tcell.Screen) {
	if _, _, width, _ := p.GetInnerRect(); p.plan != nil && width > 0 && width != p.width {
		p.width = width
		p.text = explain.Render(p.plan, width)
		p.SetText(result.EscapeTags(p.text))
	}
	p.TextView.Draw(screen)
}

func (a *App) buildPlanTab() tview.Primitive {
	a.planView = newPlanPane()
	return a.planView
}

// explainStatement asks the server how it would run the statement under the
// cursor.
//
// The buffer is not touched. Typing EXPLAIN in front of a statement and taking
// it out again is the thing this replaces, and it is exactly the edit that
// gets left behind and then run.
func (a *App) explainStatement() {
	text := a.editor.GetText()
	row, column, _, _ := a.editor.GetCursor()

	stmt, ok := sqlparse.StatementAt(text, offsetAt(text, row, column))
	if !ok {
		a.notice("no statement under the cursor")
		return
	}

	// EXPLAIN reports how a statement would run; it does not run it. That is
	// what makes this safe to fire at a production datasource with no
	// confirmation, and it is why ANALYZE — which does run the statement — is
	// not this key.
	a.notice("asking for the plan…")
	a.fetchPlan(stmt.SQL, a.currentSchema())
}

func (a *App) fetchPlan(statement, schema string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), planTimeout)
		defer cancel()

		raw, err := a.planJSON(ctx, statement, schema)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(fmt.Sprintf("explaining: %v", err))
				return
			}

			plan, err := explain.Parse([]byte(raw))
			if err != nil {
				a.notice(fmt.Sprintf("explaining: %v", err))
				return
			}

			// Shown before it is laid out: the pane has no width until it
			// is the visible tab, and it renders itself once it has one.
			a.resultTabs.show(tabPlan)
			a.planView.show(plan)
			a.notice(fmt.Sprintf("plan · %s copies it", a.copyKeyLabel()))
		})
	}()
}

// planJSON runs the EXPLAIN and returns what the server said.
//
// It goes over the control connection, like every other single statement this
// interface issues on its own behalf, so that asking for a plan never has to
// wait behind a result still streaming into the grid.
//
// The schema is set on that connection first. The control connection is
// dedicated and so keeps whatever schema it was opened with, which is the
// datasource's default — and an unqualified EXPLAIN resolved against the wrong
// schema answers about a different table with the same name. Nothing else on
// this connection is affected, because every catalog read names its schema in
// full.
func (a *App) planJSON(ctx context.Context, statement, schema string) (string, error) {
	var out string

	err := a.conn.WithControl(ctx, func(c *sql.Conn) error {
		if schema != "" {
			if _, err := c.ExecContext(ctx, "USE "+sqlparse.QuoteIdentifier(schema)); err != nil {
				return fmt.Errorf("switching to schema %q: %w", schema, err)
			}
		}

		row := c.QueryRowContext(ctx, "EXPLAIN FORMAT=JSON "+statement)
		return row.Scan(&out)
	})
	return out, err
}

// copyPlan puts the rendered tree on the clipboard.
func (a *App) copyPlan() bool {
	if a.planView == nil || a.planView.text == "" {
		return false
	}
	a.setClipboard(a.planView.text)
	a.notice("plan copied")
	return true
}
