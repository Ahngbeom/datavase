package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
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

// Attaching must not ask for a password. The prompt needs a terminal and the
// connection is opened in another process, which is why a switch mid-session
// has never prompted either.
func TestOpenPrefersAttachAndDoesNotPrompt(t *testing.T) {
	var attached string
	app := &App{
		Config: &config.Config{DataSources: []config.DataSource{{Name: "local", Env: config.EnvDev}}},
		Out:    io.Discard,
		Err:    io.Discard,
		ReadPassword: func(string) (string, error) {
			t.Error("attaching asked for a password")
			return "", nil
		},
		Attach: func(_ context.Context, ds *config.DataSource, _ *config.Config, _ UIOptions) error {
			attached = ds.Name
			return nil
		},
		OpenUI: func(context.Context, *config.DataSource, string, *config.Config, UIOptions) error {
			t.Error("OpenUI was used while a runtime was available")
			return nil
		},
	}

	if code := app.Run(nil); code != exitOK {
		t.Fatalf("Run = %d, want %d", code, exitOK)
	}
	if attached != "local" {
		t.Errorf("attached to %q, want \"local\"", attached)
	}
}

// --no-session is the escape hatch, and it has to keep the behaviour that
// existed before there was anything to escape from.
func TestWithoutARuntimeOpenUIStillRuns(t *testing.T) {
	var opened string
	app := &App{
		Config:       &config.Config{DataSources: []config.DataSource{{Name: "local", Env: config.EnvDev}}},
		Out:          io.Discard,
		Err:          io.Discard,
		Secrets:      stubSecrets{"local": "hunter2"},
		ReadPassword: func(string) (string, error) { return "hunter2", nil },
		OpenUI: func(_ context.Context, ds *config.DataSource, pw string, _ *config.Config, _ UIOptions) error {
			opened = ds.Name + ":" + pw
			return nil
		},
	}

	if code := app.Run(nil); code != exitOK {
		t.Fatalf("Run = %d, want %d", code, exitOK)
	}
	if opened != "local:hunter2" {
		t.Errorf("opened %q, want \"local:hunter2\"", opened)
	}
}

// dv status has to answer even when there is no server, because "is one
// running" is the question it exists for.
func TestStatusReportsWithNoServer(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Config:       &config.Config{},
		Out:          &out,
		Err:          io.Discard,
		ServerStatus: func() (string, error) { return "no dv server is running", nil },
	}

	if code := app.Run([]string{"status"}); code != exitOK {
		t.Fatalf("Run = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out.String(), "no dv server") {
		t.Errorf("status printed %q", out.String())
	}
}

// dv server stop must reach StopServer with force set only when --force was
// typed, since sending SIGTERM to anything is a choice only --force may make.
func TestServerStopPassesForceOnlyWhenTyped(t *testing.T) {
	h := newHarness(t)

	var got []bool
	h.app.StopServer = func(force bool) error {
		got = append(got, force)
		return nil
	}

	if code := h.app.Run([]string{"server", "stop"}); code != exitOK {
		t.Fatalf("Run(server stop) = %d, want %d", code, exitOK)
	}
	if code := h.app.Run([]string{"server", "stop", "--force"}); code != exitOK {
		t.Fatalf("Run(server stop --force) = %d, want %d", code, exitOK)
	}

	if len(got) != 2 || got[0] != false || got[1] != true {
		t.Fatalf("StopServer called with force=%v, want [false true]", got)
	}
}

// Whatever StopServer says about a session that would not end must reach the
// user — that sentence, naming the pid, is the whole recovery path when a
// wedged session leaves the stop request unanswered.
func TestServerStopReportsWhatStopServerSaid(t *testing.T) {
	h := newHarness(t)
	h.app.StopServer = func(bool) error {
		return errors.New(`a dv server is running (pid 82515); it did not stop within 5s.

  dv server stop --force   end it by signalling that pid directly`)
	}

	code := h.app.Run([]string{"server", "stop"})
	if code == exitOK {
		t.Fatal("Run(server stop) = 0, want a non-zero exit code")
	}
	if !strings.Contains(h.err.String(), "pid 82515") {
		t.Errorf("stderr = %q, want it to name the pid", h.err.String())
	}
	if !strings.Contains(h.err.String(), "--force") {
		t.Errorf("stderr = %q, want it to name --force", h.err.String())
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
