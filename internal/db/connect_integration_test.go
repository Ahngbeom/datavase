//go:build integration

package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/testmysql"
)

func TestProbeReturnsServerVersion(t *testing.T) {
	ds, password := testmysql.DataSource(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := Probe(ctx, ds, password)
	if err != nil {
		t.Fatalf("Probe() error = %v, want nil", err)
	}
	if version == "" {
		t.Fatal("Probe() returned an empty version")
	}
	t.Logf("server version: %s", version)
}

// sql.Open succeeds even against a dead host, so Probe has to actually talk
// to the server for `dv check` to mean anything.
func TestProbeFailsAgainstAClosedPort(t *testing.T) {
	ds, password := testmysql.DataSource(t)
	ds.Port = 1 // nothing listens here

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := Probe(ctx, ds, password); err == nil {
		t.Fatal("Probe() error = nil, want a connection error")
	}
}

func TestProbeFailsWithAWrongPassword(t *testing.T) {
	ds, _ := testmysql.DataSource(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Probe(ctx, ds, "definitely-not-the-password")
	if err == nil {
		t.Fatal("Probe() error = nil, want an authentication error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "denied") {
		t.Logf("error was %v; expected an access-denied style message", err)
	}
}

func TestProbeRespectsContextCancellation(t *testing.T) {
	ds, password := testmysql.DataSource(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	if _, err := Probe(ctx, ds, password); err == nil {
		t.Fatal("Probe() error = nil, want a context error")
	}
}

// The datasource must not carry a stale database name into the connection
// when the caller left it empty.
func TestProbeWorksWithoutADefaultDatabase(t *testing.T) {
	ds, password := testmysql.DataSource(t)
	ds.Database = ""

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := Probe(ctx, ds, password); err != nil {
		t.Fatalf("Probe() error = %v, want nil", err)
	}
}
