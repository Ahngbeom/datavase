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
// server, differing only in the thing the guard reads.
//
// The same server on purpose: what is being tested is that the interface moves
// to the datasource it was told to, and a second server would let a passing
// test mean nothing more than "it connected to something".
func (h *harness) addProdDataSource(t *testing.T) string {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ds.Name += "-prod"
	ds.Env = config.EnvProd

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

// The acceptance for #13. A guard reading the environment of the datasource
// you left is the failure this whole feature has to not have.
func TestSwitchingDataSourceMovesTheGuardPolicy(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.switchToProd(t)

	h.waitFor("the policy to name production", func(a *App) bool {
		return a.policy().Env == config.EnvProd
	})

	// And the policy is not a field anyone reads for its own sake: an
	// unbounded DELETE is confirmable on dev and refused outright on prod.
	h.typeSQL("DELETE FROM dv_switch_probe")
	h.do(keymap.ActionRun)

	if !h.waitForScreen("Refused") {
		t.Fatalf("an unbounded DELETE was not refused against production; screen:\n%s", h.text())
	}
}

// The spine is the one thing here meant to be believed without reading, so a
// stale one is worse than none at all. It used to be painted once, on the
// stated grounds that the environment could not change mid-session.
func TestSwitchingDataSourceRepaintsTheEnvironmentSpine(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if got := h.spineColour(); got != spineDev {
		t.Fatalf("the spine starts %v, want the dev colour", got)
	}

	h.switchToProd(t)

	h.waitFor("the spine to turn", func(a *App) bool {
		return a.spine.GetBackgroundColor() == spineProd
	})
}

func (h *harness) spineColour() tcell.Color {
	h.t.Helper()

	var got tcell.Color
	h.inspect(func(a *App) bool {
		got = a.spine.GetBackgroundColor()
		return true
	})
	return got
}

// The unlock is granted for a session and a datasource both. Carrying one
// granted on dev over to production is the single way this feature could undo
// the guard.
func TestSwitchingDataSourceLocksWritesAgain(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.inspect(func(a *App) bool { a.enableWrites(); return true })
	h.waitFor("writes to be unlocked", func(a *App) bool { return a.status.writesEnabled })

	h.switchToProd(t)

	h.waitFor("writes to be locked again", func(a *App) bool { return !a.status.writesEnabled })
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
