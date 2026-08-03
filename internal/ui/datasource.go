package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/complete"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/match"
	"github.com/Ahngbeom/datavase/internal/session"
)

// connectTimeout bounds a switch. The interface is unresponsive while it
// waits, and a bastion that is not answering should give the session back
// rather than hold it.
const connectTimeout = 15 * time.Second

// showDataSources offers the configured datasources.
func (a *App) showDataSources() {
	if a.connect == nil {
		a.notice("this session cannot switch datasource")
		return
	}
	if len(a.cfg.DataSources) < 2 {
		a.notice("only one datasource is configured")
		return
	}

	box := a.newSearchBox("datasource: ", " datasources ", pageDataSource, a.dataSourceChoices)
	a.pages.AddPage(pageDataSource, centred(box, 72, 20), true, true)
}

// dataSourceChoices filters the configured datasources.
//
// The environment is the secondary line rather than a decoration: choosing
// between two datasources is most often choosing between two environments,
// and the name alone — "orders", "orders-2" — does not say which is which.
func (a *App) dataSourceChoices(term string) []searchItem {
	current := a.conn.DataSource().Name

	rows := make([]ranked, 0, len(a.cfg.DataSources))
	for i := range a.cfg.DataSources {
		ds := &a.cfg.DataSources[i]

		score, ok := match.Fuzzy(term, ds.Name)
		if !ok {
			continue
		}

		detail := fmt.Sprintf("%s · %s@%s:%d", ds.Env, ds.User, ds.Host, ds.Port)
		if ds.Name == current {
			detail += " · current"
		}

		rows = append(rows, ranked{
			item: searchItem{
				primary:   ds.Name,
				secondary: detail,
				accept: func() {
					a.closeSearchBox(pageDataSource)
					a.switchTo(ds)
				},
			},
			score: score,
		})
	}

	items := sortRanked(rows)
	if len(items) == 0 {
		return []searchItem{message("no matching datasource", "type part of a name")}
	}
	return items
}

// switchTo moves the session to another datasource, asking about anything it
// would throw away first.
func (a *App) switchTo(ds *config.DataSource) {
	if ds.Name == a.conn.DataSource().Name {
		return
	}
	if a.running != nil {
		// Each datasource keeping its own in-flight statement would mean two
		// results and two cancellations to reason about. Refusing is the
		// smaller thing to explain, and the statement is one keystroke from
		// being stopped.
		a.notice(fmt.Sprintf("a statement is still running — %s cancels it",
			a.keyLabel(keymap.ActionCancel)))
		return
	}

	// An open transaction is unsaved work, and switching rolls it back by
	// closing the connection under it. The same question quitting asks.
	if a.conn.InTransaction() {
		a.confirmDiscard(
			fmt.Sprintf("A transaction is open on %s.\n\nSwitch to %s and roll it back?",
				a.conn.DataSource().Name, ds.Name),
			"Switch",
			func() { a.openDataSource(ds) })
		return
	}
	a.openDataSource(ds)
}

// openDataSource connects, and only then lets go of what it had.
//
// The order is the point: a failed switch leaves the session exactly where it
// was. Closing first and connecting second would turn an unreachable bastion
// into an interface with no connection behind it, which nothing here can
// recover from without a restart.
func (a *App) openDataSource(ds *config.DataSource) {
	a.notice(fmt.Sprintf("connecting to %s…", ds.Name))

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()

		sess, err := a.connect(ctx, ds)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.status.phase = phaseFailed
				a.status.err = fmt.Errorf("connecting to %s: %w", ds.Name, err)
				return
			}
			a.adopt(sess)
		})
	}()
}

// adopt makes a newly opened session the one the interface is looking at.
//
// Everything that describes where you are moves together, in one step. A
// half-switched interface — the new connection behind the old environment's
// colour, or the old datasource's tables in the tree — is worse than either
// state on its own, because both of them look like they are telling the truth.
func (a *App) adopt(sess *session.Session) {
	old := a.sess

	a.sess, a.conn = sess, sess.Conn
	ds := sess.Conn.DataSource()

	// The unlock is per session and per datasource both. Carrying an unlock
	// granted on stage over to production is the one way this feature could
	// undo the guard.
	a.status.writesEnabled = false

	// The rows on screen belong to the datasource that produced them, and
	// nothing about them is true of this one. The same goes for the schema
	// that was chosen and the definition last looked at: a name that exists
	// on both servers is the case where keeping them would mislead rather
	// than merely confuse.
	a.buf.Reset()
	a.content.unsort()
	a.grid.ScrollToBeginning()
	a.selectedSchema = ""
	a.ddlText = ""
	a.ddlView.SetText("")

	// Completion is scoped to the datasource in the cache, so it is rebuilt
	// rather than kept; a stale one offers tables that are not there.
	if a.cache != nil {
		a.completion = complete.New(a.cache.Names(), ds.Name, ds.Database)
	}

	// loadSchemas reloads the completion cache behind it, so the tree and
	// what completion offers move together.
	a.paintSpine()
	a.loadSchemas()

	a.status.phase = phaseIdle
	a.status.err = nil
	a.notice(fmt.Sprintf("switched to %s · %s", ds.Name, ds.Env))

	// Closed last, and off the interface's goroutine: Close waits on the
	// connection and then on the tunnel, and neither is something to hold a
	// redraw behind.
	go old.Close()
}

// paintSpine puts the current environment's colour on the frame.
func (a *App) paintSpine() {
	a.spine.SetBackgroundColor(envStyleFor(a.conn.DataSource().Env).bg)
}
