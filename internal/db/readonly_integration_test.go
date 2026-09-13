//go:build integration

package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func openReadOnlyTestConn(t *testing.T) *Conn {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ds.ReadOnly = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// A datasource marked read_only must refuse the write at the server, so that
// no statement the interface forgot to inspect can slip past.
func TestAReadOnlyDataSourceRefusesAWriteAndStillReads(t *testing.T) {
	table := createTempTable(t, openTestConn(t))
	conn := openReadOnlyTestConn(t)
	ctx := context.Background()

	got := drain(t, conn.Query(ctx, "INSERT INTO "+table+" (v) VALUES (1)", Options{Exec: true}))
	if got.err == nil || !strings.Contains(strings.ToUpper(got.err.Error()), "READ ONLY") {
		t.Fatalf("the write went through on a read-only datasource: err = %v", got.err)
	}

	got = drain(t, conn.Query(ctx, "SELECT COUNT(*) FROM "+table, Options{}))
	if got.err != nil {
		t.Fatalf("a read failed on a read-only datasource: %v", got.err)
	}
	if len(got.rows) != 1 || got.rows[0][0] != int64(0) {
		t.Errorf("rows = %v, want the table still empty", got.rows)
	}
}

// The transaction path pins its own connection, so read-only has to hold
// there too or "BEGIN; UPDATE …; COMMIT" would be the way round it.
func TestAReadOnlyDataSourceRefusesAWriteInsideATransaction(t *testing.T) {
	table := createTempTable(t, openTestConn(t))
	conn := openReadOnlyTestConn(t)
	ctx := context.Background()

	if err := conn.Begin(ctx); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer conn.Rollback(ctx)

	got := drain(t, conn.Query(ctx, "INSERT INTO "+table+" (v) VALUES (1)", Options{Exec: true}))
	if got.err == nil || !strings.Contains(strings.ToUpper(got.err.Error()), "READ ONLY") {
		t.Fatalf("the write went through inside a transaction on a read-only datasource: err = %v", got.err)
	}
}

// What the server says about the session, not what the configuration asked
// for. The interface's marker and every refused write rest on this answer,
// and a marker taken from the configuration would say "read-only" over a
// session that is not.
func TestTheServerReportsWhetherASessionIsReadOnly(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	c, err := conn.pool.Conn(ctx)
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	defer c.Close()

	ro, err := sessionReadOnly(ctx, c)
	if err != nil {
		t.Fatalf("sessionReadOnly() error = %v", err)
	}
	if ro {
		t.Error("a writable session reported itself read-only")
	}

	if _, err := c.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
		t.Fatalf("making the session read-only: %v", err)
	}

	ro, err = sessionReadOnly(ctx, c)
	if err != nil {
		t.Fatalf("sessionReadOnly() error = %v", err)
	}
	if !ro {
		t.Error("a read-only session reported itself writable")
	}
}

// Opening a read_only datasource asks the server to confirm it, and the
// answer is what the session carries from then on.
func TestOpeningAReadOnlyDataSourceConfirmsItWithTheServer(t *testing.T) {
	if conn := openReadOnlyTestConn(t); !conn.ReadOnlyConfirmed() {
		t.Error("a read_only datasource opened without the server confirming the session")
	}
	if conn := openTestConn(t); conn.ReadOnlyConfirmed() {
		t.Error("an ordinary datasource reported a confirmed read-only session")
	}
}

// A transaction takes its own connection, and the statements inside it never
// pass through the check every other statement pays for. START TRANSACTION
// READ ONLY stops the writes, but it leaves the session itself writable, so
// the one connection a user holds longest is the one nothing confirmed.
func TestATransactionOnAReadOnlyDataSourceRunsOnAConfirmedSession(t *testing.T) {
	conn := openReadOnlyTestConn(t)
	ctx := context.Background()

	if err := conn.Begin(ctx); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer conn.Rollback(ctx)

	ro, err := sessionReadOnly(ctx, conn.tx)
	if err != nil {
		t.Fatalf("sessionReadOnly() error = %v", err)
	}
	if !ro {
		t.Error("the transaction is running on a session the server never confirmed as read-only")
	}
}
