//go:build integration

package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

// addProdDataSource configures a second datasource against the same test
// server, differing only in name.
//
// The same server on purpose: what is being tested is that the interface moves
// to the datasource it was told to, and a second server would let a passing
// test mean nothing more than "it connected to something".
func (h *harness) addProdDataSource(t *testing.T) string {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ds.Name += "-prod"

	h.inspect(func(a *App) bool {
		a.cfg.DataSources = append(a.cfg.DataSources, *ds)
		a.connect = func(ctx context.Context, target *config.DataSource) (*session.Session, error) {
			conn, err := db.Open(ctx, target, password, "")
			if err != nil {
				return nil, err
			}
			return &session.Session{Conn: conn}, nil
		}
		return true
	})
	return ds.Name
}

// switchToProd drives the switch through the picker, which is the route a user
// takes to it.
func (h *harness) switchToProd(t *testing.T) string {
	t.Helper()

	name := h.addProdDataSource(t)

	h.do(keymap.ActionSwitchDataSource)
	h.waitFor("the datasource picker", func(a *App) bool {
		front, _ := a.pages.GetFrontPage()
		return front == pageDataSource
	})

	h.typeInto("-prod")
	h.press(tcell.KeyEnter)

	h.waitFor("the switch", func(a *App) bool { return a.conn.DataSource().Name == name })
	return name
}

// The whole point: choosing a datasource from the picker actually moves the
// session there, and leaves nothing of the old one's results behind.
func TestSwitchingDatasourceMovesTheSession(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("a row from the old datasource", func(a *App) bool { return a.buf.RowCount() > 0 })

	was := h.currentDataSource()
	name := h.switchToProd(t)

	if name == was {
		t.Fatalf("addProdDataSource gave back the same name as %q", was)
	}
	h.waitFor("the datasource to change", func(a *App) bool {
		return a.conn.DataSource().Name == name
	})
	h.waitFor("the old result to be gone", func(a *App) bool {
		return a.buf.RowCount() == 0
	})
}

// Connecting first and letting go second. A switch that closed what it had and
// then failed would leave an interface with no connection behind it, which
// nothing here recovers from without a restart.
func TestAFailedSwitchLeavesTheSessionWhereItWas(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	was := h.currentDataSource()

	h.inspect(func(a *App) bool {
		a.cfg.DataSources = append(a.cfg.DataSources, config.DataSource{
			Name: "unreachable", Env: config.EnvProd, Host: "127.0.0.1", Port: 1, User: "nobody",
		})
		a.connect = func(context.Context, *config.DataSource) (*session.Session, error) {
			return nil, errors.New("bastion refused the connection")
		}
		return true
	})

	h.inspect(func(a *App) bool {
		a.switchTo(&a.cfg.DataSources[1])
		return true
	})

	if !h.waitForScreen("bastion refused") {
		t.Fatalf("the failure was not reported; screen:\n%s", h.text())
	}
	if got := h.currentDataSource(); got != was {
		t.Errorf("the session moved to %q despite the failure, from %q", got, was)
	}

	// And the connection it kept still works.
	h.typeSQL("SELECT 1 AS still_here")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("still_here") {
		t.Errorf("the kept connection no longer runs anything; screen:\n%s", h.text())
	}
}

func (h *harness) currentDataSource() string {
	h.t.Helper()

	var name string
	h.inspect(func(a *App) bool {
		name = a.conn.DataSource().Name
		return true
	})
	return name
}

// Two datasources each holding an in-flight statement would mean two results
// and two cancellations to reason about. Refusing is the smaller thing to
// explain, and it has to actually refuse.
func TestSwitchingIsRefusedWhileAStatementRuns(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	name := h.addProdDataSource(t)

	h.typeSQL("SELECT SLEEP(5)")
	h.do(keymap.ActionRun)
	h.waitFor("the statement to be in flight", func(a *App) bool { return a.running != nil })

	h.inspect(func(a *App) bool {
		a.switchTo(&a.cfg.DataSources[1])
		return true
	})

	if !h.waitForScreen("still running") {
		t.Fatalf("switching mid-statement said nothing; screen:\n%s", h.text())
	}
	if got := h.currentDataSource(); got == name {
		t.Error("the session switched while a statement was running")
	}

	h.do(keymap.ActionCancel)
}
