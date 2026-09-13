//go:build integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/testmysql"
)

// Diagnose reads error shapes the driver and the standard library produce,
// and neither promises to keep producing them. Synthesised errors in the unit
// test say what the classification does; these say it still matches what a
// real failure looks like.
func TestRealFailuresAreClassifiedTheWayTheUnitTestSaysTheyAre(t *testing.T) {
	for _, tt := range []struct {
		name    string
		break_  func(*config.DataSource, *string)
		want    Fault
		timeout time.Duration
	}{
		{"a password the server refuses",
			func(_ *config.DataSource, pw *string) { *pw = "not-the-password" },
			FaultCredentials, 10 * time.Second},
		{"a database that is not on the server",
			func(ds *config.DataSource, _ *string) { ds.Database = "dv_no_such_database" },
			FaultNoSuchDatabase, 10 * time.Second},
		{"a host name that does not resolve",
			func(ds *config.DataSource, _ *string) { ds.Host = "dv.no.such.host.invalid" },
			FaultUnresolved, 10 * time.Second},
		{"a port with nothing listening on it",
			func(ds *config.DataSource, _ *string) { ds.Port = 13999 },
			FaultRefused, 10 * time.Second},
		{"a certificate this machine will not verify",
			func(ds *config.DataSource, _ *string) { ds.TLS = config.TLSVerifyIdentity },
			FaultTLS, 10 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ds, password := testmysql.DataSource(t)
			tt.break_(ds, &password)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			conn, err := Open(ctx, ds, password, "")
			if err == nil {
				conn.Close()
				t.Fatal("the connection was supposed to fail and did not")
			}
			if got := Diagnose(err); got != tt.want {
				t.Errorf("Diagnose(%v) = %v, want %v", err, got, tt.want)
			}
		})
	}
}
