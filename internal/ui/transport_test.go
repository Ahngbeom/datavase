package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestAFailedStatementNamesTheHopThatActuallyFailed(t *testing.T) {
	var (
		socket  = errors.New("invalid connection")
		refused = &mysql.MySQLError{Number: 1146, Message: "Table 'app.orders' doesn't exist"}
		dropped = errors.New("forwarding to db.internal:3306: channel open failed")
	)

	for _, tt := range []struct {
		name    string
		stmtErr error
		tunnel  error
		want    string
	}{
		{
			// The whole point. The driver says the same thing whichever hop
			// went, and "invalid connection" reads as the database being in
			// trouble when it was the bastion.
			name:    "a dead socket with a broken tunnel is the bastion's",
			stmtErr: socket, tunnel: dropped,
			want: "bastion.example.com",
		},
		{
			// The subtle one. Tunnel.Err returns the first failure it ever
			// recorded and never clears it, so one transient forward failure
			// early on would otherwise make every later typo read as a dead
			// bastion. A numbered server error is proof the statement got
			// there, so the tunnel cannot be what stopped it.
			name:    "a server refusal is never blamed on the bastion",
			stmtErr: refused, tunnel: dropped,
			want: "doesn't exist",
		},
		{
			name:    "a dead socket with a healthy tunnel is left alone",
			stmtErr: socket, tunnel: nil,
			want: "invalid connection",
		},
		{
			name:    "a server refusal without a tunnel is left alone",
			stmtErr: refused, tunnel: nil,
			want: "doesn't exist",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := failureCause(tt.stmtErr, tt.tunnel, "bastion.example.com")
			if got == nil {
				t.Fatal("failureCause returned nil for a failure")
			}
			if !strings.Contains(got.Error(), tt.want) {
				t.Errorf("failureCause said %q, wanted it to mention %q", got, tt.want)
			}
		})
	}
}

// A statement that did not fail has no cause to report.
func TestASuccessfulStatementHasNoCause(t *testing.T) {
	if got := failureCause(nil, errors.New("the bastion went"), "b"); got != nil {
		t.Errorf("failureCause(nil, ...) = %v, want nil", got)
	}
}

// The bastion's own error is kept rather than replaced. "The bastion stopped
// forwarding" says which hop; the reason underneath says what to do about it.
func TestTheBastionsOwnReasonSurvives(t *testing.T) {
	cause := failureCause(
		errors.New("invalid connection"),
		fmt.Errorf("channel open failed: administratively prohibited"),
		"bastion.example.com")

	for _, want := range []string{"bastion.example.com", "administratively prohibited"} {
		if !strings.Contains(cause.Error(), want) {
			t.Errorf("the reported cause %q does not mention %q", cause, want)
		}
	}
}
