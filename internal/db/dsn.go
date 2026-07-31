// Package db manages MySQL connections, statement execution and cancellation.
package db

import (
	"net"
	"strconv"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/go-sql-driver/mysql"
)

// DialTimeout bounds how long a connection attempt waits before failing, so
// an unreachable host does not hang the UI.
const DialTimeout = 10 * time.Second

// DSN builds a driver connection string for ds.
//
// addr overrides the host:port the driver dials, which is how an SSH tunnel
// is wired in: the datasource still describes the remote database while the
// driver connects to the local listener.
//
// The DSN is assembled through mysql.Config rather than string concatenation
// so that passwords containing "@", "/" or ":" are escaped correctly.
//
// It fails rather than returning a usable-looking string when the TLS
// settings cannot be honoured: a connection that quietly verified less than
// was asked of it is the failure this whole path exists to prevent.
func DSN(ds *config.DataSource, password, addr string) (string, error) {
	if addr == "" {
		addr = net.JoinHostPort(ds.Host, strconv.Itoa(ds.Port))
	}

	// An unset mode is resolved here rather than only in config.Parse,
	// because DSN is also reached by callers that build a DataSource
	// directly, and an empty string would otherwise mean "no TLS at all".
	mode := ds.TLS
	if mode == "" {
		mode = config.DefaultTLSMode(ds.Env)
	}
	tlsConfig, err := tlsSetting(ds, mode)
	if err != nil {
		return "", err
	}

	c := mysql.NewConfig()
	c.User = ds.User
	c.Passwd = password
	c.Net = "tcp"
	c.Addr = addr
	c.DBName = ds.Database

	// Return DATE/DATETIME as time.Time so the grid can format them.
	c.ParseTime = true
	c.Loc = time.Local
	c.Timeout = DialTimeout
	c.TLSConfig = tlsConfig

	// Left off deliberately: with multiple statements per round trip, guard
	// would vet one statement while the server executed several.
	c.MultiStatements = false

	return c.FormatDSN(), nil
}
