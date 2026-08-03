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
