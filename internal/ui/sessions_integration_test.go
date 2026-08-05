//go:build integration

package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/procs"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
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

// Stopping somebody else's statement, through the interface, end to end.
func TestStoppingAnotherSessionsStatement(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	stopOther := runElsewhere(t, "SELECT SLEEP(25)")
	defer stopOther()

	h.waitForOtherSession(t, "SLEEP(25)")

	h.do(keymap.ActionKillSession)
	h.waitFor("the picker", func(a *App) bool {
		front, _ := a.pages.GetFrontPage()
		return front == pageKill
	})

	h.typeInto("SLEEP(25)")
	h.press(tcell.KeyEnter)

	// Off production this is a button rather than a typed word.
	h.waitFor("the confirmation", func(a *App) bool {
		front, _ := a.pages.GetFrontPage()
		return front == pageConfirm
	})
	h.press(tcell.KeyRight)
	h.press(tcell.KeyEnter)

	if !h.waitForScreen("stopped the statement") {
		t.Fatalf("the kill was not reported:\n%s", h.text())
	}
	// And it is gone from the server, not merely reported.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		h.do(keymap.ActionSessions)
		h.waitFor("the sessions tab", func(a *App) bool { return a.resultTabs.current() == tabSessions })
		if !h.screenHas("SLEEP(25)") {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("the statement is still running:\n%s", h.text())
}

// This session's own connections are not offered at all. A row you can select
// and cannot act on is worse than one that is not there — and killing the
// control connection takes cancellation and every catalog read with it.
func TestThePickerDoesNotOfferThisSessionsOwnConnections(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	stopOther := runElsewhere(t, "SELECT SLEEP(25)")
	defer stopOther()

	h.waitForOtherSession(t, "SLEEP(25)")

	// The ids this session is holding, and the ids the picker offers.
	var ours []uint64
	h.inspect(func(a *App) bool {
		listing, err := procs.List(context.Background(), a.conn)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		for _, p := range listing.Processes {
			if a.conn.Owns(p.ID) {
				ours = append(ours, p.ID)
			}
		}
		return true
	})
	if len(ours) == 0 {
		t.Fatal("this session holds no connections; the test would prove nothing")
	}

	h.do(keymap.ActionKillSession)
	h.waitFor("the picker", func(a *App) bool {
		front, _ := a.pages.GetFrontPage()
		return front == pageKill
	})

	screen := h.text()
	for _, id := range ours {
		if strings.Contains(screen, fmt.Sprintf("%d  ", id)) {
			t.Errorf("connection %d is ours and was offered:\n%s", id, screen)
		}
	}
}

// waitForOtherSession blocks until a statement from somewhere else is visible.
func (h *harness) waitForOtherSession(t *testing.T, contains string) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.do(keymap.ActionSessions)
		h.waitFor("the sessions tab", func(a *App) bool { return a.resultTabs.current() == tabSessions })
		if h.screenHas(contains) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("no other session is running %q:\n%s", contains, h.text())
}

// The tab is composed while it is still hidden, so the width it is laid out
// for has to be the width it is eventually drawn at rather than the zero rect
// a hidden tab reports. It used to be the latter, and the twenty-column floor
// that fell back to folded every statement into a stack of four-word lines.
func TestTheProcessListUsesTheWholeWidthOfThePane(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionSessions)
	if !h.waitForScreen("information_schema") {
		t.Fatalf("the process list never appeared:\n%s", h.text())
	}

	// The listing quotes the statement it is itself running, which is long
	// enough to fold on any terminal. On a pane this wide it must fold near
	// the pane's width, not near the floor.
	widest := 0
	for _, line := range strings.Split(h.text(), "\n") {
		if strings.Contains(line, "information_schema") || strings.Contains(line, "SELECT") {
			if n := len(strings.TrimSpace(line)); n > widest {
				widest = n
			}
		}
	}
	if widest <= 30 {
		t.Errorf("the widest statement line is %d cells on a pane far wider:\n%s", widest, h.text())
	}
}
