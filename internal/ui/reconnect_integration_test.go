//go:build integration

package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// canReconnect gives the harness a way back to the same server. The default
// one refuses, so that a test which never meant to connect cannot quietly do
// it.
func (h *harness) canReconnect() {
	h.t.Helper()
	_, password := testmysql.DataSource(h.t)
	h.inspect(func(a *App) bool {
		a.connect = func(ctx context.Context, ds *config.DataSource) (*session.Session, error) {
			return session.Open(ctx, ds, password)
		}
		return true
	})
}

// loseTheSession takes the connection away while the server stays where it
// was, which is what an idle timeout, a dropped network or a slept laptop
// leave behind: a session that is gone and a database that is fine.
func (h *harness) loseTheSession() {
	h.t.Helper()
	h.inspect(func(a *App) bool {
		a.conn.Close()
		return true
	})
}

// The statement that discovers a dead session must say the session is dead.
// The driver's own words for it are "invalid connection", which reads as the
// database being in trouble and says nothing about what to do next.
func TestAStatementOnALostSessionSaysTheConnectionIsGoneAndNamesTheWayBack(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 1 AS before_it_went")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.status.phase == phaseDone })

	h.loseTheSession()

	h.typeSQL("SELECT 1 AS after_it_went")
	h.do(keymap.ActionRun)
	h.waitFor("the failure", func(a *App) bool { return a.status.phase == phaseFailed })

	said := h.inspect(func(a *App) bool {
		return a.status.err != nil && strings.Contains(a.status.err.Error(), "connection is gone")
	})
	if !said {
		t.Errorf("nothing said the connection is gone:\n%s", h.text())
	}
	if !h.inspect(func(a *App) bool { return a.sessionLost }) {
		t.Error("the interface does not know the session is lost")
	}

	// The key is named where the failure is, because a key nobody is told
	// about is a session that ends here.
	key := h.inspect(func(a *App) bool {
		return strings.Contains(a.status.err.Error(), a.keyLabel(keymap.ActionRefreshSchema))
	})
	if !key {
		t.Errorf("the way back is not named:\n%s", h.text())
	}
}

// Reconnecting is one key, and what comes back is a different session: it has
// to say which schema an unqualified statement now reaches, because landing
// on the datasource's default without a word is how the right SQL runs
// against the wrong schema.
func TestReconnectingOpensANewSessionAndSaysWhichSchemaItIs(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.canReconnect()

	h.typeSQL("SELECT 1 AS a UNION ALL SELECT 2")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.buf.RowCount() == 2 })

	// A schema that is not the datasource's default, so that losing the
	// choice and landing back on the default is a visible difference rather
	// than the same word twice.
	h.inspect(func(a *App) bool {
		a.selectedSchema = "mysql"
		return true
	})
	h.loseTheSession()

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("the failure", func(a *App) bool { return a.sessionLost })

	h.do(keymap.ActionRefreshSchema)
	h.waitFor("the new session", func(a *App) bool { return !a.sessionLost })

	if !h.inspect(func(a *App) bool { return strings.Contains(a.status.message, "reconnected") }) {
		t.Errorf("the reconnection was silent:\n%s", h.text())
	}
	if !h.inspect(func(a *App) bool { return a.selectedSchema == "mysql" }) {
		t.Error("reconnecting dropped the chosen schema back to the datasource default")
	}
	if !h.inspect(func(a *App) bool { return strings.Contains(a.status.message, "mysql") }) {
		t.Errorf("the new session did not say which schema it reaches:\n%s", h.text())
	}

	// The session really is new, not the dead one with a cleared flag.
	h.typeSQL("SELECT 3 AS after_reconnect")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("after_reconnect") {
		t.Errorf("the reconnected session cannot run a statement:\n%s", h.text())
	}
}

// A reconnection that re-ran what was on the editor would be a statement
// nobody asked for, against a session that had just changed underneath them.
func TestReconnectingDoesNotRunTheStatementAgain(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.canReconnect()
	h.loseTheSession()

	h.typeSQL("SELECT 1 AS ran_before_the_drop")
	h.do(keymap.ActionRun)
	h.waitFor("the failure", func(a *App) bool { return a.sessionLost })

	h.do(keymap.ActionRefreshSchema)
	h.waitFor("the new session", func(a *App) bool { return !a.sessionLost })

	if h.inspect(func(a *App) bool { return a.buf.RowCount() > 0 }) {
		t.Error("reconnecting ran the statement that was in the editor")
	}
}

// The guard has to be re-established on the session that replaces the dead
// one, and said again — a read-only datasource whose new session nobody
// confirmed is the state this marker exists to make impossible.
func TestReconnectingConfirmsReadOnlyAgain(t *testing.T) {
	h := newReadOnlyHarness(t)
	h.canReconnect()
	h.loseTheSession()

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("the failure", func(a *App) bool { return a.sessionLost })

	h.do(keymap.ActionRefreshSchema)
	h.waitFor("the new session", func(a *App) bool { return !a.sessionLost })

	if !h.inspect(func(a *App) bool { return a.conn.ReadOnlyConfirmed() }) {
		t.Error("the replacement session was not confirmed read-only")
	}
	if !h.inspect(func(a *App) bool { return strings.Contains(a.status.message, "read-only") }) {
		t.Errorf("the new session did not say it is still read-only:\n%s", h.text())
	}
}
