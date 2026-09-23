package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// Every hop between a terminal and a database fails differently, and the
// driver reports all of them as a connection that did not happen. Which one
// it was decides who can fix it: a name, a port, a VPN, a certificate, a
// password, or a database that is not there.
func TestDiagnoseTellsTheHopsApart(t *testing.T) {
	timeout := &net.OpError{Op: "dial", Err: &timeoutError{}}

	for _, tt := range []struct {
		name string
		err  error
		want Fault
	}{
		{"a name that does not resolve",
			fmt.Errorf("dial: %w", &net.DNSError{Err: "no such host", Name: "db.internal", IsNotFound: true}),
			FaultUnresolved},
		{"a name lookup that gave up waiting is still a name",
			fmt.Errorf("dial: %w", &net.DNSError{Err: "i/o timeout", Name: "db.internal", IsTimeout: true}),
			FaultUnresolved},
		{"nothing listening on the port",
			fmt.Errorf("dial: %w", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}),
			FaultRefused},
		{"no answer before the deadline",
			fmt.Errorf("dial: %w", timeout),
			FaultUnreachable},
		{"the context ran out first",
			fmt.Errorf("connecting: %w", context.DeadlineExceeded),
			FaultUnreachable},
		{"a certificate that was not accepted",
			fmt.Errorf("tls: %w", &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}),
			FaultTLS},
		{"a server with no TLS to offer",
			fmt.Errorf("connecting: %w", errNoTLS),
			FaultTLS},
		{"credentials the server refused",
			fmt.Errorf("opening: %w", mysqlErrorNumber(1045)),
			FaultCredentials},
		{"a password the server refused before it was asked for one",
			fmt.Errorf("opening: %w", mysqlErrorNumber(1698)),
			FaultCredentials},
		{"a user with no rights to that database",
			fmt.Errorf("opening: %w", mysqlErrorNumber(1044)),
			FaultDatabaseDenied},
		{"a host the server does not accept at all",
			fmt.Errorf("opening: %w", mysqlErrorNumber(1130)),
			FaultHostNotAllowed},
		{"a database that is not there",
			fmt.Errorf("opening: %w", mysqlErrorNumber(1049)),
			FaultNoSuchDatabase},
		{"a refusal this function has nothing to add to",
			fmt.Errorf("opening: %w", mysqlErrorNumber(1064)),
			FaultUnknown},
		{"nothing failed", nil, FaultNone},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Diagnose(tt.err); got != tt.want {
				t.Errorf("Diagnose() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A classification nobody can read is a classification that changes nothing.
// Each fault this names has to say what to go and look at, and two of them
// saying the same sentence would mean one of the two was never worth telling
// apart.
func TestEveryNamedFaultSaysSomethingDifferentToCheck(t *testing.T) {
	seen := map[string]Fault{}
	for _, f := range []Fault{FaultUnresolved, FaultRefused, FaultUnreachable,
		FaultTLS, FaultCredentials, FaultNoSuchDatabase, FaultDatabaseDenied,
		FaultHostNotAllowed} {
		hint := f.Hint()
		if hint == "" {
			t.Errorf("%v has nothing to check", f)
			continue
		}
		if other, dup := seen[hint]; dup {
			t.Errorf("%v and %v both say %q", f, other, hint)
		}
		seen[hint] = f
	}

	for _, f := range []Fault{FaultNone, FaultUnknown} {
		if f.Hint() != "" {
			t.Errorf("%v invented something to check: %q", f, f.Hint())
		}
	}
}

// The server refuses a database and a client host with their own numbers,
// having already accepted the password. Sending either one to dv auth costs
// the reader the time it takes to re-enter a password that was never wrong,
// and leaves them no closer to the grant or the database name that was.
func TestARefusalThatIsNotAboutThePasswordDoesNotSendAnyoneToIt(t *testing.T) {
	for _, tt := range []struct {
		fault Fault
		names string
	}{
		{FaultDatabaseDenied, "database"},
		{FaultHostNotAllowed, "host"},
	} {
		t.Run(tt.fault.String(), func(t *testing.T) {
			hint := tt.fault.Hint()
			for _, wrong := range []string{"dv auth", "password"} {
				if strings.Contains(hint, wrong) {
					t.Errorf("%v says %q, which sends the reader to %q", tt.fault, hint, wrong)
				}
			}
			if !strings.Contains(hint, tt.names) {
				t.Errorf("%v says %q, which never names %q", tt.fault, hint, tt.names)
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string { return "i/o timeout" }
func (timeoutError) Timeout() bool { return true }

var _ = errors.Is

// mysqlErrorNumber is a refusal from the server carrying a number, which is
// the only part Diagnose reads.
func mysqlErrorNumber(n uint16) error { return &mysql.MySQLError{Number: n} }
