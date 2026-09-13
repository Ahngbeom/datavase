package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// sessionReadOnly reports what the server says about this connection.
//
// The question is asked of the server rather than answered from the
// configuration, because those are two different facts: the configuration
// says what was asked for, and only the server knows what the session it is
// about to run a statement on will actually refuse. An interface that draws
// its marker from the first can say "read-only" over a session that is not.
//
// transaction_read_only is the one spelling both servers answer to: MySQL
// since 5.7.20 and MariaDB since 10.3, the latter keeping tx_read_only as an
// alias. The older spelling is not tried as a fallback, because a server too
// old to know this one is a server this code has never been tested against.
func sessionReadOnly(ctx context.Context, conn *sql.Conn) (bool, error) {
	var readOnly bool
	if err := conn.QueryRowContext(ctx, "SELECT @@session.transaction_read_only").Scan(&readOnly); err != nil {
		return false, fmt.Errorf("asking the server whether the session is read-only: %w", err)
	}
	return readOnly, nil
}

// errReadOnlyUnconfirmed is what every caller of enforceReadOnly turns into
// a refusal: no session opens and no statement runs while the server has not
// agreed that writes are off.
var errReadOnlyUnconfirmed = errors.New(
	"this datasource is read_only but the server did not confirm the session is read-only")

// enforceReadOnly makes conn read-only and confirms it with the server.
//
// Fail closed: a datasource marked read_only whose session cannot be
// confirmed refuses to carry statements at all. The alternative — carrying
// on and letting the marker claim a protection that is not there — is worse
// than no protection, because it is believed.
//
// Two round trips, paid only by read-only datasources: one to ask, one to
// check. The check is the point. Asking without checking is what the
// configuration already did.
func enforceReadOnly(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
		return fmt.Errorf("making the connection read-only: %w", err)
	}
	confirmed, err := sessionReadOnly(ctx, conn)
	if err != nil {
		return err
	}
	if !confirmed {
		return errReadOnlyUnconfirmed
	}
	return nil
}
