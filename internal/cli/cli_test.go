package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
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

func TestListShowsEveryDataSourceWithItsEnv(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"ls"}); code != 0 {
		t.Fatalf("Run(ls) = %d, want 0; stderr = %q", code, h.err)
	}

	out := h.out.String()
	for _, want := range []string{"local", "dev", "prod-app", "prod", "db.internal"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls output = %q, want it to contain %q", out, want)
		}
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
