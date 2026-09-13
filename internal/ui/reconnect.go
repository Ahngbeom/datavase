package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/session"
)

// lostSession reports whether err means the session is gone rather than the
// statement being refused.
//
// A numbered MySQL error is proof the statement arrived and was answered, so
// whatever else is wrong, the connection that carried it is alive. Anything
// else could have failed at any hop, and by the time this is asked the
// cancellations have already been handled elsewhere — so what is left is a
// session to stop trusting.
func lostSession(err error) bool {
	return err != nil && !db.ReachedServer(err)
}

// reconnect opens a new session on the same datasource, on one keystroke and
// never on its own.
//
// Nothing is re-run. The statement in the editor failed against a session
// that no longer exists, and sending it again — at the moment the connection
// underneath has just been replaced — is the one thing someone recovering
// from a dropped connection did not ask for.
//
// The old session is let go only after the new one is open, the same order a
// datasource switch uses: a reconnection that fails leaves the interface
// where it was, with the message about what is wrong still on screen.
func (a *App) reconnect() {
	ds := a.conn.DataSource()
	if a.connect == nil {
		a.notice("this session cannot reconnect; restart dv")
		return
	}

	a.notice("reconnecting to " + ds.Name + "…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()

		sess, err := a.connect(ctx, ds)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.status.phase = phaseFailed
				a.status.err = fmt.Errorf("reconnecting to %s: %w", ds.Name, err)
				return
			}
			a.adoptReconnected(sess)
		})
	}()
}

// adoptReconnected puts the replacement session in place of the dead one.
//
// What it keeps apart from a datasource switch is the chosen schema, and
// that is the point: dropping back to the datasource's default without a
// word is how the right statement comes to be run against the wrong schema.
// It is the same server, so the name still means what it meant.
func (a *App) adoptReconnected(sess *session.Session) {
	old := a.sess
	a.sess, a.conn = sess, sess.Conn
	a.sessionLost = false

	// The transaction died with the connection that held it. The marker for
	// one is local state, and left alone it would claim work is pending on a
	// session that never had any.
	a.status.inTransaction = false
	a.status.phase = phaseIdle
	a.status.err = nil

	a.loadSchemas()
	a.notice(newSessionSummary(a.currentSchema(), a.conn.ReadOnlyConfirmed()))

	// Closed last and off this goroutine: Close waits on the connection and
	// then on the tunnel, and neither is something to hold a redraw behind.
	go old.Close()
}

// newSessionSummary says what the session that just replaced the dead one
// is, in the two terms that decide what the next statement does.
func newSessionSummary(schema string, readOnly bool) string {
	parts := []string{"reconnected"}
	if schema != "" {
		parts = append(parts, "schema "+schema)
	}
	if readOnly {
		parts = append(parts, "read-only")
	}
	return strings.Join(parts, " · ")
}
