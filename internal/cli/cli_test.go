package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/secret"
)

const testYAML = `
datasources:
  - name: local
    env: dev
    host: 127.0.0.1
    user: root
  - name: prod-app
    env: prod
    host: db.internal
    user: readonly
`

type harness struct {
	app *App
	out *bytes.Buffer
	err *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	cfg, err := config.Parse(strings.NewReader(testYAML))
	if err != nil {
		t.Fatalf("config.Parse() error = %v", err)
	}

	clearPasswordEnv(t, cfg)

	h := &harness{out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	h.app = &App{
		Config:  cfg,
		Secrets: secret.NewMemory(),
		Out:     h.out,
		Err:     h.err,
		ReadPassword: func(string) (string, error) {
			return "typed-password", nil
		},
	}
	return h
}

// clearPasswordEnv takes any ambient DATAVASE_PASSWORD_* for these
// datasources out of the way, restoring it afterwards.
//
// The commands read the environment for real, as the binary does. Without
// this, a developer who happens to export one for a datasource of their own
// called "local" would have these tests answer about their machine rather
// than about the store the test set up — and the ones that check a password
// is *absent* would be the ones to go quietly wrong.
func clearPasswordEnv(t *testing.T, cfg *config.Config) {
	t.Helper()

	for i := range cfg.DataSources {
		name := secret.EnvVarName(cfg.DataSources[i].Name)
		was, had := os.LookupEnv(name)
		if !had {
			continue
		}
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unsetting %s: %v", name, err)
		}
		t.Cleanup(func() { os.Setenv(name, was) })
	}
}

// Without a stored password the fix is always the same command, so the
// error must say so rather than surfacing a driver-level "access denied".
func TestCheckWithoutStoredPasswordPointsAtAuth(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"check", "prod-app"}); code == 0 {
		t.Fatal("Run(check) = 0, want a non-zero exit code")
	}
	if !strings.Contains(h.err.String(), "dv auth prod-app") {
		t.Errorf("stderr = %q, want it to suggest %q", h.err, "dv auth prod-app")
	}
}

func TestCheckReportsServerVersionOnSuccess(t *testing.T) {
	h := newHarness(t)
	if err := h.app.Secrets.Set("prod-app", "pw"); err != nil {
		t.Fatalf("Secrets.Set() error = %v", err)
	}
	h.app.Probe = func(_ context.Context, ds *config.DataSource, password string) (string, error) {
		if password != "pw" {
			t.Errorf("Probe got password %q, want %q", password, "pw")
		}
		if ds.Name != "prod-app" {
			t.Errorf("Probe got datasource %q, want %q", ds.Name, "prod-app")
		}
		return "11.4.2-MariaDB", nil
	}

	if code := h.app.Run([]string{"check", "prod-app"}); code != 0 {
		t.Fatalf("Run(check) = %d, want 0; stderr = %q", code, h.err)
	}
	if !strings.Contains(h.out.String(), "11.4.2-MariaDB") {
		t.Errorf("stdout = %q, want it to contain the server version", h.out)
	}
}

func TestCheckFailsWhenTheServerIsUnreachable(t *testing.T) {
	h := newHarness(t)
	if err := h.app.Secrets.Set("prod-app", "pw"); err != nil {
		t.Fatalf("Secrets.Set() error = %v", err)
	}
	h.app.Probe = func(context.Context, *config.DataSource, string) (string, error) {
		return "", errors.New("dial tcp: connection refused")
	}

	if code := h.app.Run([]string{"check", "prod-app"}); code == 0 {
		t.Fatal("Run(check) = 0, want a non-zero exit code")
	}
	if !strings.Contains(h.err.String(), "connection refused") {
		t.Errorf("stderr = %q, want it to carry the underlying error", h.err)
	}
}

func TestListShowsEveryDataSource(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"ls"}); code != 0 {
		t.Fatalf("Run(ls) = %d, want 0; stderr = %q", code, h.err)
	}

	out := h.out.String()
	for _, want := range []string{"local", "prod-app", "db.internal"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls output = %q, want it to contain %q", out, want)
		}
	}
}

// lineFor returns the ls line for one datasource, so a claim about one entry
// cannot be satisfied by what is written about another.
func lineFor(t *testing.T, out, name string) string {
	t.Helper()

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, name+" ") {
			return line
		}
	}
	t.Fatalf("ls output = %q, want a line for %q", out, name)
	return ""
}

// "stored" is a promise the environment never made: an exported variable is
// gone when the shell closes, and someone exporting a short-lived token per
// session read that word as the keychain holding onto it.
func TestListNamesTheVariableAPasswordCameFrom(t *testing.T) {
	h := newHarness(t)
	t.Setenv(secret.EnvVarName("prod-app"), "from-env")

	if code := h.app.Run([]string{"ls"}); code != 0 {
		t.Fatalf("Run(ls) = %d, want 0; stderr = %q", code, h.err)
	}

	line := lineFor(t, h.out.String(), "prod-app")
	if !strings.Contains(line, secret.EnvVarName("prod-app")) {
		t.Errorf("ls line = %q, want it to name the variable the password came from", line)
	}
	if strings.Contains(line, "keychain") {
		t.Errorf("ls line = %q, want it not to credit the keychain for an exported password", line)
	}
}

func TestListSaysWhenThePasswordIsInTheKeychain(t *testing.T) {
	h := newHarness(t)
	if err := h.app.Secrets.Set("local", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if code := h.app.Run([]string{"ls"}); code != 0 {
		t.Fatalf("Run(ls) = %d, want 0; stderr = %q", code, h.err)
	}

	out := h.out.String()
	if line := lineFor(t, out, "local"); !strings.Contains(line, "keychain") {
		t.Errorf("ls line = %q, want it to say where the password is", line)
	}
	if line := lineFor(t, out, "prod-app"); !strings.Contains(line, "no password") {
		t.Errorf("ls line = %q, want an entry with no password to say so", line)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"frobnicate"}); code == 0 {
		t.Fatal("Run(frobnicate) = 0, want a non-zero exit code")
	}
	if !strings.Contains(h.err.String(), "frobnicate") {
		t.Errorf("stderr = %q, want it to name the unknown command", h.err)
	}
}

func TestAuthStoresTypedPassword(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"auth", "prod-app"}); code != 0 {
		t.Fatalf("Run(auth) = %d, want 0; stderr = %q", code, h.err)
	}

	got, err := h.app.Secrets.Get("prod-app")
	if err != nil {
		t.Fatalf("Secrets.Get() error = %v, want nil", err)
	}
	if got != "typed-password" {
		t.Errorf("stored password = %q, want %q", got, "typed-password")
	}
}

// A typo in the datasource name must not silently create a keychain entry
// that will never be read.
func TestAuthRejectsUnknownDataSource(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"auth", "prod-ap"}); code == 0 {
		t.Fatal("Run(auth prod-ap) = 0, want a non-zero exit code")
	}
	if _, err := h.app.Secrets.Get("prod-ap"); err == nil {
		t.Error("a password was stored for an unknown datasource")
	}
}

func TestAuthRequiresADataSourceName(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"auth"}); code == 0 {
		t.Fatal("Run(auth) with no name = 0, want a non-zero exit code")
	}
}

// The password must never reach stdout, stderr or the terminal echo.
func TestAuthNeverPrintsThePassword(t *testing.T) {
	h := newHarness(t)

	h.app.Run([]string{"auth", "prod-app"})

	if strings.Contains(h.out.String()+h.err.String(), "typed-password") {
		t.Errorf("password leaked into output: stdout=%q stderr=%q", h.out, h.err)
	}
}

// stubSecrets is a minimal secret.Store for tests that only need a fixed
// answer, without the bookkeeping secret.Memory offers.
type stubSecrets map[string]string

func (s stubSecrets) Get(account string) (string, error) {
	pw, ok := s[account]
	if !ok {
		return "", secret.ErrNotFound
	}
	return pw, nil
}
func (s stubSecrets) Set(account, password string) error { s[account] = password; return nil }
func (s stubSecrets) Delete(account string) error        { delete(s, account); return nil }

func TestRmDeletesStoredPassword(t *testing.T) {
	h := newHarness(t)
	if err := h.app.Secrets.Set("prod-app", "pw"); err != nil {
		t.Fatalf("Secrets.Set() error = %v", err)
	}

	if code := h.app.Run([]string{"auth", "-rm", "prod-app"}); code != 0 {
		t.Fatalf("Run(auth -rm) = %d, want 0; stderr = %q", code, h.err)
	}

	if _, err := h.app.Secrets.Get("prod-app"); err == nil {
		t.Error("password still present after auth -rm")
	}
}

func TestOpeningWithNoNameAndSeveralDatasourcesLaunchesTheList(t *testing.T) {
	launched := false
	app := &App{
		Config: &config.Config{DataSources: []config.DataSource{{Name: "a"}, {Name: "b"}}},
		Out:    io.Discard, Err: io.Discard,
		Launch: func() error { launched = true; return nil },
	}
	if code := app.Run(nil); code != exitOK || !launched {
		t.Errorf("Run() = %d, launched = %v; want the list", code, launched)
	}
}

func TestOpeningWithNoDatasourcesLaunchesTheList(t *testing.T) {
	launched := false
	app := &App{Config: config.Empty(), Out: io.Discard, Err: io.Discard,
		Launch: func() error { launched = true; return nil }}
	if code := app.Run(nil); code != exitOK || !launched {
		t.Errorf("Run() = %d, launched = %v; a first run must open the list", code, launched)
	}
}

// "connecting failed" is the one thing the reader already knows. What they
// cannot tell from it is whether to look at this file, their VPN, or the
// password they stored, and check exists to be run at exactly that moment.
func TestCheckSaysWhatToLookAtWhenItCanTell(t *testing.T) {
	h := newHarness(t)
	if err := h.app.Secrets.Set("prod-app", "pw"); err != nil {
		t.Fatalf("Secrets.Set() error = %v", err)
	}
	h.app.Probe = func(context.Context, *config.DataSource, string) (string, error) {
		return "", &net.DNSError{Err: "no such host", Name: "db.internal", IsNotFound: true}
	}

	if code := h.app.Run([]string{"check", "prod-app"}); code == 0 {
		t.Fatal("Run(check) = 0, want a non-zero exit code")
	}
	if got := h.err.String(); !strings.Contains(got, db.FaultUnresolved.Hint()) {
		t.Errorf("stderr = %q, want it to say what to check", got)
	}
}
