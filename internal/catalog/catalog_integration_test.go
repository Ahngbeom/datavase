//go:build integration

package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func openTestConn(t *testing.T) *db.Conn {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v, want nil", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestSchemasExcludesServerInternals(t *testing.T) {
	conn := openTestConn(t)

	got, err := Schemas(context.Background(), conn)
	if err != nil {
		t.Fatalf("Schemas() error = %v, want nil", err)
	}
	if len(got) == 0 {
		t.Fatal("Schemas() returned nothing")
	}

	// The server's own schemas are noise in a tree meant for application
	// data; DataGrip and DBeaver hide them by default too.
	hidden := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}
	for _, s := range got {
		if hidden[s] {
			t.Errorf("Schemas() included the internal schema %q", s)
		}
	}

	if !contains(got, testmysql.DefaultDatabase) {
		t.Errorf("Schemas() = %v, want it to include %q", got, testmysql.DefaultDatabase)
	}
}

func TestTablesReportsNamesAndKind(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	mustExec(t, conn, "DROP VIEW IF EXISTS dv_cat_view")
	mustExec(t, conn, "DROP TABLE IF EXISTS dv_cat_table")
	mustExec(t, conn, "CREATE TABLE dv_cat_table (id INT PRIMARY KEY, name VARCHAR(50))")
	mustExec(t, conn, "CREATE VIEW dv_cat_view AS SELECT id FROM dv_cat_table")
	t.Cleanup(func() {
		mustExec(t, conn, "DROP VIEW IF EXISTS dv_cat_view")
		mustExec(t, conn, "DROP TABLE IF EXISTS dv_cat_table")
	})

	got, err := Tables(ctx, conn, testmysql.DefaultDatabase)
	if err != nil {
		t.Fatalf("Tables() error = %v, want nil", err)
	}

	byName := map[string]Table{}
	for _, tbl := range got {
		byName[tbl.Name] = tbl
	}

	table, ok := byName["dv_cat_table"]
	if !ok {
		t.Fatalf("Tables() = %v, want it to include dv_cat_table", got)
	}
	if table.IsView {
		t.Error("dv_cat_table reported as a view")
	}

	view, ok := byName["dv_cat_view"]
	if !ok {
		t.Fatalf("Tables() = %v, want it to include dv_cat_view", got)
	}
	if !view.IsView {
		t.Error("dv_cat_view not reported as a view")
	}
}

func TestColumnsReportsTypeNullabilityAndKey(t *testing.T) {
	conn := openTestConn(t)

	mustExec(t, conn, "DROP TABLE IF EXISTS dv_cat_cols")
	mustExec(t, conn, `CREATE TABLE dv_cat_cols (
		id INT PRIMARY KEY,
		email VARCHAR(255) NOT NULL,
		note TEXT NULL
	)`)
	t.Cleanup(func() { mustExec(t, conn, "DROP TABLE IF EXISTS dv_cat_cols") })

	got, err := Columns(context.Background(), conn, testmysql.DefaultDatabase, "dv_cat_cols")
	if err != nil {
		t.Fatalf("Columns() error = %v, want nil", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(Columns()) = %d, want 3", len(got))
	}

	// Ordinal order matters: it is the order the table was declared in, and
	// the order a "SELECT *" returns.
	if got[0].Name != "id" || got[1].Name != "email" || got[2].Name != "note" {
		t.Errorf("column order = %v, want [id email note]", names(got))
	}
	if !got[0].IsPrimaryKey {
		t.Error("id not reported as the primary key")
	}
	if got[1].Nullable {
		t.Error("email reported as nullable, want NOT NULL")
	}
	if !got[2].Nullable {
		t.Error("note reported as NOT NULL, want nullable")
	}
	if got[1].Type == "" {
		t.Error("email has an empty type")
	}
}

// A name typed by the user reaches these queries, so it must be bound as a
// parameter rather than pasted into the SQL.
func TestCatalogQueriesAreNotVulnerableToInjectedNames(t *testing.T) {
	conn := openTestConn(t)

	evil := "x' OR '1'='1"
	got, err := Columns(context.Background(), conn, testmysql.DefaultDatabase, evil)
	if err != nil {
		t.Fatalf("Columns() error = %v, want nil for an unknown table", err)
	}
	if len(got) != 0 {
		t.Errorf("Columns(%q) returned %d columns, want none", evil, len(got))
	}
}

func TestColumnsForAnUnknownTableIsEmpty(t *testing.T) {
	conn := openTestConn(t)

	got, err := Columns(context.Background(), conn, testmysql.DefaultDatabase, "no_such_table")
	if err != nil {
		t.Fatalf("Columns() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("len(Columns()) = %d, want 0", len(got))
	}
}

func mustExec(t *testing.T, conn *db.Conn, sql string) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), sql); err != nil {
		t.Fatalf("Exec(%q) error = %v", sql, err)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func names(cols []Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Name
	}
	return out
}
