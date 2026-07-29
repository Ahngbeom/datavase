//go:build integration

package catalog

import (
	"context"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func TestTableDDLReturnsTheCreateStatement(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	mustExec(t, conn, "DROP TABLE IF EXISTS dv_ddl")
	mustExec(t, conn, `CREATE TABLE dv_ddl (
		id INT NOT NULL AUTO_INCREMENT,
		email VARCHAR(255) NOT NULL,
		PRIMARY KEY (id)
	)`)
	t.Cleanup(func() { mustExec(t, conn, "DROP TABLE IF EXISTS dv_ddl") })

	ddl, err := TableDDL(ctx, conn, testmysql.DefaultDatabase, "dv_ddl", false)
	if err != nil {
		t.Fatalf("TableDDL() error = %v, want nil", err)
	}

	for _, want := range []string{"CREATE TABLE", "dv_ddl", "email", "PRIMARY KEY"} {
		if !strings.Contains(ddl, want) {
			t.Errorf("the definition is missing %q:\n%s", want, ddl)
		}
	}
}

// A view needs SHOW CREATE VIEW; asking for a table would fail outright.
func TestTableDDLHandlesViews(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	mustExec(t, conn, "DROP VIEW IF EXISTS dv_ddl_view")
	mustExec(t, conn, "DROP TABLE IF EXISTS dv_ddl_base")
	mustExec(t, conn, "CREATE TABLE dv_ddl_base (id INT PRIMARY KEY)")
	mustExec(t, conn, "CREATE VIEW dv_ddl_view AS SELECT id FROM dv_ddl_base")
	t.Cleanup(func() {
		mustExec(t, conn, "DROP VIEW IF EXISTS dv_ddl_view")
		mustExec(t, conn, "DROP TABLE IF EXISTS dv_ddl_base")
	})

	ddl, err := TableDDL(ctx, conn, testmysql.DefaultDatabase, "dv_ddl_view", true)
	if err != nil {
		t.Fatalf("TableDDL() error = %v, want nil", err)
	}
	if !strings.Contains(ddl, "CREATE") || !strings.Contains(ddl, "VIEW") {
		t.Errorf("the definition does not look like a view:\n%s", ddl)
	}
}

// Identifiers are pasted into the statement rather than bound, so a name
// containing a backtick has to survive the round trip.
func TestTableDDLHandlesAwkwardNames(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	const name = "dv_we`ird"
	quoted := QuoteIdentifier(name)

	mustExec(t, conn, "DROP TABLE IF EXISTS "+quoted)
	mustExec(t, conn, "CREATE TABLE "+quoted+" (id INT PRIMARY KEY)")
	t.Cleanup(func() { mustExec(t, conn, "DROP TABLE IF EXISTS "+quoted) })

	ddl, err := TableDDL(ctx, conn, testmysql.DefaultDatabase, name, false)
	if err != nil {
		t.Fatalf("TableDDL(%q) error = %v, want nil", name, err)
	}
	if !strings.Contains(ddl, "CREATE TABLE") {
		t.Errorf("the definition is missing for a backticked name:\n%s", ddl)
	}
}

// A name that tries to close the quote and append SQL must fail as a missing
// table, not execute.
func TestTableDDLRejectsAnInjectedName(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	if _, err := TableDDL(ctx, conn, testmysql.DefaultDatabase,
		"nope`; DROP TABLE dv_seq; --", false); err == nil {
		t.Fatal("TableDDL() error = nil, want a failure for a nonexistent table")
	}

	// dv_seq must still be there.
	if _, err := conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS dv_seq (n INT PRIMARY KEY)"); err != nil {
		t.Fatalf("recreating dv_seq: %v", err)
	}
	tables, err := Tables(ctx, conn, testmysql.DefaultDatabase)
	if err != nil {
		t.Fatalf("Tables() error = %v", err)
	}
	var found bool
	for _, tbl := range tables {
		if tbl.Name == "dv_seq" {
			found = true
		}
	}
	if !found {
		t.Error("dv_seq is gone; the injected name was executed")
	}
}

func TestTableDDLForAMissingTable(t *testing.T) {
	conn := openTestConn(t)

	if _, err := TableDDL(context.Background(), conn,
		testmysql.DefaultDatabase, "no_such_table_here", false); err == nil {
		t.Error("TableDDL() error = nil, want an error for a missing table")
	}
}
