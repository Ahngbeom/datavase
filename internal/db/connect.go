package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Ahngbeom/datavase/internal/config"
	_ "github.com/go-sql-driver/mysql" // registers the "mysql" driver
)

// Probe opens a connection to ds and returns the server version.
//
// sql.Open only validates the DSN, so it succeeds even against a host that
// is down. The version query is what actually proves reachability.
func Probe(ctx context.Context, ds *config.DataSource, password string) (string, error) {
	dsn, err := DSN(ds, password, "")
	if err != nil {
		return "", err
	}

	handle, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", fmt.Errorf("opening connection: %w", err)
	}
	defer handle.Close()

	var version string
	if err := handle.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}
