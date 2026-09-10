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
