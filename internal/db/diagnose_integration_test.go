//go:build integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// Diagnose reads error shapes the driver and the standard library produce,
// and neither promises to keep producing them. Synthesised errors in the unit
// test say what the classification does; these say it still matches what a
// real failure looks like.
func TestRealFailuresAreClassifiedTheWayTheUnitTestSaysTheyAre(t *testing.T) {
	for _, tt := range []struct {
		name    string
		break_  func(*config.DataSource, *string)
		want    Fault
		timeout time.Duration
	}{
		{"a password the server refuses",
			func(_ *config.DataSource, pw *string) { *pw = "not-the-password" },
			FaultCredentials, 10 * time.Second},
		{"a database that is not on the server",
			func(ds *config.DataSource, _ *string) { ds.Database = "dv_no_such_database" },
			FaultNoSuchDatabase, 10 * time.Second},
		{"a host name that does not resolve",
			func(ds *config.DataSource, _ *string) { ds.Host = "dv.no.such.host.invalid" },
			FaultUnresolved, 10 * time.Second},
		{"a port with nothing listening on it",
			func(ds *config.DataSource, _ *string) { ds.Port = 13999 },
			FaultRefused, 10 * time.Second},
		{"a certificate this machine will not verify",
			func(ds *config.DataSource, _ *string) { ds.TLS = config.TLSVerifyIdentity },
			FaultTLS, 10 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ds, password := testmysql.DataSource(t)
			tt.break_(ds, &password)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			conn, err := Open(ctx, ds, password, "")
			if err == nil {
				conn.Close()
				t.Fatal("the connection was supposed to fail and did not")
			}
			if got := Diagnose(err); got != tt.want {
				t.Errorf("Diagnose(%v) = %v, want %v", err, got, tt.want)
			}
		})
	}
}

// The other refusal about a database, and the one the unit test cannot
// produce from a number alone: the password was accepted and the database
// was not. Reading it as a credential failure sends the reader to dv auth
// to re-enter a password the server had already taken.
func TestADatabaseTheAccountMayNotUseIsNotReadAsARefusedPassword(t *testing.T) {
	const account = "'dv_denied'@'%'"

	root := openTestConn(t)
	mustExec(t, root, "DROP USER IF EXISTS "+account)
	// No password: an account with one would have to survive both servers'
	// default authentication plugins, which differ, and the grant is the
	// only part of this that the test is about.
	mustExec(t, root, "CREATE USER "+account)
	t.Cleanup(func() { mustExec(t, root, "DROP USER IF EXISTS "+account) })

	ds, _ := testmysql.DataSource(t)
	ds.User = "dv_denied"
	// A database that is on every server and that an account with no grants
	// cannot open, so the refusal is about the grant rather than the name.
	ds.Database = "mysql"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := Open(ctx, ds, "", "")
	if err == nil {
		conn.Close()
		t.Fatal("an account with no grants opened the mysql database")
	}
	if got := Diagnose(err); got != FaultDatabaseDenied {
		t.Errorf("Diagnose(%v) = %v, want %v", err, got, FaultDatabaseDenied)
	}
}
