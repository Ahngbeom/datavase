package db

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// A statement that failed because the server said no is proof the server was
// reached. Anything else — a socket that has gone — could have failed at any
// hop, and only that case may be attributed to something underneath.
func TestReachedServerDistinguishesAServerRefusalFromALostConnection(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{"a server error names a number", &mysql.MySQLError{Number: 1064, Message: "syntax"}, true},
		{"wrapped, still the server", fmt.Errorf("running: %w", &mysql.MySQLError{Number: 1146}), true},
		{"a dead socket is not", errors.New("invalid connection"), false},
		{"a driver bad connection is not", driver.ErrBadConn, false},
		{"nothing failed", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReachedServer(tt.err); got != tt.want {
				t.Errorf("ReachedServer(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// A refused write on a read-only datasource has to be recognisable by
// number: the interface replaces the server's sentence with its own, and
// matching on the words would break on a server that phrases it differently.
func TestIsReadOnlyRefusalKnowsTheServersNumber(t *testing.T) {
	if !IsReadOnlyRefusal(fmt.Errorf("running: %w", &mysql.MySQLError{Number: 1792})) {
		t.Error("error 1792 was not recognised as a read-only refusal")
	}
	if IsReadOnlyRefusal(&mysql.MySQLError{Number: 1064}) {
		t.Error("a syntax error was taken for a read-only refusal")
	}
	if IsReadOnlyRefusal(nil) {
		t.Error("nil was taken for a read-only refusal")
	}
}
