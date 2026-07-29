package db

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/go-sql-driver/mysql"
)

func testDataSource() *config.DataSource {
	return &config.DataSource{
		Name:     "prod-app",
		Env:      config.EnvProd,
		Host:     "db.internal",
		Port:     3306,
		User:     "readonly",
		Database: "app_db",
	}
}

func TestDSNCarriesConnectionDetails(t *testing.T) {
	got, err := mysql.ParseDSN(DSN(testDataSource(), "hunter2", ""))
	if err != nil {
		t.Fatalf("ParseDSN() error = %v, want nil", err)
	}

	if got.User != "readonly" {
		t.Errorf("User = %q, want %q", got.User, "readonly")
	}
	if got.Passwd != "hunter2" {
		t.Errorf("Passwd = %q, want %q", got.Passwd, "hunter2")
	}
	if got.Addr != "db.internal:3306" {
		t.Errorf("Addr = %q, want %q", got.Addr, "db.internal:3306")
	}
	if got.DBName != "app_db" {
		t.Errorf("DBName = %q, want %q", got.DBName, "app_db")
	}
	if got.Net != "tcp" {
		t.Errorf("Net = %q, want %q", got.Net, "tcp")
	}
}

// Production passwords contain "@", "/" and ":" often enough that hand-built
// DSN strings break on them. Round-tripping proves the escaping holds.
func TestDSNSurvivesSpecialCharactersInPassword(t *testing.T) {
	passwords := []string{
		"p@ssw0rd",
		"a/b",
		"colon:pass",
		"tcp(evil)",
		"trailing?query=1",
		`quote"and'both`,
		"한글비밀번호",
	}

	for _, want := range passwords {
		t.Run(want, func(t *testing.T) {
			got, err := mysql.ParseDSN(DSN(testDataSource(), want, ""))
			if err != nil {
				t.Fatalf("ParseDSN() error = %v, want nil", err)
			}
			if got.Passwd != want {
				t.Errorf("Passwd = %q, want %q", got.Passwd, want)
			}
			if got.Addr != "db.internal:3306" {
				t.Errorf("Addr = %q, want %q; password leaked into the address",
					got.Addr, "db.internal:3306")
			}
		})
	}
}

// multiStatements would let "SELECT 1; DROP TABLE users" through as a single
// call, so guard would inspect one statement while the server ran two.
func TestDSNDisablesMultiStatements(t *testing.T) {
	raw := DSN(testDataSource(), "pw", "")
	if strings.Contains(raw, "multiStatements=true") {
		t.Fatalf("DSN = %q, must not enable multiStatements", raw)
	}

	got, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatalf("ParseDSN() error = %v", err)
	}
	if got.MultiStatements {
		t.Error("MultiStatements = true, want false; guard could be bypassed")
	}
}

func TestDSNParsesTimeValues(t *testing.T) {
	got, err := mysql.ParseDSN(DSN(testDataSource(), "pw", ""))
	if err != nil {
		t.Fatalf("ParseDSN() error = %v", err)
	}
	if !got.ParseTime {
		t.Error("ParseTime = false, want true")
	}
}

// When a tunnel is up the driver must dial the local listener, not the
// remote host, while every other field stays the same.
func TestDSNUsesTunnelAddressWhenGiven(t *testing.T) {
	got, err := mysql.ParseDSN(DSN(testDataSource(), "pw", "127.0.0.1:54321"))
	if err != nil {
		t.Fatalf("ParseDSN() error = %v", err)
	}

	if got.Addr != "127.0.0.1:54321" {
		t.Errorf("Addr = %q, want %q", got.Addr, "127.0.0.1:54321")
	}
	if got.DBName != "app_db" {
		t.Errorf("DBName = %q, want %q", got.DBName, "app_db")
	}
}

// An empty database is legitimate: the user picks a schema from the tree.
func TestDSNAllowsEmptyDatabase(t *testing.T) {
	ds := testDataSource()
	ds.Database = ""

	got, err := mysql.ParseDSN(DSN(ds, "pw", ""))
	if err != nil {
		t.Fatalf("ParseDSN() error = %v, want nil", err)
	}
	if got.DBName != "" {
		t.Errorf("DBName = %q, want empty", got.DBName)
	}
}
