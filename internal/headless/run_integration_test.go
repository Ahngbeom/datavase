//go:build integration

package headless

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func openConn(t *testing.T, readOnly bool) *db.Conn {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ds.ReadOnly = readOnly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// threeRowTable creates a table of three rows through a writable connection
// and drops it when the test ends.
func threeRowTable(t *testing.T) string {
	t.Helper()

	conn := openConn(t, false)
	ctx := context.Background()
	table := "headless_" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS " + table,
		"CREATE TABLE " + table + " (n INT)",
		"INSERT INTO " + table + " VALUES (1), (2), (3)",
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() { conn.Exec(context.Background(), "DROP TABLE IF EXISTS "+table) })
	return table
}

func mustAdmit(t *testing.T, sql string) sqlparse.Statement {
	t.Helper()
	stmt, err := Admit(sql)
	if err != nil {
		t.Fatalf("Admit(%q) error = %v", sql, err)
	}
	return stmt
}

func TestAReadOnlyDataSourceAnswersWithEveryRowAndASummary(t *testing.T) {
	table := threeRowTable(t)
	conn := openConn(t, true)

	var b bytes.Buffer
	err := Run(context.Background(), conn, mustAdmit(t, "SELECT n FROM "+table+" ORDER BY n"),
		Limits{MaxRows: 10, Timeout: 5 * time.Second}, &b)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := decodeLines(t, &b)
	if len(got) != 5 {
		t.Fatalf("wrote %d lines, want header, 3 rows and a summary: %s", len(got), b.String())
	}
	if summary := got[4].(map[string]any); summary["rows"] != float64(3) || summary["truncated"] != false {
		t.Errorf("summary = %v", summary)
	}
}

func TestTheCapStopsTheResultAndSaysSo(t *testing.T) {
	table := threeRowTable(t)
	conn := openConn(t, true)

	var b bytes.Buffer
	err := Run(context.Background(), conn, mustAdmit(t, "SELECT n FROM "+table),
		Limits{MaxRows: 2, Timeout: 5 * time.Second}, &b)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := decodeLines(t, &b)
	if summary := got[len(got)-1].(map[string]any); summary["rows"] != float64(2) || summary["truncated"] != true {
		t.Errorf("summary = %v, want 2 rows and truncated", summary)
	}
}

// The admission checks are a guard; this is the reason they can be one. A
// session the server has not confirmed read-only carries nothing at all.
func TestNothingRunsOnADataSourceThatIsNotReadOnly(t *testing.T) {
	conn := openConn(t, false)

	var b bytes.Buffer
	err := Run(context.Background(), conn, mustAdmit(t, "SELECT 1"),
		Limits{MaxRows: 1, Timeout: time.Second}, &b)

	var refused *Refusal
	if !errors.As(err, &refused) {
		t.Fatalf("Run() error = %v, want a *Refusal", err)
	}
	if b.Len() != 0 {
		t.Errorf("wrote %q before refusing", b.String())
	}
}

// A deadline on the client alone leaves the statement running on a shared
// server after the caller has gone. The timeout has to reach the server.
func TestATimeoutEndsTheStatementOnTheServer(t *testing.T) {
	conn := openConn(t, true)
	marker := "/* headless-timeout-" + time.Now().Format("150405.000000") + " */"

	// SLEEP is refused by Admit; the statement is built directly so that the
	// one thing under test is what the timeout does.
	stmt := sqlparse.Parse("SELECT SLEEP(30) " + marker)

	started := time.Now()
	err := Run(context.Background(), conn, stmt, Limits{MaxRows: 1, Timeout: 300 * time.Millisecond}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "KILL QUERY") {
		t.Fatalf("Run() error = %v, want it to say the statement was stopped", err)
	}
	if waited := time.Since(started); waited > 5*time.Second {
		t.Errorf("Run() returned after %s, want it soon after the 300ms timeout", waited)
	}

	observer := openConn(t, false)
	deadline := time.Now().Add(3 * time.Second)
	for {
		var still int
		err := observer.WithControl(context.Background(), func(c *sql.Conn) error {
			return c.QueryRowContext(context.Background(),
				"SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE INFO LIKE ? AND INFO NOT LIKE '%PROCESSLIST%'",
				"%"+marker+"%").Scan(&still)
		})
		if err != nil {
			t.Fatalf("reading the process list: %v", err)
		}
		if still == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the statement was still running on the server after the timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
