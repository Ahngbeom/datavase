package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/complete"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/session"
)

// connectTimeout bounds a switch. The interface is unresponsive while it
// waits, and a bastion that is not answering should give the session back
// rather than hold it.
const connectTimeout = 15 * time.Second

// showDataSources opens the list: connect to another, or change the file.
func (a *App) showDataSources() {
	if a.connect == nil {
		a.notice("this session cannot switch datasource")
		return
	}

	save := func() error {
		if a.configPath == "" {
			return errors.New("no configuration file to save to")
		}
		return config.Save(a.configPath, a.cfg)
	}
	a.picker = newDSPicker(a.app, pickerDeps{
		cfg:     a.cfg,
		current: a.conn.DataSource().Name,
		save:    save,
		secrets: a.secrets,
		probe:   a.probe,
		close:   a.closeDataSources,
		connect: func(ds *config.DataSource) {
			a.closeDataSources()
			a.switchTo(ds)
		},
	})
	a.pages.AddPage(pageDataSource, centred(a.picker.Primitive(), 80, 24), true, true)
	a.app.SetFocus(a.picker.Primitive())
}

func (a *App) closeDataSources() {
	a.pages.RemovePage(pageDataSource)
	a.picker = nil
	a.app.SetFocus(a.editor)
}

// switchTo moves the session to another datasource, asking about anything it
// would throw away first.
func (a *App) switchTo(ds *config.DataSource) {
	if ds.Name == a.conn.DataSource().Name {
		a.notice("already on " + ds.Name)
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
// half-switched interface — the new connection behind the old datasource's
// tables in the tree — is worse than either state on its own, because both
// of them look like they are telling the truth.
func (a *App) adopt(sess *session.Session) {
	old := a.sess

	a.sess, a.conn = sess, sess.Conn
	ds := sess.Conn.DataSource()

	// The rows on screen belong to the datasource that produced them, and
	// nothing about them is true of this one. The same goes for the schema
	// that was chosen: a name that exists on both servers is the case where
	// keeping it would mislead rather than merely confuse.
	a.buf.Reset()
	a.content.unsort()
	a.grid.ScrollToBeginning()
	a.selectedSchema = ""

	// Completion is scoped to the datasource in the cache, so it is rebuilt
	// rather than kept; a stale one offers tables that are not there.
	if a.cache != nil {
		a.completion = complete.New(a.cache.Names(), ds.Name, ds.Database)
	}

	// loadSchemas reloads the completion cache behind it, so the tree and
	// what completion offers move together.
	a.loadSchemas()

	a.status.phase = phaseIdle
	a.status.err = nil
	a.notice(fmt.Sprintf("switched to %s", ds.Name))

	// Closed last, and off the interface's goroutine: Close waits on the
	// connection and then on the tunnel, and neither is something to hold a
	// redraw behind.
	go old.Close()
}
