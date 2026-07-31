package db

import (
	"context"
	"database/sql"
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
	pool, err := sql.Open("mysql", DSN(ds, password, addr))
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
