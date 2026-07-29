package config

import (
	"strings"
	"testing"
)

func TestParseMinimalDataSource(t *testing.T) {
	const src = `
datasources:
  - name: local
    env: dev
    host: 127.0.0.1
    port: 3306
    user: root
    database: app_db
`

	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	if len(cfg.DataSources) != 1 {
		t.Fatalf("len(DataSources) = %d, want 1", len(cfg.DataSources))
	}

	got := cfg.DataSources[0]
	if got.Name != "local" {
		t.Errorf("Name = %q, want %q", got.Name, "local")
	}
	if got.Env != EnvDev {
		t.Errorf("Env = %q, want %q", got.Env, EnvDev)
	}
	if got.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", got.Host, "127.0.0.1")
	}
	if got.Port != 3306 {
		t.Errorf("Port = %d, want 3306", got.Port)
	}
	if got.User != "root" {
		t.Errorf("User = %q, want %q", got.User, "root")
	}
	if got.Database != "app_db" {
		t.Errorf("Database = %q, want %q", got.Database, "app_db")
	}
	if got.Tunnel != nil {
		t.Errorf("Tunnel = %+v, want nil", got.Tunnel)
	}
}

func TestParseAppliesDefaults(t *testing.T) {
	const src = `
datasources:
  - name: local
    env: dev
    host: 127.0.0.1
    user: root
    tunnel:
      host: bastion
      user: bahn
`

	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	ds := cfg.DataSources[0]
	if ds.Port != 3306 {
		t.Errorf("Port = %d, want 3306", ds.Port)
	}
	if ds.Tunnel.Port != 22 {
		t.Errorf("Tunnel.Port = %d, want 22", ds.Tunnel.Port)
	}
	if cfg.Defaults.AutoLimit != 1000 {
		t.Errorf("Defaults.AutoLimit = %d, want 1000", cfg.Defaults.AutoLimit)
	}
	if cfg.Defaults.FetchChunk != 500 {
		t.Errorf("Defaults.FetchChunk = %d, want 500", cfg.Defaults.FetchChunk)
	}
	if cfg.Defaults.BufferMax != 50000 {
		t.Errorf("Defaults.BufferMax = %d, want 50000", cfg.Defaults.BufferMax)
	}
}

func TestParseKeepsExplicitDefaults(t *testing.T) {
	const src = `
datasources:
  - name: local
    env: dev
    host: 127.0.0.1
    port: 3307
    user: root
defaults:
  auto_limit: 25
  fetch_chunk: 10
  buffer_max: 100
`

	cfg, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	if cfg.DataSources[0].Port != 3307 {
		t.Errorf("Port = %d, want 3307", cfg.DataSources[0].Port)
	}
	if cfg.Defaults.AutoLimit != 25 {
		t.Errorf("Defaults.AutoLimit = %d, want 25", cfg.Defaults.AutoLimit)
	}
	if cfg.Defaults.FetchChunk != 10 {
		t.Errorf("Defaults.FetchChunk = %d, want 10", cfg.Defaults.FetchChunk)
	}
	if cfg.Defaults.BufferMax != 100 {
		t.Errorf("Defaults.BufferMax = %d, want 100", cfg.Defaults.BufferMax)
	}
}

// A mistyped key must fail loudly. Silently ignoring "hots:" would leave
// Host empty and produce a misleading error far from the real mistake.
func TestParseRejectsUnknownFields(t *testing.T) {
	const src = `
datasources:
  - name: local
    env: dev
    hots: 127.0.0.1
    user: root
`

	_, err := Parse(strings.NewReader(src))
	if err == nil {
		t.Fatal("Parse() error = nil, want error about the unknown field")
	}
	if !strings.Contains(err.Error(), "hots") {
		t.Errorf("Parse() error = %q, want it to mention the unknown key %q", err, "hots")
	}
}

// A config that fails to parse is safe; a config that parses into the wrong
// env label is not. "production" must never be read as anything other than
// an error, because guard would otherwise treat it as non-prod.
func TestParseRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name: "unknown env label",
			src: `
datasources:
  - name: db
    env: production
    host: h
    user: u
`,
			wantErr: "env",
		},
		{
			name: "missing env label",
			src: `
datasources:
  - name: db
    host: h
    user: u
`,
			wantErr: "env",
		},
		{
			name: "missing name",
			src: `
datasources:
  - env: dev
    host: h
    user: u
`,
			wantErr: "name",
		},
		{
			name: "missing host",
			src: `
datasources:
  - name: db
    env: dev
    user: u
`,
			wantErr: "host",
		},
		{
			name: "missing user",
			src: `
datasources:
  - name: db
    env: dev
    host: h
`,
			wantErr: "user",
		},
		{
			name: "duplicate datasource name",
			src: `
datasources:
  - name: db
    env: dev
    host: h
    user: u
  - name: db
    env: prod
    host: h2
    user: u2
`,
			wantErr: "duplicate",
		},
		{
			name:    "no datasources",
			src:     "datasources: []\n",
			wantErr: "no datasources",
		},
		{
			name: "tunnel without host",
			src: `
datasources:
  - name: db
    env: dev
    host: h
    user: u
    tunnel:
      user: bastionuser
`,
			wantErr: "tunnel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.src))
			if err == nil {
				t.Fatalf("Parse() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Parse() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
