package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

// openedWith runs the command and reports what the interface was asked for.
func openedWith(t *testing.T, args ...string) (name, workDir string, code int) {
	t.Helper()

	h := newHarness(t)
	if err := h.app.Secrets.Set("local", "pw"); err != nil {
		t.Fatal(err)
	}
	h.app.OpenUI = func(_ context.Context, ds *config.DataSource, _ string, _ *config.Config, opt UIOptions) error {
		name, workDir = ds.Name, opt.WorkDir
		return nil
	}

	code = h.app.Run(append([]string{"open"}, args...))
	return name, workDir, code
}

// The flag has to work in the position people actually type it. Go's flag
// package stops at the first positional argument, so "open local --dir X"
// would otherwise drop the directory without a word of complaint.
func TestDirIsAcceptedOnEitherSideOfTheDatasource(t *testing.T) {
	for _, args := range [][]string{
		{"local", "--dir", "/tmp/work"},
		{"--dir", "/tmp/work", "local"},
		{"local", "-dir", "/tmp/work"},
		{"--dir=/tmp/work", "local"},
	} {
		name, dir, code := openedWith(t, args...)
		if code != 0 {
			t.Errorf("dv open %s exited %d", strings.Join(args, " "), code)
		}
		if name != "local" {
			t.Errorf("dv open %s opened %q, want local", strings.Join(args, " "), name)
		}
		if dir != "/tmp/work" {
			t.Errorf("dv open %s passed %q, want /tmp/work", strings.Join(args, " "), dir)
		}
	}
}

// Opening without a directory is the ordinary case and must stay unchanged.
func TestOpeningWithoutADirectoryAttachesNothing(t *testing.T) {
	name, dir, code := openedWith(t, "local")
	if code != 0 || name != "local" {
		t.Fatalf("dv open local = %d, opened %q", code, name)
	}
	if dir != "" {
		t.Errorf("a session with no --dir was given %q", dir)
	}
}

// A second datasource name is a mistake, not a second directory to attach.
func TestASecondNameIsRejectedRatherThanIgnored(t *testing.T) {
	h := newHarness(t)
	h.app.OpenUI = func(context.Context, *config.DataSource, string, *config.Config, UIOptions) error {
		t.Error("the interface was opened despite a usage error")
		return nil
	}

	if code := h.app.Run([]string{"open", "local", "prod-app"}); code == 0 {
		t.Error("two datasource names were accepted")
	}
}

// The usage text is where anyone will look for this, so it has to name it.
func TestUsageMentionsTheDirectoryFlag(t *testing.T) {
	h := newHarness(t)
	h.app.Run([]string{"help"})

	if !strings.Contains(h.err.String(), "--dir") {
		t.Errorf("usage does not mention --dir:\n%s", h.err)
	}
}
