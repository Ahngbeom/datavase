package db

import (
	"os"
	"path/filepath"
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
	rawGot, err := DSN(testDataSource(), "hunter2", "")
	if err != nil {
		t.Fatalf("DSN() error = %v, want nil", err)
	}
	got, err := mysql.ParseDSN(rawGot)
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
			rawGot, err := DSN(testDataSource(), want, "")
			if err != nil {
				t.Fatalf("DSN() error = %v, want nil", err)
			}
			got, err := mysql.ParseDSN(rawGot)
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
	raw, err := DSN(testDataSource(), "pw", "")
	if err != nil {
		t.Fatalf("DSN() error = %v, want nil", err)
	}
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
	rawGot, err := DSN(testDataSource(), "pw", "")
	if err != nil {
		t.Fatalf("DSN() error = %v, want nil", err)
	}
	got, err := mysql.ParseDSN(rawGot)
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
	rawGot, err := DSN(testDataSource(), "pw", "127.0.0.1:54321")
	if err != nil {
		t.Fatalf("DSN() error = %v, want nil", err)
	}
	got, err := mysql.ParseDSN(rawGot)
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

	rawGot, err := DSN(ds, "pw", "")
	if err != nil {
		t.Fatalf("DSN() error = %v, want nil", err)
	}
	got, err := mysql.ParseDSN(rawGot)
	if err != nil {
		t.Fatalf("ParseDSN() error = %v, want nil", err)
	}
	if got.DBName != "" {
		t.Errorf("DBName = %q, want empty", got.DBName)
	}
}

// A datasource that says nothing about TLS still gets the decision its env
// implies. DSN is reached by callers that never went through config.Parse —
// the test fixtures among them — so the fallback cannot live only there.
func TestDSNAppliesTheEnvironmentsDefaultWhenTLSIsUnset(t *testing.T) {
	tests := []struct {
		env  config.Env
		want string
	}{
		{config.EnvProd, "skip-verify"},
		{config.EnvStage, "preferred"},
		{config.EnvDev, "preferred"},
	}

	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			ds := testDataSource()
			ds.Env = tt.env
			ds.TLS = ""

			raw, err := DSN(ds, "pw", "")
			if err != nil {
				t.Fatalf("DSN() error = %v, want nil", err)
			}
			got, err := mysql.ParseDSN(raw)
			if err != nil {
				t.Fatalf("ParseDSN() error = %v", err)
			}
			if got.TLSConfig != tt.want {
				t.Errorf("TLSConfig = %q, want %q", got.TLSConfig, tt.want)
			}
		})
	}
}

func TestDSNTranslatesEveryTLSMode(t *testing.T) {
	tests := []struct {
		mode config.TLSMode
		want string
	}{
		{config.TLSDisabled, "false"},
		{config.TLSPreferred, "preferred"},
		{config.TLSRequired, "skip-verify"},
		{config.TLSVerifyIdentity, "true"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			ds := testDataSource()
			ds.TLS = tt.mode

			raw, err := DSN(ds, "pw", "")
			if err != nil {
				t.Fatalf("DSN() error = %v, want nil", err)
			}
			got, err := mysql.ParseDSN(raw)
			if err != nil {
				t.Fatalf("ParseDSN() error = %v", err)
			}
			if got.TLSConfig != tt.want {
				t.Errorf("TLSConfig = %q, want %q", got.TLSConfig, tt.want)
			}
		})
	}
}

// verify-ca has no driver shorthand: it checks the chain but not the name, so
// it is a registered configuration rather than one of the reserved words.
func TestVerifyCARegistersAConfigurationThatChecksTheChain(t *testing.T) {
	ds := testDataSource()
	ds.TLS = config.TLSVerifyCA

	raw, err := DSN(ds, "pw", "")
	if err != nil {
		t.Fatalf("DSN() error = %v, want nil", err)
	}
	got, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatalf("ParseDSN() error = %v", err)
	}

	for _, reserved := range []string{"true", "false", "skip-verify", "preferred"} {
		if got.TLSConfig == reserved {
			t.Fatalf("TLSConfig = %q; verify-ca is none of the driver's shorthands", reserved)
		}
	}
	if got.TLSConfig == "" {
		t.Fatal("TLSConfig is empty, so nothing would be verified")
	}
}

// A CA that cannot be read must stop the connection rather than quietly
// falling back to the system roots, which would verify against a different
// set of authorities than the one the operator named.
func TestDSNFailsWhenTheNamedAuthorityCannotBeRead(t *testing.T) {
	ds := testDataSource()
	ds.TLS = config.TLSVerifyIdentity
	ds.TLSCA = filepath.Join(t.TempDir(), "absent.pem")

	if _, err := DSN(ds, "pw", ""); err == nil {
		t.Fatal("DSN() error = nil, want a refusal to continue without the named roots")
	}
}

func TestDSNFailsWhenTheNamedAuthorityHoldsNoCertificate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(path, []byte("not a certificate\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ds := testDataSource()
	ds.TLS = config.TLSVerifyCA
	ds.TLSCA = path

	if _, err := DSN(ds, "pw", ""); err == nil {
		t.Fatal("DSN() error = nil, want a refusal of a file holding no roots")
	}
}
