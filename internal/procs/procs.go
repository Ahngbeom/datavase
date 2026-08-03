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
