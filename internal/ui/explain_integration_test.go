//go:build integration

package ui

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// planLines reads the rendered plan out of the pane.
func (h *harness) planLines() []string {
	h.t.Helper()

	var text string
	h.inspect(func(a *App) bool {
		text = a.planView.text
		return true
	})
	return strings.Split(text, "\n")
}

// The acceptance for #14, against a real server: a join is readable as a tree.
func TestExplainingAJoinShowsItAsATree(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	createPlanFixtures(t)

	h.typeSQL("SELECT dv_p_authors.name, dv_p_books.title FROM dv_p_books JOIN dv_p_authors ON dv_p_authors.id = dv_p_books.author_id WHERE dv_p_books.year > 2001 ORDER BY dv_p_books.title")
	h.do(keymap.ActionExplain)

	h.waitFor("the plan", func(a *App) bool { return a.planView.text != "" })

	lines := h.planLines()
	joined := strings.Join(lines, "\n")

	// Both tables appear, each on its own step, with the tree drawn between
	// them — which is the whole difference from the grid.
	for _, want := range []string{"dv_p_books", "dv_p_authors", "└─"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the plan does not show %q:\n%s", want, joined)
		}
	}

	// And it goes to its own tab rather than replacing the results.
	h.waitFor("the plan tab", func(a *App) bool { return a.resultTabs.current() == tabPlan })
}

// The reason this is not the ordinary grid. A plan that reaches past the pane
// has reinvented the horizontal scrolling it exists to avoid.
func TestThePlanFitsThePaneItIsDrawnIn(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	createPlanFixtures(t)

	h.typeSQL("SELECT dv_p_authors.name, dv_p_books.title FROM dv_p_books JOIN dv_p_authors ON dv_p_authors.id = dv_p_books.author_id WHERE dv_p_books.year > 2001 AND dv_p_authors.country = 'uk' ORDER BY dv_p_books.title")
	h.do(keymap.ActionExplain)
	h.waitFor("the plan", func(a *App) bool { return a.planView.text != "" })

	// The pane's own rect, not the width the tree happens to have been laid
	// out for — comparing the render against itself would have passed while
	// the first version folded a join into ten columns.
	var pane, laidOutFor int
	h.inspect(func(a *App) bool {
		_, _, pane, _ = a.planView.GetInnerRect()
		laidOutFor = a.planView.width
		return true
	})

	if pane <= 20 {
		t.Fatalf("the plan pane is %d columns; the test cannot tell fitting from folding", pane)
	}
	if laidOutFor != pane {
		t.Errorf("the plan was laid out for %d columns in a pane of %d", laidOutFor, pane)
	}
	for _, line := range h.planLines() {
		if n := len([]rune(line)); n > pane {
			t.Errorf("a plan line is %d wide in a pane of %d:\n%s", n, pane, line)
		}
	}
}

// The whole point of asking is to find the expensive step, and a plan that
// leaves it to be spotted among thirty fields is the grid again.
func TestThePlanCallsOutAFullScan(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	createPlanFixtures(t)

	h.typeSQL("SELECT * FROM dv_p_books WHERE title = 'a'")
	h.do(keymap.ActionExplain)
	h.waitFor("the plan", func(a *App) bool { return a.planView.text != "" })

	if joined := strings.Join(h.planLines(), "\n"); !strings.Contains(joined, "full scan") {
		t.Errorf("a scan of an unindexed column went unflagged:\n%s", joined)
	}
}

// Typing EXPLAIN in front of a statement and taking it out again is what this
// replaces, and it is exactly the edit that gets left behind and then run.
func TestExplainingLeavesTheBufferAlone(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	createPlanFixtures(t)

	const sql = "SELECT * FROM dv_p_books"
	h.typeSQL(sql)
	h.do(keymap.ActionExplain)
	h.waitFor("the plan", func(a *App) bool { return a.planView.text != "" })

	if got := h.editorText(); got != sql {
		t.Errorf("the buffer reads %q, want %q", got, sql)
	}
}

// A statement the server refuses to explain has to say so. Rendering an empty
// tree would read as a query with no steps.
func TestAnUnexplainableStatementSaysWhy(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT * FROM dv_no_such_table_anywhere")
	h.do(keymap.ActionExplain)

	if !h.waitForScreen("explaining:") {
		t.Fatalf("the failure was not reported; screen:\n%s", h.text())
	}
	h.inspect(func(a *App) bool {
		if a.planView.text != "" {
			t.Errorf("a plan was shown for a statement that cannot run: %q", a.planView.text)
		}
		return true
	})
}

// createPlanFixtures makes two related tables, so a join has something to join
// and the optimiser something to choose between.
//
// Built on a connection of its own rather than through the interface. The
// guard stops to confirm every CREATE, which is what it is for — and answering
// those dialogs would make each of these tests mostly about the guard.
func createPlanFixtures(t *testing.T) {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	defer conn.Close()

	err = conn.WithControl(ctx, func(c *sql.Conn) error {
		for _, stmt := range []string{
			"CREATE TABLE IF NOT EXISTS dv_p_authors (id INT PRIMARY KEY, name VARCHAR(64), country VARCHAR(32))",
			"CREATE TABLE IF NOT EXISTS dv_p_books (id INT PRIMARY KEY, author_id INT, title VARCHAR(128), year INT, KEY(author_id))",
			"INSERT IGNORE INTO dv_p_authors VALUES (1,'ada','uk'),(2,'bob','us')",
			"INSERT IGNORE INTO dv_p_books VALUES (1,1,'a',2001),(2,1,'b',2002),(3,2,'c',2003)",
		} {
			if _, err := c.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("%s: %w", stmt, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("creating the plan fixtures: %v", err)
	}
}

// The control connection keeps whatever schema it was opened with, which is the
// datasource's default. An unqualified EXPLAIN sent down it after the user has
// chosen another schema would answer about a different table with the same
// name — or, as here, fail to find one that is plainly there.
func TestExplainingFollowsTheChosenSchema(t *testing.T) {
	const other = "datavase_test_plan"
	createSchemaFixture(t, other)

	h := newHarness(t, config.EnvDev)
	h.inspect(func(a *App) bool { a.selectedSchema = other; return true })

	// dv_p_only_here exists in that schema and nowhere else, so the plan can
	// only be produced if the EXPLAIN went to it.
	h.typeSQL("SELECT * FROM dv_p_only_here")
	h.do(keymap.ActionExplain)

	h.waitFor("the plan", func(a *App) bool { return a.planView.text != "" })

	if joined := strings.Join(h.planLines(), "\n"); !strings.Contains(joined, "dv_p_only_here") {
		t.Errorf("the plan does not name the table from the chosen schema:\n%s", joined)
	}
}

// createSchemaFixture makes a second schema holding a table that exists in no
// other, on a connection of its own.
func createSchemaFixture(t *testing.T, schema string) {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	defer conn.Close()

	err = conn.WithControl(ctx, func(c *sql.Conn) error {
		for _, stmt := range []string{
			"CREATE DATABASE IF NOT EXISTS " + schema,
			"CREATE TABLE IF NOT EXISTS " + schema + ".dv_p_only_here (id INT PRIMARY KEY, note VARCHAR(32))",
		} {
			if _, err := c.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("%s: %w", stmt, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("creating the second schema: %v", err)
	}
}
