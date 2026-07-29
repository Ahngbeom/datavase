package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `
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

func TestLoadReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validYAML), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(cfg.DataSources) != 2 {
		t.Fatalf("len(DataSources) = %d, want 2", len(cfg.DataSources))
	}
}

// The error must name the path; "no such file or directory" alone leaves the
// user guessing which file datavase was looking for.
func TestLoadMissingFileMentionsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.yaml")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("Load() error = %q, want it to contain the path %q", err, path)
	}
}

func TestFindDataSource(t *testing.T) {
	cfg, err := Parse(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	ds, err := cfg.Find("prod-app")
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if ds.Env != EnvProd {
		t.Errorf("Env = %q, want %q", ds.Env, EnvProd)
	}
}

// An unknown name should list what is available; the usual cause is a typo.
func TestFindUnknownDataSourceListsKnownNames(t *testing.T) {
	cfg, err := Parse(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	_, err = cfg.Find("prod-ap")
	if err == nil {
		t.Fatal("Find() error = nil, want an error")
	}
	for _, want := range []string{"prod-ap", "local", "prod-app"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Find() error = %q, want it to contain %q", err, want)
		}
	}
}

func TestDefaultPathHonoursXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v, want nil", err)
	}
	want := filepath.Join("/tmp/xdg", "datavase", "config.yaml")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v, want nil", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	want := filepath.Join(home, ".config", "datavase", "config.yaml")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}
