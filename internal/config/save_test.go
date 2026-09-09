package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sample() *Config {
	c := &Config{DataSources: []DataSource{
		{Name: "local", Host: "127.0.0.1", User: "root", Database: "app"},
		{Name: "bastion", Host: "db.internal", User: "ro", TLS: TLSRequired,
			Tunnel: &Tunnel{Host: "jump.example.com", User: "me"}},
	}}
	c.applyDefaults()
	return c
}

func TestWriteThenParseRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	back, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse(Write()) error = %v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(back.DataSources, sample().DataSources) {
		t.Errorf("round trip changed the datasources:\n got %+v\nwant %+v", back.DataSources, sample().DataSources)
	}
}

func TestWriteLeavesOutWhatWasNeverSet(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample()); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"env:", "tls_ca:", "identity:", "keymap:", "mouse:"} {
		if strings.Contains(buf.String(), absent) {
			t.Errorf("Write() contains %q for a field that was never set:\n%s", absent, buf.String())
		}
	}
	if strings.Count(buf.String(), "tunnel:") != 1 {
		t.Errorf("Write() must write the tunnel once, for the datasource that has one:\n%s", buf.String())
	}
}

func TestSaveWritesAnOwnerOnlyFileTheLoaderReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := Save(path, sample()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600: the file names hosts and accounts", info.Mode().Perm())
	}
	if _, err := Load(path); err != nil {
		t.Errorf("Load(Save()) error = %v", err)
	}
}

func TestSaveRefusesAnInvalidConfigAndKeepsTheOldFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, sample()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	bad := sample()
	bad.DataSources[1].Name = "local" // duplicate
	if err := Save(path, bad); err == nil {
		t.Fatal("Save() = nil, want an error for a duplicate name")
	}

	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("a refused save changed the file")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only config.yaml; a temp file was left behind", len(entries))
	}
}

func TestAConfigWithNoDatasourcesIsValid(t *testing.T) {
	cfg, err := Parse(strings.NewReader("datasources: []\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v; an empty list is where a first run starts", err)
	}
	if len(cfg.DataSources) != 0 || cfg.Defaults.AutoLimit != DefaultAutoLimit {
		t.Errorf("cfg = %+v", cfg)
	}
	if got := Empty(); got.Defaults.FetchChunk != DefaultFetchChunk {
		t.Errorf("Empty() lacks defaults: %+v", got.Defaults)
	}
}
