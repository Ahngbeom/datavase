// Package testmysql supplies connection details for the integration tests.
//
// The tests talk to a real MariaDB rather than a stub because the behaviour
// they pin down — server-side cancellation, streaming, information_schema
// timing — only exists on a real server.
//
// Start one with: make db-up
package testmysql

import (
	"os"
	"strconv"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

// Defaults matching the container started by `make db-up`. The port is
// deliberately not 3306 so the tests never reach a developer's own MySQL.
const (
	DefaultHost     = "127.0.0.1"
	DefaultPort     = 13306
	DefaultUser     = "root"
	DefaultPassword = "datavase-test"
	DefaultDatabase = "datavase_test"
)

// Environment variables that override the defaults.
const (
	EnvHost     = "DATAVASE_TEST_HOST"
	EnvPort     = "DATAVASE_TEST_PORT"
	EnvUser     = "DATAVASE_TEST_USER"
	EnvPassword = "DATAVASE_TEST_PASSWORD"
	EnvDatabase = "DATAVASE_TEST_DATABASE"
)

// DataSource returns the test datasource and its password.
//
// It is labelled EnvDev so guard's production rules never apply to test
// fixtures by accident.
func DataSource(t *testing.T) (*config.DataSource, string) {
	t.Helper()

	return &config.DataSource{
		Name:     "integration",
		Env:      config.EnvDev,
		Host:     envOr(EnvHost, DefaultHost),
		Port:     envIntOr(t, EnvPort, DefaultPort),
		User:     envOr(EnvUser, DefaultUser),
		Database: envOr(EnvDatabase, DefaultDatabase),

		// Stated rather than left to the env default, because tests relabel
		// this datasource: a harness asking for a production environment
		// would otherwise inherit "required" and fail to reach a container
		// on loopback that speaks no TLS — for reasons having nothing to do
		// with what the test was about.
		TLS: config.TLSDisabled,
	}, envOr(EnvPassword, DefaultPassword)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(t *testing.T, key string, fallback int) int {
	t.Helper()

	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("%s = %q, want an integer: %v", key, v, err)
	}
	return n
}
