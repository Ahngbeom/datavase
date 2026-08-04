// Package procs reads what else is running on the server.
//
// It answers the question a session cannot answer about itself: who else is
// connected, what are they doing, and how long have they been doing it. The
// rows come from information_schema.PROCESSLIST, which both MySQL and MariaDB
// have — only the columns they share are read, because a listing that works on
// one server and not the other is worse than a shorter one.
package procs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
)

// Process is one connection on the server.
type Process struct {
	ID   uint64
	User string
	Host string
	// DB is the schema the connection is using, empty when it has none.
	DB string
	// Command is what the connection is doing at protocol level — "Query",
	// "Sleep", "Binlog Dump". Sleep is a connection holding nothing.
	Command string
	// State is the server's word for the stage a query has reached.
	State string
	// SQL is the statement in flight, empty when there is none.
	SQL string
	// Elapsed is how long the connection has been in this command.
	Elapsed time.Duration
}

// Working reports whether the connection is doing something rather than
// holding a socket open.
//
// It is the difference between a server that is busy and one that merely has
// clients, and it decides nothing on its own — the caller shows both.
func (p Process) Working() bool {
	return !strings.EqualFold(p.Command, "Sleep") && p.Command != ""
}

// Listing is what the server was willing to show.
type Listing struct {
	Processes []Process
	// Complete is false when the connected user lacks the PROCESS privilege,
	// in which case the server silently returns only that user's own
	// connections. A short list and a quiet server look identical, and the
	// difference is the whole reason to have looked.
	Complete bool
}

// List reads the server's connections, longest-running first.
func List(ctx context.Context, conn *db.Conn) (Listing, error) {
	var out Listing

	err := conn.WithControl(ctx, func(c *sql.Conn) error {
		complete, err := hasProcessPrivilege(ctx, c)
		if err != nil {
			return err
		}
		out.Complete = complete

		out.Processes, err = readProcesses(ctx, c)
		return err
	})
	return out, err
}

func readProcesses(ctx context.Context, c *sql.Conn) ([]Process, error) {
	// Only the columns MySQL and MariaDB both have. MariaDB offers more —
	// TIME_MS, PROGRESS, EXAMINED_ROWS — and naming them would make this work
	// on one server and fail on the other.
	const query = `SELECT ID, USER, HOST, DB, COMMAND, TIME, STATE, INFO
		FROM information_schema.PROCESSLIST`

	rows, err := c.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("reading the process list: %w", err)
	}
	defer rows.Close()

	var out []Process
	for rows.Next() {
		var (
			p                   Process
			schema, state, info sql.NullString
			user, host, command sql.NullString
			seconds             sql.NullInt64
		)
		if err := rows.Scan(&p.ID, &user, &host, &schema, &command, &seconds, &state, &info); err != nil {
			return nil, fmt.Errorf("reading the process list: %w", err)
		}

		p.User, p.Host, p.DB = user.String, host.String, schema.String
		p.Command, p.State, p.SQL = command.String, state.String, info.String
		p.Elapsed = time.Duration(seconds.Int64) * time.Second
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading the process list: %w", err)
	}

	Order(out)
	return out, nil
}

// Order puts the connections in the order they are worth reading: what is
// working first, longest first within that, and the idle ones after.
//
// A server in trouble has one statement that has been running for minutes and
// forty connections asleep. Sorting by time alone buries it under whichever
// client last held a socket open the longest.
func Order(ps []Process) {
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].Working() != ps[j].Working() {
			return ps[i].Working()
		}
		return ps[i].Elapsed > ps[j].Elapsed
	})
}

// hasProcessPrivilege asks whether this user can see connections other than
// their own.
//
// The answer has to be asked for rather than inferred. Without the privilege
// the server returns the user's own connections and no error, so a list of one
// means either "nothing else is running" or "you cannot see it", and those are
// opposite answers.
func hasProcessPrivilege(ctx context.Context, c *sql.Conn) (bool, error) {
	rows, err := c.QueryContext(ctx, "SHOW GRANTS")
	if err != nil {
		// A user allowed to connect but not to read their own grants is
		// unusual enough that the listing is better off saying it may be
		// partial than failing outright.
		return false, nil
	}
	defer rows.Close()

	granted := false
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return false, fmt.Errorf("reading grants: %w", err)
		}
		// The line is tested and dropped. SHOW GRANTS carries password
		// hashes, and a grant line kept in a struct is a hash waiting for
		// some error message to print it.
		if grantsProcess(line) {
			granted = true
		}
	}
	return granted, rows.Err()
}

// grantsProcess reports whether one line of SHOW GRANTS confers PROCESS on the
// whole server.
//
// PROCESS is global: granting it on a schema is not a thing, so a line that
// does not say ON *.* cannot be the one however it reads.
func grantsProcess(line string) bool {
	upper := strings.ToUpper(line)
	if !strings.HasPrefix(upper, "GRANT ") || !strings.Contains(upper, " ON *.*") {
		return false
	}

	privileges, _, ok := strings.Cut(upper[len("GRANT "):], " ON *.*")
	if !ok {
		return false
	}
	for _, p := range strings.Split(privileges, ",") {
		switch strings.TrimSpace(p) {
		case "PROCESS", "ALL PRIVILEGES":
			return true
		}
	}
	return false
}

// Stop says how far a kill goes.
type Stop int

const (
	// StopStatement ends the statement in flight and leaves the connection
	// open, which is what "make this stop" almost always means: the client
	// gets an error and its session survives.
	StopStatement Stop = iota
	// StopConnection ends the connection itself, and with it any transaction
	// it was holding — which the server rolls back.
	StopConnection
)

func (s Stop) String() string {
	if s == StopConnection {
		return "connection"
	}
	return "statement"
}

// ErrOwnConnection is returned rather than killing one of this session's own
// connections.
//
// Killing the control connection takes cancellation and every catalog read
// with it, permanently; killing a pooled one loses whatever it was running for
// us. Neither is something to discover by having done it.
var ErrOwnConnection = errors.New("that connection belongs to this session")

// Kill stops a connection's statement, or the connection itself.
//
// It goes over the control connection, which is where KILL has always been
// sent from: a kill that had to wait behind a result streaming into the grid
// would be useless exactly when it was needed.
func Kill(ctx context.Context, conn *db.Conn, id uint64, stop Stop) error {
	if conn.Owns(id) {
		return ErrOwnConnection
	}

	statement := fmt.Sprintf("KILL QUERY %d", id)
	if stop == StopConnection {
		statement = fmt.Sprintf("KILL %d", id)
	}

	return conn.WithControl(ctx, func(c *sql.Conn) error {
		if _, err := c.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("stopping the %s on connection %d: %w", stop, id, err)
		}
		return nil
	})
}

// Thread is a connection seen from a lock's point of view.
type Thread struct {
	ID   uint64
	User string
	Host string
	// SQL is what the connection is running now, which for a blocker is
	// usually not the statement that took the lock: a transaction holds what
	// it took until it ends, whatever it does in between.
	SQL string
	// Idle says the connection holds its locks and is running nothing at all.
	// It is the most common answer to "why is everything stuck", and the one
	// a list of statements cannot give.
	Idle bool
}

// Wait is one connection waiting on another.
type Wait struct {
	Waiter  Thread
	Blocker Thread
	Waited  time.Duration
}

// LockTree is who is waiting on whom, and whether the server would say.
type LockTree struct {
	// Roots are the connections blocking others without waiting themselves.
	Roots []*Blocked
	// Supported is false when this server keeps its lock waits somewhere this
	// does not know to look. An empty tree and a server that will not say look
	// identical, and only one of them means nothing is stuck.
	Supported bool
}

// Blocked is one connection in the tree, with whatever is waiting on it.
type Blocked struct {
	Thread Thread
	// Waited is how long this connection has been waiting for its own lock,
	// and is zero at a root.
	Waited  time.Duration
	Waiters []*Blocked
}

// lockWaitQueries are the shapes a server keeps its InnoDB lock waits in.
//
// MariaDB has information_schema.INNODB_LOCK_WAITS; MySQL 8 moved to
// performance_schema.data_lock_waits. Both join to information_schema.INNODB_TRX
// for the transaction behind each side, so only the wait table differs.
var lockWaitQueries = []struct {
	table string
	query string
}{
	{
		table: "information_schema.INNODB_LOCK_WAITS",
		query: lockWaitSQL("information_schema.INNODB_LOCK_WAITS", "requesting_trx_id", "blocking_trx_id"),
	},
	{
		table: "performance_schema.data_lock_waits",
		query: lockWaitSQL("performance_schema.data_lock_waits",
			"REQUESTING_ENGINE_TRANSACTION_ID", "BLOCKING_ENGINE_TRANSACTION_ID"),
	},
}

func lockWaitSQL(table, requesting, blocking string) string {
	return fmt.Sprintf(`SELECT
		r.trx_mysql_thread_id, rp.USER, rp.HOST, COALESCE(r.trx_query, ''),
		b.trx_mysql_thread_id, bp.USER, bp.HOST, COALESCE(b.trx_query, ''),
		TIMESTAMPDIFF(SECOND, r.trx_wait_started, NOW())
	FROM %s w
	JOIN information_schema.INNODB_TRX r ON r.trx_id = w.%s
	JOIN information_schema.INNODB_TRX b ON b.trx_id = w.%s
	LEFT JOIN information_schema.PROCESSLIST rp ON rp.ID = r.trx_mysql_thread_id
	LEFT JOIN information_schema.PROCESSLIST bp ON bp.ID = b.trx_mysql_thread_id`,
		table, requesting, blocking)
}

// LockWaits reads who is waiting on whom.
func LockWaits(ctx context.Context, conn *db.Conn) (LockTree, error) {
	var out LockTree

	err := conn.WithControl(ctx, func(c *sql.Conn) error {
		for _, shape := range lockWaitQueries {
			exists, err := tableExists(ctx, c, shape.table)
			if err != nil {
				return err
			}
			if !exists {
				continue
			}

			waits, err := readWaits(ctx, c, shape.query)
			if err != nil {
				return err
			}
			out = LockTree{Roots: Tree(waits), Supported: true}
			return nil
		}
		return nil
	})
	return out, err
}

func tableExists(ctx context.Context, c *sql.Conn, qualified string) (bool, error) {
	schema, name, _ := strings.Cut(qualified, ".")

	var n int
	err := c.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?",
		schema, name).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("looking for %s: %w", qualified, err)
	}
	return n > 0, nil
}

func readWaits(ctx context.Context, c *sql.Conn, query string) ([]Wait, error) {
	rows, err := c.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("reading lock waits: %w", err)
	}
	defer rows.Close()

	var out []Wait
	for rows.Next() {
		var (
			w                          Wait
			rUser, rHost, bUser, bHost sql.NullString
			waiterSQL, blockerSQL      string
			seconds                    sql.NullInt64
		)
		if err := rows.Scan(
			&w.Waiter.ID, &rUser, &rHost, &waiterSQL,
			&w.Blocker.ID, &bUser, &bHost, &blockerSQL,
			&seconds,
		); err != nil {
			return nil, fmt.Errorf("reading lock waits: %w", err)
		}

		w.Waiter.User, w.Waiter.Host, w.Waiter.SQL = rUser.String, rHost.String, waiterSQL
		w.Blocker.User, w.Blocker.Host, w.Blocker.SQL = bUser.String, bHost.String, blockerSQL
		// A blocker running nothing is a transaction left open, which is the
		// most common reason a table is stuck and the one a list of running
		// statements cannot show.
		w.Blocker.Idle = blockerSQL == ""
		w.Waiter.Idle = waiterSQL == ""
		w.Waited = time.Duration(seconds.Int64) * time.Second

		out = append(out, w)
	}
	return out, rows.Err()
}

// Tree turns the pairs into who-waits-on-whom.
//
// A flat list of pairs is what the server gives and what a DBA then has to
// correlate by hand — and the thing worth finding is the connection at the
// bottom that waits on nothing, because that is the one to deal with.
func Tree(waits []Wait) []*Blocked {
	nodes := make(map[uint64]*Blocked, len(waits)*2)
	waiting := make(map[uint64]bool, len(waits))

	node := func(t Thread) *Blocked {
		if existing, ok := nodes[t.ID]; ok {
			// A thread seen first as a blocker has no statement recorded
			// against it yet; the fuller of the two descriptions wins.
			if existing.Thread.SQL == "" {
				existing.Thread = t
			}
			return existing
		}
		created := &Blocked{Thread: t}
		nodes[t.ID] = created
		return created
	}

	for _, w := range waits {
		waiter, blocker := node(w.Waiter), node(w.Blocker)
		waiter.Waited = w.Waited
		waiting[w.Waiter.ID] = true

		// The same pair can appear once per lock held; one edge is enough.
		if !alreadyWaiting(blocker, waiter.Thread.ID) {
			blocker.Waiters = append(blocker.Waiters, waiter)
		}
	}

	var roots []*Blocked
	for id, n := range nodes {
		if !waiting[id] {
			roots = append(roots, n)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Thread.ID < roots[j].Thread.ID })
	return roots
}

func alreadyWaiting(b *Blocked, id uint64) bool {
	for _, w := range b.Waiters {
		if w.Thread.ID == id {
			return true
		}
	}
	return false
}
