package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Ahngbeom/datavase/internal/config"
)

// MaxQueryConns bounds the query pool. A handful is plenty: the user runs
// one statement at a time, and the spare slots exist so a stalled stream
// cannot starve the next one.
const MaxQueryConns = 4

// Conn is a live connection to one datasource.
//
// It deliberately holds two separate connections to the same server:
//
//   - pool serves the statements the user runs
//   - control is reserved for KILL QUERY and catalog reads
//
// MySQL will not accept another statement on a connection until the current
// result set has been read to the end, so without this split a long stream
// would block both cancellation and schema browsing — the two things most
// needed while a long stream is in flight.
type Conn struct {
	ds   *config.DataSource
	pool *sql.DB

	// controlMu serialises use of the control connection.
	//
	// A single MySQL connection handles one command at a time. Two goroutines
	// using it at once desynchronise the protocol — "commands out of sync" —
	// and the driver then discards the connection permanently, taking
	// cancellation and every catalog read down with it. Handing out the
	// *sql.Conn made that failure possible, so the connection is reached only
	// through WithControl.
	controlMu sync.Mutex
	control   *sql.Conn

	// txMu guards the transaction below. It is never held across a round
	// trip, so opening or ending a transaction cannot block on a statement.
	txMu sync.Mutex
	// tx is the connection pinned for the duration of a transaction, nil when
	// none is open. txBusy says a statement is using it: one connection
	// handles one command at a time, and two at once desynchronise the
	// protocol — the same failure controlMu exists to prevent, with the
	// transaction lost along with the connection.
	tx     *sql.Conn
	txID   uint64
	txBusy bool

	// schemaEverSwitched records that some query has issued USE.
	//
	// Until that happens no pooled connection can be pointing anywhere but
	// the datasource's own schema, so the reset below can be skipped and the
	// common case costs no extra round trip. Over an SSH tunnel that round
	// trip is tens of milliseconds on every statement.
	schemaEverSwitched atomic.Bool

	version string
}

// Open connects to ds. addr overrides the dialled address, which is how an
// SSH tunnel is wired in.
func Open(ctx context.Context, ds *config.DataSource, password, addr string) (*Conn, error) {
	dsn, err := DSN(ds, password, addr)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", ds.Name, err)
	}

	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", ds.Name, err)
	}
	pool.SetMaxOpenConns(MaxQueryConns)
	pool.SetMaxIdleConns(MaxQueryConns)

	control, err := pool.Conn(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("opening the control connection to %s: %w", ds.Name, err)
	}

	var version string
	if err := control.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		control.Close()
		pool.Close()
		return nil, fmt.Errorf("connecting to %s: %w", ds.Name, err)
	}

	return &Conn{ds: ds, pool: pool, control: control, version: version}, nil
}

// DataSource returns the datasource this connection serves.
func (c *Conn) DataSource() *config.DataSource { return c.ds }

// ServerVersion returns the version string reported at connection time.
func (c *Conn) ServerVersion() string { return c.version }

// WithControl runs fn with exclusive use of the control connection.
//
// This connection is reserved for cancellation and catalog reads, which is
// what keeps schema browsing responsive while the query pool streams a large
// result. Access goes through a callback rather than an accessor so that
// concurrent use — which would corrupt the protocol and kill the connection
// for good — cannot be expressed.
//
// fn must not retain the connection beyond its return.
func (c *Conn) WithControl(ctx context.Context, fn func(*sql.Conn) error) error {
	c.controlMu.Lock()
	defer c.controlMu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(c.control)
}

// Close releases both connections.
func (c *Conn) Close() error {
	// An open transaction would otherwise be left holding locks on the server
	// until its connection was reaped. Rolling back is the safe reading of a
	// session that ended without committing.
	if c.InTransaction() {
		c.Rollback(context.Background())
	}
	if c.control != nil {
		c.control.Close()
	}
	return c.pool.Close()
}

// Options tunes how a result set is streamed.
type Options struct {
	// ChunkSize is how many rows are gathered before a batch is published.
	ChunkSize int
	// MaxRows caps how many rows are read. Zero means no cap.
	MaxRows int
	// Schema is the one an unqualified name resolves against. Empty means the
	// schema the connection was opened with.
	Schema string

	// Exec sends the statement as one that returns a count rather than rows,
	// which is the only way the server's affected-row count can be read.
	//
	// It is the caller's decision because only the caller has the parsed
	// statement; asking the driver afterwards is not possible, since a write
	// sent as a query simply yields a result set with no columns and the
	// count is gone.
	Exec bool
}

func (o Options) chunkSize() int {
	if o.ChunkSize <= 0 {
		return config.DefaultFetchChunk
	}
	return o.ChunkSize
}

// ExecResult summarises a statement that returns no rows.
type ExecResult struct {
	RowsAffected int64
	LastInsertID int64
}

// Exec runs a statement that produces no result set.
func (c *Conn) Exec(ctx context.Context, sql string) (ExecResult, error) {
	res, err := c.pool.ExecContext(ctx, sql)
	if err != nil {
		return ExecResult{}, err
	}

	// Both are advisory: not every statement or driver reports them.
	affected, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()
	return ExecResult{RowsAffected: affected, LastInsertID: lastID}, nil
}

// InTransaction reports whether a transaction is open on this connection.
func (c *Conn) InTransaction() bool {
	c.txMu.Lock()
	defer c.txMu.Unlock()
	return c.tx != nil
}

// Begin opens a transaction and pins the connection it runs on.
//
// The pin is the whole point. Statements ordinarily take a connection from
// the pool and hand it straight back, so a transaction opened on one would be
// abandoned by the next statement — BEGIN would have nothing to commit and
// ROLLBACK nothing to undo, while both reported success.
//
// The connection id is read once, here, rather than per statement: it is the
// same connection every time, and KILL QUERY still has something to aim at.
func (c *Conn) Begin(ctx context.Context) error {
	c.txMu.Lock()
	if c.tx != nil {
		c.txMu.Unlock()
		return errors.New("a transaction is already open")
	}
	c.txMu.Unlock()

	conn, err := c.pool.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring a connection for the transaction: %w", err)
	}

	var id uint64
	if err := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&id); err != nil {
		conn.Close()
		return fmt.Errorf("reading the connection id: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "START TRANSACTION"); err != nil {
		conn.Close()
		return err
	}

	c.txMu.Lock()
	defer c.txMu.Unlock()
	// Another Begin could have won the race between the check above and here.
	if c.tx != nil {
		conn.ExecContext(ctx, "ROLLBACK")
		conn.Close()
		return errors.New("a transaction is already open")
	}
	c.tx, c.txID = conn, id
	return nil
}

// Commit ends the transaction, keeping its work.
func (c *Conn) Commit(ctx context.Context) error { return c.endTransaction(ctx, "COMMIT") }

// Rollback ends the transaction, discarding its work.
func (c *Conn) Rollback(ctx context.Context) error { return c.endTransaction(ctx, "ROLLBACK") }

// endTransaction runs the closing statement and releases the pinned
// connection, whether or not that statement succeeded.
//
// Keeping a connection whose COMMIT failed would leave the session pinned to
// something in an unknown state; returning it to the pool lets the driver
// reset it.
func (c *Conn) endTransaction(ctx context.Context, statement string) error {
	c.txMu.Lock()
	if c.tx == nil {
		c.txMu.Unlock()
		return errors.New("no transaction is open")
	}
	if c.txBusy {
		c.txMu.Unlock()
		return errors.New("a statement is still running in this transaction")
	}
	conn := c.tx
	c.tx, c.txID = nil, 0
	c.txMu.Unlock()

	_, err := conn.ExecContext(ctx, statement)
	conn.Close()
	return err
}

// acquire returns the connection this statement runs on, its server-side id,
// and the release to call when the statement has finished with it.
//
// It is the only place that knows where a connection comes from. Inside a
// transaction both resolve to the pinned connection and the release only
// marks it free again — the transaction owns its lifetime, not the statement.
// Everything downstream takes the connection as an argument and never asks.
func (c *Conn) acquire(ctx context.Context) (*sql.Conn, uint64, func(), error) {
	c.txMu.Lock()
	if c.tx == nil {
		c.txMu.Unlock()

		conn, err := c.pool.Conn(ctx)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("acquiring a connection: %w", err)
		}

		// Read on the very connection that will run the statement, since that
		// is what KILL QUERY targets.
		var id uint64
		if err := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&id); err != nil {
			conn.Close()
			return nil, 0, nil, fmt.Errorf("reading the connection id: %w", err)
		}
		return conn, id, func() { conn.Close() }, nil
	}

	if c.txBusy {
		c.txMu.Unlock()
		return nil, 0, nil, errors.New("a statement is already running in this transaction")
	}
	c.txBusy = true
	conn, id := c.tx, c.txID
	c.txMu.Unlock()

	return conn, id, func() {
		c.txMu.Lock()
		c.txBusy = false
		c.txMu.Unlock()
	}, nil
}
