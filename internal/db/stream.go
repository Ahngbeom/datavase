package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"github.com/go-sql-driver/mysql"
)

// EventKind identifies what a stream event carries.
type EventKind int

const (
	// EventColumns arrives once, when the server returns the result header.
	EventColumns EventKind = iota
	// EventRows carries a batch of rows.
	EventRows
)

// Event is one update from a running statement.
//
// There is deliberately no terminal event: the stream ends when Events is
// closed, and the reason is read from Err. A final event could be dropped
// when a cancelled stream closes, leaving the caller unable to tell a
// cancellation from a clean finish.
type Event struct {
	Kind EventKind

	// Columns and Types are set on EventColumns.
	Columns []string
	Types   []*sql.ColumnType

	// Rows is set on EventRows.
	Rows [][]any
}

// Stream is a statement in flight.
//
// Everything after Query returns is delivered on Events, which is closed
// when the statement ends for any reason. This shape exists because the
// driver's QueryContext blocks until the server produces a result header:
// a slow statement would otherwise freeze the caller — and in the TUI, the
// caller is the event loop that has to stay responsive enough to cancel it.
type Stream struct {
	// Events yields progress until it is closed.
	Events <-chan Event

	conn     *Conn
	cancelFn context.CancelFunc
	runCtx   context.Context

	// idReady is closed once connID holds the server-side connection id.
	idReady chan struct{}
	connID  uint64

	mu        sync.Mutex
	finished  bool
	err       error
	truncated bool
	closeOnce sync.Once
}

// Err reports why the stream ended, once Events is closed. It is nil when
// the result set was read to the end.
//
// This mirrors sql.Rows and bufio.Scanner: iterate until the source is
// exhausted, then ask why it stopped.
func (s *Stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Truncated reports whether the stream stopped at Options.MaxRows rather
// than at the end of the result set.
func (s *Stream) Truncated() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.truncated
}

// finish records the outcome before the events channel closes.
func (s *Stream) finish(err error, truncated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.finished = true
	s.truncated = truncated
	if s.err == nil {
		s.err = err
	}
}

// ConnectionID returns the server-side connection id, or zero if the
// statement has not reached the server yet.
func (s *Stream) ConnectionID() uint64 {
	select {
	case <-s.idReady:
		return s.connID
	default:
		return 0
	}
}

// WaitConnectionID blocks until the server-side connection id is known.
func (s *Stream) WaitConnectionID(ctx context.Context) (uint64, error) {
	select {
	case <-s.idReady:
		return s.connID, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// Query runs sql and streams the result.
//
// It returns immediately; connecting, sending the statement and reading rows
// all happen in the background. Failures — including a syntax error — surface
// through Err once Events is closed.
func (c *Conn) Query(ctx context.Context, sql string, opt Options) *Stream {
	runCtx, cancel := context.WithCancel(ctx)
	events := make(chan Event, 1)

	s := &Stream{
		Events:   events,
		conn:     c,
		runCtx:   runCtx,
		cancelFn: cancel,
		idReady:  make(chan struct{}),
	}

	go s.run(events, sql, opt)
	return s
}

func (s *Stream) run(out chan<- Event, query string, opt Options) {
	defer close(out)
	defer s.markFinished()

	conn, err := s.conn.pool.Conn(s.runCtx)
	if err != nil {
		s.done(fmt.Errorf("acquiring a connection: %w", err), false)
		return
	}
	defer conn.Close()

	// The id has to be read on the very connection that will run the
	// statement, since that is what KILL QUERY targets.
	var connID uint64
	if err := conn.QueryRowContext(s.runCtx, "SELECT CONNECTION_ID()").Scan(&connID); err != nil {
		s.done(fmt.Errorf("reading the connection id: %w", err), false)
		return
	}
	s.connID = connID
	close(s.idReady)

	if schema := s.schemaFor(opt); schema != "" {
		if _, err := conn.ExecContext(s.runCtx, "USE "+sqlparse.QuoteIdentifier(schema)); err != nil {
			s.done(fmt.Errorf("switching to schema %q: %w", schema, err), false)
			return
		}
	}

	rows, err := conn.QueryContext(s.runCtx, query)
	if err != nil {
		s.done(err, false)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		s.done(err, false)
		return
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		s.done(err, false)
		return
	}
	if !s.emit(out, Event{Kind: EventColumns, Columns: columns, Types: types}) {
		s.done(nil, false)
		return
	}

	truncated, err := s.pump(out, rows, len(columns), opt)
	s.done(err, truncated)
}

// schemaFor decides which schema this statement runs against, and returns the
// empty string when nothing needs to be sent.
//
// USE is connection-local and pooled connections are handed round, so a query
// that switched schema leaves the connection pointing elsewhere for whoever
// gets it next — which is how an unqualified statement ends up reading the
// wrong data. Once any query has switched, every later one states its schema
// explicitly rather than trusting what it finds.
func (s *Stream) schemaFor(opt Options) string {
	if opt.Schema != "" {
		s.conn.schemaEverSwitched.Store(true)
		return opt.Schema
	}
	if !s.conn.schemaEverSwitched.Load() {
		return ""
	}
	return s.conn.ds.Database
}

// done records the outcome. Events closes right after, and Err reports why.
func (s *Stream) done(err error, truncated bool) {
	// A cancelled statement often ends without a driver error; report the
	// cancellation rather than a silent, successful-looking finish.
	if err == nil {
		if ctxErr := s.runCtx.Err(); ctxErr != nil {
			err = ctxErr
		}
	}
	s.finish(err, truncated)
}

// pump reads rows into batches until the result ends or the cap is reached.
func (s *Stream) pump(out chan<- Event, rows *sql.Rows, ncols int, opt Options) (bool, error) {
	chunk := opt.chunkSize()
	batch := make([][]any, 0, chunk)
	read := 0

	// Scan targets are reused across rows; values are copied out per row.
	cells := make([]any, ncols)
	targets := make([]any, ncols)
	for i := range cells {
		targets[i] = &cells[i]
	}

	flush := func() bool {
		if len(batch) == 0 {
			return true
		}
		if !s.emit(out, Event{Kind: EventRows, Rows: batch}) {
			return false
		}
		batch = make([][]any, 0, chunk)
		return true
	}

	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return false, err
		}

		batch = append(batch, copyRow(cells))
		read++

		if opt.MaxRows > 0 && read >= opt.MaxRows {
			flush()
			return true, nil
		}
		if len(batch) >= chunk && !flush() {
			return false, s.runCtx.Err()
		}
	}

	if err := rows.Err(); err != nil {
		return false, err
	}
	flush()
	return false, nil
}

// emit publishes an event, giving up if the statement was cancelled so a
// consumer that walked away cannot leak this goroutine.
func (s *Stream) emit(out chan<- Event, ev Event) bool {
	select {
	case out <- ev:
		return true
	case <-s.runCtx.Done():
		return false
	}
}

// copyRow detaches scanned values from the driver's reusable buffers;
// []byte in particular is only valid until the next Scan.
func copyRow(cells []any) []any {
	row := make([]any, len(cells))
	for i, c := range cells {
		if b, ok := c.([]byte); ok {
			cp := make([]byte, len(b))
			copy(cp, b)
			row[i] = cp
			continue
		}
		row[i] = c
	}
	return row
}

// Cancel stops the statement on the server.
//
// Cancelling the context alone only detaches the client: the server keeps
// executing until it finishes. KILL QUERY, sent over the separate control
// connection, is what actually stops the work.
func (s *Stream) Cancel() error {
	s.mu.Lock()
	finished := s.finished
	s.mu.Unlock()

	// Once the statement has finished the id may already have been reused by
	// another session, and killing it would interrupt unrelated work.
	if finished {
		return nil
	}

	// Detach the client first so a statement that has not yet reached the
	// server is stopped too.
	s.cancelFn()

	connID := s.ConnectionID()
	if connID == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), DialTimeout)
	defer cancel()

	err := s.conn.WithControl(ctx, func(c *sql.Conn) error {
		_, err := c.ExecContext(ctx, fmt.Sprintf("KILL QUERY %d", connID))
		return err
	})
	if err != nil && !isUnknownThreadError(err) {
		return fmt.Errorf("cancelling the query: %w", err)
	}
	return nil
}

// Close releases the stream's resources. It is safe to call repeatedly.
func (s *Stream) Close() error {
	s.closeOnce.Do(func() {
		s.cancelFn()
	})
	return nil
}

func (s *Stream) markFinished() {
	s.mu.Lock()
	s.finished = true
	s.mu.Unlock()
}

// MySQL error numbers this package reasons about.
const (
	// erNoSuchThread is returned by KILL QUERY when the statement finished
	// between the decision to cancel and the kill reaching the server.
	erNoSuchThread = 1094
	// erQueryInterrupted is what a killed statement reports to its own
	// connection.
	erQueryInterrupted = 1317
)

func isUnknownThreadError(err error) bool {
	return isMySQLError(err, erNoSuchThread)
}

// IsInterrupted reports whether err is the server saying the statement was
// killed. Callers use it to present a cancellation as an outcome the user
// chose rather than as a failure.
func IsInterrupted(err error) bool {
	return isMySQLError(err, erQueryInterrupted)
}

func isMySQLError(err error, number uint16) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == number
	}
	return false
}
