//go:build integration

package catalog

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func TestFetchSnapshotReadsTheWholeSchema(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	mustExec(t, conn, "DROP TABLE IF EXISTS dv_snap")
	mustExec(t, conn, `CREATE TABLE dv_snap (
		id INT PRIMARY KEY,
		label VARCHAR(50) NOT NULL,
		note TEXT NULL
	)`)
	t.Cleanup(func() { mustExec(t, conn, "DROP TABLE IF EXISTS dv_snap") })

	snap, err := FetchSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v, want nil", err)
	}

	if !contains(snap.Schemas, testmysql.DefaultDatabase) {
		t.Fatalf("Schemas = %v, want it to include %q", snap.Schemas, testmysql.DefaultDatabase)
	}

	var found bool
	for _, tbl := range snap.Tables[testmysql.DefaultDatabase] {
		if tbl.Name == "dv_snap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Tables = %v, want it to include dv_snap", snap.Tables[testmysql.DefaultDatabase])
	}

	columns := snap.Columns[ColumnKey(testmysql.DefaultDatabase, "dv_snap")]
	if len(columns) != 3 {
		t.Fatalf("Columns = %v, want three entries", columns)
	}
	// Declaration order is what "SELECT *" returns, so it has to survive.
	if columns[0].Name != "id" || columns[1].Name != "label" || columns[2].Name != "note" {
		t.Errorf("column order = %v, want [id label note]", names(columns))
	}
	if !columns[0].IsPrimaryKey {
		t.Error("the primary key was not recorded")
	}
	if columns[1].Nullable {
		t.Error("label reported as nullable, want NOT NULL")
	}
}

// The full round trip: read from the server, cache it, read it back.
func TestSnapshotSurvivesTheCache(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	mustExec(t, conn, "DROP TABLE IF EXISTS dv_snap_cached")
	mustExec(t, conn, "CREATE TABLE dv_snap_cached (id INT PRIMARY KEY, email VARCHAR(255))")
	t.Cleanup(func() { mustExec(t, conn, "DROP TABLE IF EXISTS dv_snap_cached") })

	snap, err := FetchSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	cache, err := OpenCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("OpenCache() error = %v", err)
	}
	defer cache.Close()

	if err := cache.Save(ctx, "integration", snap); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	columns, err := cache.Columns(ctx, "integration", testmysql.DefaultDatabase, "dv_snap_cached")
	if err != nil {
		t.Fatalf("Columns() error = %v", err)
	}
	if len(columns) != 2 || columns[1].Name != "email" {
		t.Errorf("cached columns = %v, want [id email]", names(columns))
	}
}

// Completion runs on a keystroke, so the cached lookup has to be fast enough
// to feel instant — far faster than information_schema, which is the reason
// the cache exists at all.
func TestCachedLookupIsFasterThanTheServer(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	snap, err := FetchSnapshot(ctx, conn)
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	cache, err := OpenCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("OpenCache() error = %v", err)
	}
	defer cache.Close()

	if err := cache.Save(ctx, "integration", snap); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Warm both paths before timing.
	cache.Tables(ctx, "integration", testmysql.DefaultDatabase)
	Tables(ctx, conn, testmysql.DefaultDatabase)

	start := time.Now()
	for i := 0; i < 20; i++ {
		if _, err := cache.Tables(ctx, "integration", testmysql.DefaultDatabase); err != nil {
			t.Fatalf("cached Tables() error = %v", err)
		}
	}
	cached := time.Since(start) / 20

	start = time.Now()
	for i := 0; i < 20; i++ {
		if _, err := Tables(ctx, conn, testmysql.DefaultDatabase); err != nil {
			t.Fatalf("server Tables() error = %v", err)
		}
	}
	server := time.Since(start) / 20

	t.Logf("cached lookup %v, server lookup %v", cached, server)
	if cached > server {
		t.Errorf("the cache (%v) is slower than the server (%v); it has no purpose", cached, server)
	}
}
