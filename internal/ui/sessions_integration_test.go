//go:build integration

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// The question this exists for: something else is running, and this session
// can see it. Held on a connection of its own, because a session cannot answer
// it about itself.
func TestTheSessionsTabShowsAStatementRunningElsewhere(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	stopOther := runElsewhere(t, "SELECT SLEEP(20)")
	defer stopOther()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.do(keymap.ActionSessions)
		h.waitFor("the sessions tab", func(a *App) bool { return a.resultTabs.current() == tabSessions })

		if h.screenHas("SLEEP(20)") {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("the other session's statement never appeared:\n%s", h.text())
}

// A short list and a quiet server look identical, and the difference is the
// whole reason to have looked.
func TestAUserWhoCannotSeeOtherSessionsIsTold(t *testing.T) {
	makeLimitedUser(t)

	ds, _ := testmysql.DataSource(t)
	ds.User = uiLimitedUser

	h := newHarnessAs(t, ds, uiLimitedPassword)
	stopOther := runElsewhere(t, "SELECT SLEEP(20)")
	defer stopOther()

	h.do(keymap.ActionSessions)
	h.waitFor("the sessions tab", func(a *App) bool { return a.resultTabs.current() == tabSessions })

	if !h.screenHas("no PROCESS privilege") {
		t.Fatalf("a partial view was not admitted:\n%s", h.text())
	}
	// And it really is partial, so the message is not merely decorative.
	if h.screenHas("SLEEP(20)") {
		t.Error("the limited user could see another session after all; the test proves nothing")
	}
}

// runElsewhere holds a statement open on a connection of its own, and returns
// the way to stop it.
func runElsewhere(t *testing.T, sql string) func() {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	open, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Open(open, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}

	running, stop := context.WithCancel(context.Background())
	streams := make(chan *db.Stream, 1)
	go func() {
		stream := conn.Query(running, sql, db.Options{})
		streams <- stream
		defer stream.Close()
		for range stream.Events {
		}
	}()

	return func() {
		// Cancel rather than only dropping the context. Cancelling the
		// context detaches the client and leaves the server running the
		// statement to the end — twenty seconds of work on a server the rest
		// of the suite is sharing.
		if stream := <-streams; stream != nil {
			stream.Cancel()
		}
		stop()
		conn.Close()
	}
}

const (
	uiLimitedUser     = "dv_ui_limited"
	uiLimitedPassword = "limited"
)

func makeLimitedUser(t *testing.T) {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	defer conn.Close()

	for _, stmt := range []string{
		"CREATE USER IF NOT EXISTS '" + uiLimitedUser + "'@'%' IDENTIFIED BY '" + uiLimitedPassword + "'",
		"GRANT SELECT ON *.* TO '" + uiLimitedUser + "'@'%'",
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Skipf("cannot create the limited user (%v)", err)
		}
	}
}

// newHarnessAs opens the interface as a given user, so a test can be about
// what that user is allowed to see.
func newHarnessAs(t *testing.T, ds *config.DataSource, password string) *harness {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open(%s) error = %v", ds.User, err)
	}
	return harnessOver(t, &session.Session{Conn: conn}, ds)
}
