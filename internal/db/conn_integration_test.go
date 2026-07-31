//go:build integration

package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func openTestConn(t *testing.T) *Conn {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// collected is everything a consumer learns from a stream.
type collected struct {
	columns   []string
	rows      [][]any
	err       error
	truncated bool
	firstRows time.Duration
}

// drain consumes a stream to completion.
func drain(t *testing.T, s *Stream) collected {
	t.Helper()

	var (
		got   collected
		start = time.Now()
	)
	for ev := range s.Events {
		switch ev.Kind {
		case EventColumns:
			got.columns = ev.Columns
		case EventRows:
			if got.rows == nil {
				got.firstRows = time.Since(start)
			}
			got.rows = append(got.rows, ev.Rows...)
		}
	}

	// The stream has no terminal event: it ends when Events closes, and the
	// reason is read from Err.
	got.err = s.Err()
	got.truncated = s.Truncated()
	return got
}

func TestOpenReportsTheServerVersion(t *testing.T) {
	conn := openTestConn(t)

	if conn.ServerVersion() == "" {
		t.Error("ServerVersion() is empty")
	}
}

func TestQueryStreamsRows(t *testing.T) {
	conn := openTestConn(t)

	stream := conn.Query(context.Background(), "SELECT 1 AS a, 'two' AS b", Options{})
	defer stream.Close()

	got := drain(t, stream)
	if got.err != nil {
		t.Fatalf("stream error = %v, want nil", got.err)
	}
	if len(got.columns) != 2 || got.columns[0] != "a" || got.columns[1] != "b" {
		t.Errorf("columns = %v, want [a b]", got.columns)
	}
	if len(got.rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(got.rows))
	}
	if v := fmt.Sprint(got.rows[0][0]); v != "1" {
		t.Errorf("rows[0][0] = %q, want %q", v, "1")
	}
}

// Query must never block on the server. If it did, the UI would freeze for
// the entire duration of a slow statement — and the user could not even
// press Ctrl+C to cancel it.
func TestQueryReturnsImmediatelyForASlowStatement(t *testing.T) {
	conn := openTestConn(t)

	start := time.Now()
	stream := conn.Query(context.Background(), "SELECT SLEEP(10)", Options{})
	elapsed := time.Since(start)
	defer stream.Close()

	if elapsed > 2*time.Second {
		t.Errorf("Query() took %v before returning; it must not wait on the server", elapsed)
	}

	stream.Cancel()
	drain(t, stream)
}

// A syntax error surfaces through Err rather than a return value, because
// the statement is only sent after Query has returned.
func TestQueryReportsSyntaxErrorsThroughErr(t *testing.T) {
	conn := openTestConn(t)

	stream := conn.Query(context.Background(), "SELECT FROM WHERE", Options{})
	defer stream.Close()

	got := drain(t, stream)
	if got.err == nil {
		t.Fatal("stream error = nil, want a syntax error")
	}
	if !strings.Contains(got.err.Error(), "1064") && !strings.Contains(strings.ToLower(got.err.Error()), "syntax") {
		t.Errorf("error = %v, want the server's syntax error", got.err)
	}
}

// The first batch has to arrive long before the last row does; that is what
// makes a large result feel instant.
func TestQueryDeliversRowsProgressively(t *testing.T) {
	conn := openTestConn(t)
	seedSequence(t, conn)

	stream := conn.Query(context.Background(), selectRows(20000), Options{ChunkSize: 100})
	defer stream.Close()

	got := drain(t, stream)
	if got.err != nil {
		t.Fatalf("stream error = %v, want nil", got.err)
	}
	if len(got.rows) != 20000 {
		t.Fatalf("streamed %d rows, want 20000", len(got.rows))
	}
	t.Logf("first rows after %v", got.firstRows)
}

// The core promise of the tool: a runaway query really stops. Cancelling the
// context alone leaves the server working, so this checks the process list.
func TestCancelStopsTheQueryOnTheServer(t *testing.T) {
	conn := openTestConn(t)

	stream := conn.Query(context.Background(), "SELECT SLEEP(30)", Options{})
	defer stream.Close()

	connID := waitForConnectionID(t, stream)
	if !waitForServerQuery(t, conn, connID, true) {
		t.Fatal("the query never appeared in the process list")
	}

	if err := stream.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	if !waitForServerQuery(t, conn, connID, false) {
		t.Error("the query is still running on the server after Cancel()")
	}

	if got := drain(t, stream); got.err == nil {
		t.Error("stream error = nil after Cancel(), want a cancellation error")
	}
}

// A consumer that ignores events entirely must still learn why it ended.
func TestErrIsReportedToAConsumerThatIgnoresEvents(t *testing.T) {
	conn := openTestConn(t)

	stream := conn.Query(context.Background(), "SELECT SLEEP(30)", Options{})
	defer stream.Close()

	if _, err := stream.WaitConnectionID(context.Background()); err != nil {
		t.Fatalf("WaitConnectionID() error = %v", err)
	}
	if err := stream.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	// Drain without inspecting events at all.
	for range stream.Events {
	}

	if stream.Err() == nil {
		t.Error("Err() = nil after Cancel(), want a cancellation error")
	}
}

func TestCancelBeforeTheStatementStartsIsSafe(t *testing.T) {
	conn := openTestConn(t)

	stream := conn.Query(context.Background(), "SELECT SLEEP(10)", Options{})
	defer stream.Close()

	if err := stream.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	drain(t, stream)
}

// Cancelling twice, or after the statement already finished, must not error
// or kill an unrelated session that reused the connection id.
func TestCancelIsIdempotent(t *testing.T) {
	conn := openTestConn(t)

	stream := conn.Query(context.Background(), "SELECT 1", Options{})
	drain(t, stream)
	stream.Close()

	if err := stream.Cancel(); err != nil {
		t.Errorf("first Cancel() after completion = %v, want nil", err)
	}
	if err := stream.Cancel(); err != nil {
		t.Errorf("second Cancel() = %v, want nil", err)
	}
}

// MaxRows protects the UI from unbounded memory growth; the stream says so
// rather than pretending the result ended naturally.
func TestQueryStopsAtMaxRowsAndReportsTruncation(t *testing.T) {
	conn := openTestConn(t)
	seedSequence(t, conn)

	stream := conn.Query(context.Background(), selectRows(5000),
		Options{ChunkSize: 100, MaxRows: 250})
	defer stream.Close()

	got := drain(t, stream)
	if got.err != nil {
		t.Fatalf("stream error = %v, want nil", got.err)
	}
	if len(got.rows) != 250 {
		t.Errorf("streamed %d rows, want 250", len(got.rows))
	}
	if !got.truncated {
		t.Error("Truncated = false, want true")
	}
}

// A long stream must not block schema browsing, which is the whole reason
// the control connection is kept separate from the query pool.
func TestControlConnectionStaysUsableDuringAStream(t *testing.T) {
	conn := openTestConn(t)
	seedSequence(t, conn)

	stream := conn.Query(context.Background(), selectRows(20000), Options{ChunkSize: 10})
	defer stream.Close()

	// Read one event, then leave the rest unread so the stream is mid-flight.
	<-stream.Events

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	if err := conn.control.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
		t.Fatalf("control query during a stream: %v", err)
	}

	stream.Cancel()
	drain(t, stream)
}

// Concurrent use of the control connection once desynchronised the MySQL
// protocol — "commands out of sync" — after which the driver discarded the
// connection for good, silently disabling both cancellation and every schema
// read for the rest of the session. WithControl serialises access so that
// cannot happen; this holds it to that.
func TestControlConnectionSurvivesConcurrentUse(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	const workers = 8
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			errs <- conn.WithControl(ctx, func(c *sql.Conn) error {
				// A query returning several rows: the failure needs a result
				// set left partly read to corrupt the protocol.
				rows, err := c.QueryContext(ctx,
					"SELECT SCHEMA_NAME FROM information_schema.SCHEMATA")
				if err != nil {
					return err
				}
				defer rows.Close()

				for rows.Next() {
					var name string
					if err := rows.Scan(&name); err != nil {
						return err
					}
				}
				return rows.Err()
			})
		}()
	}

	for i := 0; i < workers; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatalf("concurrent control use failed: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("concurrent control use deadlocked")
		}
	}

	// The connection must still be usable afterwards — the original bug left
	// it permanently closed.
	if err := conn.WithControl(ctx, func(c *sql.Conn) error {
		var n int
		return c.QueryRowContext(ctx, "SELECT 1").Scan(&n)
	}); err != nil {
		t.Errorf("the control connection is unusable after concurrent access: %v", err)
	}
}

// Cancellation shares the control connection with catalog reads, so a schema
// read in flight must not stop a query from being killed.
func TestCancelWorksWhileTheCatalogIsBeingRead(t *testing.T) {
	conn := openTestConn(t)
	ctx := context.Background()

	stream := conn.Query(context.Background(), "SELECT SLEEP(30)", Options{})
	defer stream.Close()

	if _, err := stream.WaitConnectionID(ctx); err != nil {
		t.Fatalf("WaitConnectionID() error = %v", err)
	}

	// Hammer the control connection while cancelling.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				conn.WithControl(ctx, func(c *sql.Conn) error {
					var n int
					return c.QueryRowContext(ctx, "SELECT 1").Scan(&n)
				})
			}
		}
	}()
	defer close(stop)

	if err := stream.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	done := make(chan struct{})
	go func() {
		for range stream.Events {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("the query was not cancelled while the catalog was busy")
	}
}

func TestExecReportsAffectedRows(t *testing.T) {
	conn := openTestConn(t)
	table := createTempTable(t, conn)

	res, err := conn.Exec(context.Background(),
		fmt.Sprintf("INSERT INTO %s (v) VALUES (1), (2), (3)", table))
	if err != nil {
		t.Fatalf("Exec() error = %v, want nil", err)
	}
	if res.RowsAffected != 3 {
		t.Errorf("RowsAffected = %d, want 3", res.RowsAffected)
	}
}

// MariaDB caps recursive CTEs at max_recursive_iterations (1000 by default)
// and only warns, so the fixture uses a real table instead.
func seedSequence(t *testing.T, conn *Conn) {
	t.Helper()
	ctx := context.Background()

	stmts := []string{
		"CREATE TABLE IF NOT EXISTS dv_seq (n INT PRIMARY KEY)",
		"DELETE FROM dv_seq",
		`INSERT INTO dv_seq (n)
		 WITH RECURSIVE s (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM s WHERE n < 1000)
		 SELECT n FROM s`,
	}
	for _, s := range stmts {
		if _, err := conn.Exec(ctx, s); err != nil {
			t.Fatalf("seeding dv_seq with %q: %v", s, err)
		}
	}

	var n int
	if err := conn.control.QueryRowContext(ctx, "SELECT COUNT(*) FROM dv_seq").Scan(&n); err != nil {
		t.Fatalf("counting dv_seq: %v", err)
	}
	if n != 1000 {
		t.Fatalf("dv_seq has %d rows, want 1000", n)
	}
}

// selectRows returns a query yielding n rows by cross-joining the seed table.
func selectRows(n int) string {
	return fmt.Sprintf(
		`SELECT (a.n - 1) * 1000 + b.n AS n, CONCAT('row-', b.n) AS label
		 FROM dv_seq a CROSS JOIN dv_seq b LIMIT %d`, n)
}

func createTempTable(t *testing.T, conn *Conn) string {
	t.Helper()

	name := fmt.Sprintf("dv_test_%d", time.Now().UnixNano())
	ctx := context.Background()

	if _, err := conn.Exec(ctx, fmt.Sprintf(
		"CREATE TABLE %s (id INT AUTO_INCREMENT PRIMARY KEY, v INT)", name)); err != nil {
		t.Fatalf("creating fixture table: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(context.Background(), "DROP TABLE IF EXISTS "+name)
	})
	return name
}

func waitForConnectionID(t *testing.T, s *Stream) uint64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	id, err := s.WaitConnectionID(ctx)
	if err != nil {
		t.Fatalf("WaitConnectionID() error = %v, want nil", err)
	}
	return id
}

// waitForServerQuery polls the process list until the connection is either
// running a statement (want=true) or idle/gone (want=false).
func waitForServerQuery(t *testing.T, conn *Conn, connID uint64, want bool) bool {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		running, err := connectionIsRunning(conn, connID)
		if err != nil {
			t.Fatalf("inspecting the process list: %v", err)
		}
		if running == want {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func connectionIsRunning(conn *Conn, connID uint64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const q = `SELECT COUNT(*) FROM information_schema.PROCESSLIST
	           WHERE ID = ? AND COMMAND <> 'Sleep'`

	var n int
	if err := conn.control.QueryRowContext(ctx, q, connID).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// The guard stops the user, explains what a write will do and makes them
// agree to it. Then it has to say what it actually did: "1 row" and "4,812
// rows" are the difference between a routine edit and an incident.
func TestExecReportsHowManyRowsAStatementChanged(t *testing.T) {
	conn := openTestConn(t)
	seedSequence(t, conn)

	stream := conn.Query(context.Background(), "UPDATE dv_seq SET n = n + 1000 WHERE n <= 3", Options{Exec: true})
	defer stream.Close()

	got := drain(t, stream)
	if got.err != nil {
		t.Fatalf("stream error = %v, want nil", got.err)
	}

	res, ok := stream.Result()
	if !ok {
		t.Fatal("Result() reported nothing; the caller has no way to say what the write did")
	}
	if res.RowsAffected != 3 {
		t.Errorf("RowsAffected = %d, want 3", res.RowsAffected)
	}
}

func TestExecReportsTheIdOfAnInsertedRow(t *testing.T) {
	conn := openTestConn(t)
	mustExec(t, conn, "DROP TABLE IF EXISTS dv_ids")
	mustExec(t, conn, "CREATE TABLE dv_ids (id INT AUTO_INCREMENT PRIMARY KEY, n INT)")
	t.Cleanup(func() { mustExec(t, conn, "DROP TABLE IF EXISTS dv_ids") })

	stream := conn.Query(context.Background(), "INSERT INTO dv_ids (n) VALUES (7)", Options{Exec: true})
	defer stream.Close()

	if got := drain(t, stream); got.err != nil {
		t.Fatalf("stream error = %v, want nil", got.err)
	}

	res, _ := stream.Result()
	if res.LastInsertID == 0 {
		t.Error("LastInsertID = 0, want the id the server assigned")
	}
}

// A statement that streams rows has no count to report, and saying "0 rows
// affected" under a SELECT would be a number that means nothing.
func TestAQueryReportsNoAffectedRowCount(t *testing.T) {
	conn := openTestConn(t)
	seedSequence(t, conn)

	stream := conn.Query(context.Background(), "SELECT n FROM dv_seq", Options{})
	defer stream.Close()
	drain(t, stream)

	if _, ok := stream.Result(); ok {
		t.Error("Result() reported a count for a statement that returned rows")
	}
}

// This is the regression that matters. Sending writes through the pool would
// be the obvious way to get a row count, and it would silently cost server-
// side cancellation: KILL QUERY needs the id of the connection actually
// running the statement, and a pooled Exec never reads one. A runaway UPDATE
// is precisely the statement a user most needs to stop.
func TestAWriteIsStillKilledOnTheServerWhenCancelled(t *testing.T) {
	conn := openTestConn(t)
	seedSequence(t, conn)

	stream := conn.Query(context.Background(),
		"UPDATE dv_seq SET n = n + SLEEP(30) WHERE n = 1", Options{Exec: true})
	defer stream.Close()

	connID := waitForConnectionID(t, stream)
	if !waitForServerQuery(t, conn, connID, true) {
		t.Fatal("the write never appeared in the process list")
	}

	if err := stream.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	if !waitForServerQuery(t, conn, connID, false) {
		t.Error("the write is still running on the server after Cancel()")
	}
}

// mustExec runs a fixture statement through the control path, where a failure
// is a broken test rather than a result worth reporting.
func mustExec(t *testing.T, conn *Conn, sql string) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), sql); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
