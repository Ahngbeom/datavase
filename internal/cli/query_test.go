package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/headless"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

const queryYAML = `
datasources:
  - name: seed
    host: 127.0.0.1
    user: claude_ro
    read_only: true
  - name: writable
    host: 127.0.0.1
    user: root
defaults:
  auto_limit: 250
`

type queryCall struct {
	ds   string
	pw   string
	sql  string
	lims headless.Limits
}

type queryHarness struct {
	*harness
	calls []queryCall
	reply func(w io.Writer) error
}

func newQueryHarness(t *testing.T, stdin string) *queryHarness {
	t.Helper()

	cfg, err := config.Parse(strings.NewReader(queryYAML))
	if err != nil {
		t.Fatalf("config.Parse() error = %v", err)
	}
	clearPasswordEnv(t, cfg)

	h := &queryHarness{harness: &harness{out: &bytes.Buffer{}, err: &bytes.Buffer{}}}
	h.app = &App{
		Config:  cfg,
		Secrets: secret.NewMemory(),
		In:      strings.NewReader(stdin),
		Out:     h.out,
		Err:     h.err,
		Query: func(_ context.Context, ds *config.DataSource, pw string, stmt sqlparse.Statement, lim headless.Limits, w io.Writer) error {
			h.calls = append(h.calls, queryCall{ds.Name, pw, stmt.SQL, lim})
			if h.reply != nil {
				return h.reply(w)
			}
			return nil
		},
	}
	for _, name := range []string{"seed", "writable"} {
		if err := h.app.Secrets.Set(name, "pw-"+name); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func TestQueryHandsTheStatementAndItsLimitsOverAndPassesTheResultThrough(t *testing.T) {
	h := newQueryHarness(t, "SELECT id FROM users;\n")
	h.reply = func(w io.Writer) error {
		_, err := io.WriteString(w, "{\"rows\":0}\n")
		return err
	}

	code := h.app.Run([]string{"query", "-max-rows", "5", "-timeout", "2s", "seed"})
	if code != exitOK {
		t.Fatalf("Run(query) = %d, stderr = %q", code, h.err)
	}
	if len(h.calls) != 1 {
		t.Fatalf("Query called %d times, want once", len(h.calls))
	}
	want := queryCall{"seed", "pw-seed", "SELECT id FROM users", headless.Limits{MaxRows: 5, Timeout: 2 * time.Second}}
	if h.calls[0] != want {
		t.Errorf("Query got %+v, want %+v", h.calls[0], want)
	}
	if h.out.String() != "{\"rows\":0}\n" {
		t.Errorf("stdout = %q, want the result untouched", h.out)
	}
}

// Without flags the cap is the one the configuration already sets for
// unbounded SELECTs in the interface, so the two cannot disagree.
func TestQueryCapsRowsAtTheConfiguredAutoLimitByDefault(t *testing.T) {
	h := newQueryHarness(t, "SELECT 1")

	if code := h.app.Run([]string{"query", "seed"}); code != exitOK {
		t.Fatalf("Run(query) = %d, stderr = %q", code, h.err)
	}
	if got := h.calls[0].lims; got.MaxRows != 250 || got.Timeout != DefaultQueryTimeout {
		t.Errorf("limits = %+v, want 250 rows and %s", got, DefaultQueryTimeout)
	}
}

// Every refusal says so with its own exit code, so whatever runs dv can tell
// "this was not allowed" from "the database could not be reached".
func TestQueryRefusesWithoutConnecting(t *testing.T) {
	for _, tc := range []struct {
		name, stdin, ds, want string
	}{
		{"a datasource that is not read_only", "SELECT 1", "writable", "read_only"},
		{"a write", "UPDATE users SET email = NULL", "seed", "UPDATE"},
		{"two statements", "SELECT 1; SELECT 2", "seed", "one statement"},
		{"nothing at all", "  \n", "seed", "no statement"},
		{"a statement too long to be a query", "SELECT '" + strings.Repeat("x", MaxQueryBytes) + "'", "seed", "bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newQueryHarness(t, tc.stdin)

			if code := h.app.Run([]string{"query", tc.ds}); code != exitRefused {
				t.Errorf("Run(query) = %d, want %d (refused)", code, exitRefused)
			}
			if len(h.calls) != 0 {
				t.Errorf("Query was called; a refusal must come before any connection")
			}
			if !strings.Contains(h.err.String(), tc.want) {
				t.Errorf("stderr = %q, want it to name %q", h.err, tc.want)
			}
		})
	}
}

// The server refusing the session, or the runner finding it unconfirmed, is
// the same answer as a refusal here: the request was not allowed to run.
func TestQueryReportsARefusalFromTheRunnerAsARefusal(t *testing.T) {
	h := newQueryHarness(t, "SELECT 1")
	h.reply = func(io.Writer) error { return &headless.Refusal{Reason: "not confirmed"} }

	if code := h.app.Run([]string{"query", "seed"}); code != exitRefused {
		t.Errorf("Run(query) = %d, want %d", code, exitRefused)
	}
}

func TestQueryFailsPlainlyWhenTheDatabaseDoes(t *testing.T) {
	h := newQueryHarness(t, "SELECT 1")
	h.reply = func(io.Writer) error { return fmt.Errorf("dial tcp 127.0.0.1:3306: connection refused") }

	if code := h.app.Run([]string{"query", "seed"}); code != exitError {
		t.Errorf("Run(query) = %d, want %d", code, exitError)
	}
	if !strings.Contains(h.err.String(), "connection refused") {
		t.Errorf("stderr = %q, want the cause", h.err)
	}
}

// A machine running dv for a program is the machine with no keychain, so the
// way out named has to be the environment variable.
func TestQueryWithoutAPasswordNamesTheVariable(t *testing.T) {
	h := newQueryHarness(t, "SELECT 1")
	h.app.Secrets.Delete("seed")

	if code := h.app.Run([]string{"query", "seed"}); code != exitError {
		t.Errorf("Run(query) = %d, want %d", code, exitError)
	}
	if !strings.Contains(h.err.String(), secret.EnvVarName("seed")) {
		t.Errorf("stderr = %q, want it to name %s", h.err, secret.EnvVarName("seed"))
	}
}

func TestQueryUsageMistakes(t *testing.T) {
	for _, args := range [][]string{
		{"query"},
		{"query", "seed", "writable"},
		{"query", "-max-rows", "0", "seed"},
		{"query", "-max-rows", "999999", "seed"},
		{"query", "-timeout", "0s", "seed"},
		{"query", "-c", "/elsewhere.yaml", "seed"},
	} {
		h := newQueryHarness(t, "SELECT 1")
		if code := h.app.Run(args); code != exitUsage {
			t.Errorf("Run(%q) = %d, want %d", args, code, exitUsage)
		}
		if len(h.calls) != 0 {
			t.Errorf("Run(%q) called Query", args)
		}
	}
}
