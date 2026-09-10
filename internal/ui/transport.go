package ui

import (
	"errors"
	"fmt"

	"github.com/Ahngbeom/datavase/internal/db"
)

// failureCause decides what a failed statement was actually about.
//
// Every production datasource in this configuration reaches its server through
// a bastion, and when one stops forwarding the driver reports a socket that has
// gone — "invalid connection". That reads as the database being in trouble,
// which is the difference between reconnecting and paging whoever owns it.
//
// The tunnel is only blamed for a failure that could have happened anywhere
// between here and the server. A numbered MySQL error is proof the statement
// arrived and was answered, so whatever the tunnel once recorded, it is not
// what stopped this one — and Tunnel.Err returns the first failure it ever saw
// and never clears it, so without that rule a single transient forward failure
// would make every later typo read as a dead bastion for the rest of the
// session.
func failureCause(stmtErr, tunnelErr error, bastion string) error {
	if stmtErr == nil {
		return nil
	}
	if tunnelErr == nil || db.ReachedServer(stmtErr) {
		return stmtErr
	}

	// The bastion's own reason is kept: naming the hop says where to look, and
	// the reason underneath says what to do about it.
	return fmt.Errorf("the bastion %s stopped forwarding — this connection is gone: %w",
		bastion, tunnelErr)
}

// transportFailure is what the session's tunnel has recorded, if anything.
func (a *App) transportFailure() error {
	if a.sess == nil {
		return nil
	}
	return a.sess.TunnelErr()
}

// bastionName is the host a datasource is reached through, for a message that
// has to name it.
func (a *App) bastionName() string {
	if tun := a.conn.DataSource().Tunnel; tun != nil {
		return tun.Host
	}
	return "the bastion"
}

// readOnlyRefusal replaces the server's account of a refused write with the
// interface's own, when the interface is what asked for read-only.
//
// The server says "cannot execute statement in a READ ONLY transaction":
// true, seventy cells wide, and about a transaction nobody opened. On a
// datasource that is not read_only the words are the right ones — the user
// set that transaction up themselves — and they are left alone.
func readOnlyRefusal(err error, readOnly bool) error {
	if !readOnly || !db.IsReadOnlyRefusal(err) {
		return err
	}
	return errors.New("write refused: read_only is set on this datasource")
}
