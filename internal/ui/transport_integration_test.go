//go:build integration

package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/Ahngbeom/datavase/internal/testssh"
	"golang.org/x/crypto/ssh"
)

// newBastionHarness opens the interface over a real forwarded connection, so
// that taking the bastion away is the thing that actually happens rather than
// something stood in for.
func newBastionHarness(t *testing.T) (*harness, *testssh.Server) {
	t.Helper()

	srv := testssh.Start(t)
	ds, password := testmysql.DataSource(t)
	ds.Tunnel = srv.Bastion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sess, err := session.OpenWith(ctx, ds, password, session.Options{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(testssh.Signer(t))},
		HostKeyCallback: ssh.FixedHostKey(srv.HostKey),
	})
	if err != nil {
		t.Fatalf("OpenWith() error = %v", err)
	}
	return harnessOver(t, sess, ds), srv
}

// The failure this issue is about. The driver reports a socket that has gone,
// which reads as the database being in trouble — and the difference between
// that and a bastion is the difference between reconnecting and paging whoever
// owns the database.
func TestAStatementFailingWithTheBastionGoneNamesTheBastion(t *testing.T) {
	h, srv := newBastionHarness(t)

	// Working first, so what follows is a session that broke rather than one
	// that never worked.
	h.typeSQL("SELECT 1 AS before_it_went")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("before_it_went") {
		t.Fatalf("the session did not work through the bastion; screen:\n%s", h.text())
	}

	srv.Stop()

	// The driver retries on a fresh connection, and that is the attempt with
	// no bastion left to carry it.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.typeSQL("SELECT 1 AS after_it_went")
		h.do(keymap.ActionRun)
		h.waitFor("the statement to finish", func(a *App) bool { return a.running == nil })

		if h.screenHas(srv.Bastion(t).Host) {
			return
		}
	}
	t.Fatalf("the bastion is gone and nothing on screen says so:\n%s", h.text())
}

// A statement the server refused is proof the server was reached, so it keeps
// its own message however the tunnel is doing. Tunnel.Err never clears, so
// without this every later typo would read as a dead bastion.
func TestAServerRefusalKeepsItsOwnMessageThroughABastion(t *testing.T) {
	h, _ := newBastionHarness(t)

	h.typeSQL("SELECT * FROM dv_no_such_table_at_all")
	h.do(keymap.ActionRun)

	if !h.waitForScreen("dv_no_such_table_at_all") {
		t.Fatalf("the server's own refusal was not shown; screen:\n%s", h.text())
	}
}

// screenHas is waitForScreen without the failure: this loop is asking whether
// the message has arrived yet, not asserting that it has.
func (h *harness) screenHas(want string) bool {
	h.t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(h.text(), want) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
