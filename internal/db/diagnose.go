package db

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"syscall"

	"github.com/go-sql-driver/mysql"
)

// Fault is which hop between here and the database failed.
//
// The driver reports all of them the same way — a connection that did not
// happen — and the difference is who can do something about it. A name, a
// port, a VPN, a certificate, a password, an absent database, a database
// this account was never granted and a server that turns this machine away
// are eight different mornings.
type Fault int

const (
	// FaultNone is no failure at all.
	FaultNone Fault = iota
	// FaultUnknown is a failure this package has nothing to add to. The
	// server's or the driver's own words stand on their own.
	FaultUnknown
	// FaultUnresolved is a host name that did not resolve.
	FaultUnresolved
	// FaultRefused is a port with nothing listening on it.
	FaultRefused
	// FaultUnreachable is no answer at all before the deadline.
	FaultUnreachable
	// FaultTLS is a certificate that was not accepted, or a server with no
	// TLS to offer a datasource that requires it.
	FaultTLS
	// FaultCredentials is the server refusing who this is.
	FaultCredentials
	// FaultNoSuchDatabase is a server reached and a database that is not on it.
	FaultNoSuchDatabase
	// FaultDatabaseDenied is a password the server accepted and a database it
	// then refused. MySQL answers "does not exist" and "you have no grant on
	// it" with this one number on purpose, so that an account cannot learn
	// which databases exist by reading the refusals; the hint has to carry
	// both possibilities for the same reason.
	FaultDatabaseDenied
	// FaultHostNotAllowed is the server turning this machine away before any
	// password was asked for.
	FaultHostNotAllowed
)

// errNoTLS is the driver's own sentinel for a server that cannot do TLS,
// which is a configuration answer rather than a network one.
var errNoTLS = mysql.ErrNoTLS

// MySQL's numbers for the refusals that are about who you are rather than
// what you asked for.
const (
	erAccessDenied       = 1045
	erDBAccessDenied     = 1044
	erHostNotPrivileged  = 1130
	erBadDB              = 1049
	erAccessDeniedNoPass = 1698
)

// Diagnose says which hop failed, as far as it can be told apart.
//
// The server's numbered refusals are read first: a number is proof the
// connection reached the database and was answered, so nothing underneath it
// is what went wrong, however the socket looks afterwards.
func Diagnose(err error) Fault {
	if err == nil {
		return FaultNone
	}

	switch {
	case isMySQLError(err, erAccessDenied),
		isMySQLError(err, erAccessDeniedNoPass):
		return FaultCredentials
	case isMySQLError(err, erDBAccessDenied):
		return FaultDatabaseDenied
	case isMySQLError(err, erHostNotPrivileged):
		return FaultHostNotAllowed
	case isMySQLError(err, erBadDB):
		return FaultNoSuchDatabase
	}

	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) || errors.Is(err, errNoTLS) {
		return FaultTLS
	}

	// Before the timeout check: a name lookup that gave up waiting is still a
	// name nobody could resolve, and "check your VPN reaches the host" is the
	// wrong errand when the host is a typo.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return FaultUnresolved
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return FaultRefused
	}

	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return FaultUnreachable
	}
	return FaultUnknown
}

// Hint is what to go and look at. Empty where there is nothing specific to
// say, because a sentence that fits every failure directs nobody.
//
// Each names the configuration key or the thing outside this program that
// decides it, since a fault the reader cannot act on is only a longer way of
// saying the connection failed.
func (f Fault) Hint() string {
	switch f {
	case FaultUnresolved:
		return "the host name did not resolve — check host, and whether the VPN or DNS it needs is up"
	case FaultRefused:
		return "nothing is listening there — check port, and whether the server or the proxy in front of it is running"
	case FaultUnreachable:
		return "no answer before the timeout — check whether a VPN, firewall or security group lets this machine reach it"
	case FaultTLS:
		return "the certificate was not accepted — check tls and tls_ca for this datasource"
	case FaultCredentials:
		return "the server refused the credentials — check user, and the stored password with dv auth"
	case FaultNoSuchDatabase:
		return "the server has no such database — check database for this datasource"
	case FaultDatabaseDenied:
		return "the credentials were accepted and the database was not — check database for this datasource, and whether this user is granted it"
	case FaultHostNotAllowed:
		return "the server refuses connections from this machine — check which host this user is granted, and which one you are reaching it from"
	}
	return ""
}

// String names the fault for a test's failure message.
func (f Fault) String() string {
	switch f {
	case FaultNone:
		return "no fault"
	case FaultUnresolved:
		return "unresolved host"
	case FaultRefused:
		return "connection refused"
	case FaultUnreachable:
		return "unreachable"
	case FaultTLS:
		return "TLS"
	case FaultCredentials:
		return "credentials"
	case FaultNoSuchDatabase:
		return "no such database"
	case FaultDatabaseDenied:
		return "database denied"
	case FaultHostNotAllowed:
		return "host not allowed"
	}
	return "unknown"
}
