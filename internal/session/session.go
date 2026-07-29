// Package session opens a datasource, raising an SSH tunnel first when the
// configuration calls for one.
//
// It exists so that "connect to this datasource" is one step for every
// caller, and so the tunnel's lifetime is tied to the connection's rather
// than left to each command to remember.
package session

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/tunnel"
	"golang.org/x/crypto/ssh"
)

// Session is a database connection plus the tunnel carrying it, if any.
type Session struct {
	Conn   *db.Conn
	tunnel *tunnel.Tunnel
}

// Options override how the bastion is authenticated and verified.
//
// Both default to the real thing — ssh-agent and ~/.ssh/known_hosts — and
// exist as fields so tests can drive the same Open path against an
// in-process SSH server.
type Options struct {
	Auth            []ssh.AuthMethod
	HostKeyCallback ssh.HostKeyCallback
}

// Open connects to ds, dialling through its bastion when one is configured.
func Open(ctx context.Context, ds *config.DataSource, password string) (*Session, error) {
	return OpenWith(ctx, ds, password, Options{})
}

// OpenWith is Open with explicit SSH credentials.
func OpenWith(ctx context.Context, ds *config.DataSource, password string, opt Options) (*Session, error) {
	if ds.Tunnel == nil {
		conn, err := db.Open(ctx, ds, password, "")
		if err != nil {
			return nil, err
		}
		return &Session{Conn: conn}, nil
	}

	opt, err := opt.withDefaults(ds.Name)
	if err != nil {
		return nil, err
	}

	tun, err := tunnel.Open(ctx, tunnel.Options{
		Bastion:         ds.Tunnel,
		Remote:          remoteAddr(ds),
		Auth:            opt.Auth,
		HostKeyCallback: opt.HostKeyCallback,
	})
	if err != nil {
		return nil, err
	}

	// The driver dials the local end of the tunnel; the datasource still
	// describes the database as it is named on the far side.
	conn, err := db.Open(ctx, ds, password, tun.LocalAddr())
	if err != nil {
		tun.Close()

		// A tunnel that failed to forward explains the failure far better
		// than the driver's bare "connection reset" would.
		if forwardErr := tun.Err(); forwardErr != nil {
			return nil, fmt.Errorf("connecting through bastion %s: %w (%v)",
				ds.Tunnel.Host, forwardErr, err)
		}
		return nil, err
	}

	return &Session{Conn: conn, tunnel: tun}, nil
}

// withDefaults fills in the production credential sources.
func (o Options) withDefaults(dsName string) (Options, error) {
	if o.Auth == nil {
		auth, err := tunnel.AgentAuth()
		if err != nil {
			return o, fmt.Errorf("datasource %q needs an SSH tunnel: %w", dsName, err)
		}
		o.Auth = auth
	}
	if o.HostKeyCallback == nil {
		callback, err := tunnel.KnownHostsCallback()
		if err != nil {
			return o, fmt.Errorf("datasource %q needs an SSH tunnel: %w", dsName, err)
		}
		o.HostKeyCallback = callback
	}
	return o, nil
}

// Close releases the connection and then the tunnel underneath it.
func (s *Session) Close() error {
	err := s.Conn.Close()
	if s.tunnel != nil {
		s.tunnel.Close()
	}
	return err
}

// TunnelErr reports a forwarding failure that happened after connecting,
// which is how a bastion dropping mid-session becomes visible.
func (s *Session) TunnelErr() error {
	if s.tunnel == nil {
		return nil
	}
	return s.tunnel.Err()
}

func remoteAddr(ds *config.DataSource) string {
	return net.JoinHostPort(ds.Host, strconv.Itoa(ds.Port))
}
