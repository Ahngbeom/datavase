//go:build integration

package procs

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func open(t *testing.T, user, password string) *db.Conn {
	t.Helper()

	ds, ownPassword := testmysql.DataSource(t)
	if user != "" {
		ds.User, ownPassword = user, password
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, ownPassword, "")
	if err != nil {
		t.Fatalf("db.Open(%s) error = %v", ds.User, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// The listing has to show the connection that is actually working, which is
// the one it is opened to find.
func TestListSeesAStatementRunningOnAnotherConnection(t *testing.T) {
	watcher := open(t, "", "")
	busy := open(t, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// A statement held open on a connection of its own, cancelled when the
	// test ends rather than waited out.
	sleeping, cancelSleep := context.WithCancel(context.Background())
	defer cancelSleep()

	streams := make(chan *db.Stream, 1)
	go func() {
		stream := busy.Query(sleeping, "SELECT SLEEP(10)", db.Options{})
		streams <- stream
		defer stream.Close()
		for range stream.Events {
		}
	}()
	// Cancel rather than only dropping the context: the context detaches the
	// client and leaves the server running the statement to the end, on a
	// server the rest of the suite is sharing.
	defer func() {
		if stream := <-streams; stream != nil {
			stream.Cancel()
		}
	}()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		listing, err := List(ctx, watcher)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if !listing.Complete {
			t.Fatal("the test user cannot see other connections; the listing would prove nothing")
		}

		for i, p := range listing.Processes {
			if !p.Working() || !strings.Contains(p.SQL, "SLEEP(10)") {
				continue
			}

			// Ordered ahead of every idle connection. Deliberately not "first
			// in the list": the suite runs its packages in parallel against
			// one server, so another test's statement may well be older and
			// belongs above this one — which is the ordering working, not
			// failing.
			for _, other := range listing.Processes[i+1:] {
				if !other.Working() {
					continue
				}
				if other.Elapsed > p.Elapsed {
					t.Errorf("a shorter statement sorted above a longer one:\n%+v", listing.Processes)
				}
			}
			for j, before := range listing.Processes[:i] {
				if !before.Working() {
					t.Errorf("an idle connection at %d sorted above the running statement at %d:\n%+v",
						j, i, listing.Processes)
				}
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("the running statement never appeared in the listing")
}

// Without the PROCESS privilege the server answers with the user's own
// connections and no error at all, so the listing has to say the view is
// partial rather than let a short list read as a quiet server.
func TestAUserWithoutThePrivilegeIsToldTheViewIsPartial(t *testing.T) {
	grantLimitedUser(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	full, err := List(ctx, open(t, "", ""))
	if err != nil {
		t.Fatalf("List() as the test user error = %v", err)
	}
	if !full.Complete {
		t.Error("the test user has PROCESS and the listing says otherwise")
	}

	limited, err := List(ctx, open(t, limitedUser, limitedPassword))
	if err != nil {
		t.Fatalf("List() as the limited user error = %v", err)
	}
	if limited.Complete {
		t.Error("a user without PROCESS was told the listing was complete")
	}
}

const (
	limitedUser     = "dv_procs_limited"
	limitedPassword = "limited"
)

// grantLimitedUser makes a user that may connect and read, and may not see
// anyone else's connections.
func grantLimitedUser(t *testing.T) {
	t.Helper()

	conn := open(t, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, stmt := range []string{
		"CREATE USER IF NOT EXISTS '" + limitedUser + "'@'%' IDENTIFIED BY '" + limitedPassword + "'",
		"GRANT SELECT ON *.* TO '" + limitedUser + "'@'%'",
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Skipf("cannot create the limited user (%v); this server does not let the test user grant", err)
		}
	}
}
