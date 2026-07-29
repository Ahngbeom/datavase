//go:build integration

package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// Choosing a schema in the interface has to reach the server, or an
// unqualified query silently keeps hitting the one connected to — which is
// how someone reads production data believing they are on staging.
func TestQueryRunsAgainstTheChosenSchema(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	got := drain(t, conn.Query(ctx, "SELECT DATABASE()", Options{Schema: "information_schema"}))
	if got.err != nil {
		t.Fatalf("Err() = %v, want nil", got.err)
	}
	if len(got.rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(got.rows))
	}
	if name := database(got); name != "information_schema" {
		t.Errorf("DATABASE() = %q, want the chosen schema", name)
	}
}

// Without a schema the connection's own default stands.
func TestQueryWithoutASchemaUsesTheDefault(t *testing.T) {
	conn := openTestConn(t)

	got := drain(t, conn.Query(context.Background(), "SELECT DATABASE()", Options{}))
	if name := database(got); name != testmysql.DefaultDatabase {
		t.Errorf("DATABASE() = %q, want %q", name, testmysql.DefaultDatabase)
	}
}

// The switch must not leak: connections go back to a shared pool, and one
// left pointing at another schema would take the next query with it.
func TestSchemaDoesNotLeakToTheNextQuery(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	drain(t, conn.Query(ctx, "SELECT DATABASE()", Options{Schema: "information_schema"}))

	got := drain(t, conn.Query(ctx, "SELECT DATABASE()", Options{}))
	if name := database(got); name != testmysql.DefaultDatabase {
		t.Errorf("DATABASE() = %q after a switch, want %q", name, testmysql.DefaultDatabase)
	}
}

// A schema that does not exist has to fail as a schema error, not as a
// puzzling failure of the statement itself.
func TestQueryRejectsAnUnknownSchema(t *testing.T) {
	conn := openTestConn(t)

	got := drain(t, conn.Query(context.Background(), "SELECT 1",
		Options{Schema: "no_such_schema_here"}))

	if got.err == nil {
		t.Fatal("Err() = nil, want a failure for an unknown schema")
	}
}

// An identifier is pasted into USE rather than bound, so a name that tries to
// close the quote must fail rather than execute.
func TestQueryRejectsAnInjectedSchema(t *testing.T) {
	conn := openTestConn(t)

	got := drain(t, conn.Query(context.Background(), "SELECT 1",
		Options{Schema: "x`; DROP TABLE dv_seq; --"}))

	if got.err == nil {
		t.Fatal("Err() = nil, want a failure for an injected schema name")
	}
}

// database reads the single DATABASE() value out of a drained stream.
func database(got collected) string {
	if len(got.rows) == 0 || len(got.rows[0]) == 0 {
		return ""
	}
	if b, ok := got.rows[0][0].([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(got.rows[0][0])
}
