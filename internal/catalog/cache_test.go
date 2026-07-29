package catalog

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()

	c, err := OpenCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("OpenCache() error = %v, want nil", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func sampleSchema() Snapshot {
	return Snapshot{
		Schemas: []string{"app_db", "log_db"},
		Tables: map[string][]Table{
			"app_db": {
				{Name: "users"},
				{Name: "orders", Rows: 120},
				{Name: "user_view", IsView: true},
			},
			"log_db": {{Name: "events"}},
		},
		Columns: map[string][]Column{
			"app_db.users": {
				{Name: "id", Type: "int(11)", IsPrimaryKey: true},
				{Name: "email", Type: "varchar(255)"},
			},
			"app_db.orders": {{Name: "id", Type: "int(11)", IsPrimaryKey: true}},
		},
	}
}

func TestCacheRoundTrip(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Save(ctx, "prod-app", sampleSchema()); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	schemas, err := c.Schemas(ctx, "prod-app")
	if err != nil {
		t.Fatalf("Schemas() error = %v", err)
	}
	if len(schemas) != 2 || schemas[0] != "app_db" {
		t.Errorf("Schemas() = %v, want [app_db log_db]", schemas)
	}

	tables, err := c.Tables(ctx, "prod-app", "app_db")
	if err != nil {
		t.Fatalf("Tables() error = %v", err)
	}
	if len(tables) != 3 {
		t.Fatalf("Tables() = %v, want three entries", tables)
	}

	columns, err := c.Columns(ctx, "prod-app", "app_db", "users")
	if err != nil {
		t.Fatalf("Columns() error = %v", err)
	}
	if len(columns) != 2 || columns[0].Name != "id" {
		t.Errorf("Columns() = %v, want [id email]", columns)
	}
	if !columns[0].IsPrimaryKey {
		t.Error("the primary key flag was not preserved")
	}
}

// Column order is declaration order, which is also what "SELECT *" returns;
// losing it would make the completion list read strangely.
func TestCachePreservesColumnOrder(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	snap := Snapshot{
		Schemas: []string{"s"},
		Tables:  map[string][]Table{"s": {{Name: "t"}}},
		Columns: map[string][]Column{
			"s.t": {{Name: "zebra"}, {Name: "apple"}, {Name: "mango"}},
		},
	}
	if err := c.Save(ctx, "ds", snap); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	columns, err := c.Columns(ctx, "ds", "s", "t")
	if err != nil {
		t.Fatalf("Columns() error = %v", err)
	}
	for i, want := range []string{"zebra", "apple", "mango"} {
		if columns[i].Name != want {
			t.Errorf("column %d = %q, want %q", i, columns[i].Name, want)
		}
	}
}

// Datasources must not see each other's schemas.
func TestCacheKeepsDataSourcesApart(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Save(ctx, "prod", sampleSchema()); err != nil {
		t.Fatalf("Save(prod) error = %v", err)
	}
	other := Snapshot{
		Schemas: []string{"other_db"},
		Tables:  map[string][]Table{"other_db": {{Name: "t"}}},
	}
	if err := c.Save(ctx, "dev", other); err != nil {
		t.Fatalf("Save(dev) error = %v", err)
	}

	schemas, err := c.Schemas(ctx, "dev")
	if err != nil {
		t.Fatalf("Schemas(dev) error = %v", err)
	}
	if len(schemas) != 1 || schemas[0] != "other_db" {
		t.Errorf("Schemas(dev) = %v, want [other_db]", schemas)
	}
}

// A refresh replaces what was there; a table dropped on the server must stop
// being offered.
func TestSaveReplacesThePreviousSnapshot(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Save(ctx, "ds", sampleSchema()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	trimmed := Snapshot{
		Schemas: []string{"app_db"},
		Tables:  map[string][]Table{"app_db": {{Name: "users"}}},
		Columns: map[string][]Column{"app_db.users": {{Name: "id"}}},
	}
	if err := c.Save(ctx, "ds", trimmed); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	schemas, _ := c.Schemas(ctx, "ds")
	if len(schemas) != 1 {
		t.Errorf("Schemas() = %v, want only app_db after the refresh", schemas)
	}
	tables, _ := c.Tables(ctx, "ds", "app_db")
	if len(tables) != 1 {
		t.Errorf("Tables() = %v, want only users after the refresh", tables)
	}
}

// An empty cache answers rather than failing: the first run has nothing yet,
// and completion must degrade to offering nothing.
func TestEmptyCacheReturnsNothing(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	schemas, err := c.Schemas(ctx, "never-saved")
	if err != nil {
		t.Fatalf("Schemas() error = %v, want nil", err)
	}
	if len(schemas) != 0 {
		t.Errorf("Schemas() = %v, want none", schemas)
	}

	if _, err := c.Columns(ctx, "never-saved", "s", "t"); err != nil {
		t.Errorf("Columns() error = %v, want nil", err)
	}
}

// The cache is opened again on every start, so the schema must survive.
func TestCacheReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	ctx := context.Background()

	first, err := OpenCache(path)
	if err != nil {
		t.Fatalf("OpenCache() error = %v", err)
	}
	if err := first.Save(ctx, "ds", sampleSchema()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	first.Close()

	second, err := OpenCache(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer second.Close()

	schemas, err := second.Schemas(ctx, "ds")
	if err != nil {
		t.Fatalf("Schemas() error = %v", err)
	}
	if len(schemas) != 2 {
		t.Errorf("Schemas() = %v, want the saved snapshot", schemas)
	}
}

// The whole point of caching: a background refresh must not block the
// completion popup. Without WAL, the write would hold readers off.
func TestReadsSucceedDuringAWrite(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	if err := c.Save(ctx, "ds", sampleSchema()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// A snapshot big enough that the write takes measurable time.
	big := Snapshot{Schemas: []string{"big"}, Tables: map[string][]Table{}, Columns: map[string][]Column{}}
	tables := make([]Table, 2000)
	for i := range tables {
		tables[i] = Table{Name: "t" + itoa(i)}
		big.Columns["big.t"+itoa(i)] = []Column{{Name: "id"}, {Name: "value"}}
	}
	big.Tables["big"] = tables

	var wg sync.WaitGroup
	wg.Add(1)
	writeErr := make(chan error, 1)
	go func() {
		defer wg.Done()
		writeErr <- c.Save(ctx, "other", big)
	}()

	deadline := time.Now().Add(10 * time.Second)
	reads := 0
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := c.Schemas(readCtx, "ds")
		cancel()
		if err != nil {
			t.Fatalf("read during a write failed after %d reads: %v", reads, err)
		}
		reads++

		select {
		case err := <-writeErr:
			if err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			wg.Wait()
			t.Logf("completed %d reads while the write was in flight", reads)
			return
		default:
		}
	}
	t.Fatal("the write never finished")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
