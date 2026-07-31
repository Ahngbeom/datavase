// Package config loads and validates datavase's YAML configuration.
//
// Parsing is deliberately split from file access: Parse works on any
// io.Reader so the whole validation surface can be tested without touching
// the filesystem.
package config

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Env labels how dangerous a datasource is. The guard package keys its
// policy off this value, so an unrecognised label must never silently
// degrade into a permissive one.
type Env string

const (
	EnvProd  Env = "prod"
	EnvStage Env = "stage"
	EnvDev   Env = "dev"
)

// TLSMode says how much the connection has to prove about the server before
// a credential is sent over it. The names match MySQL's own ssl-mode so that
// what is configured here can be checked against the server.
type TLSMode string

const (
	// TLSDisabled sends everything in clear text.
	TLSDisabled TLSMode = "disabled"
	// TLSPreferred encrypts when the server offers it and silently does not
	// when it does not, which is why it is not enough for production.
	TLSPreferred TLSMode = "preferred"
	// TLSRequired encrypts or fails, but proves nothing about who answered.
	TLSRequired TLSMode = "required"
	// TLSVerifyCA additionally requires the certificate to chain to a trusted
	// root, without requiring the name on it to match the address dialled.
	TLSVerifyCA TLSMode = "verify-ca"
	// TLSVerifyIdentity additionally requires that name to match.
	TLSVerifyIdentity TLSMode = "verify-identity"
)

// verifies reports whether the mode checks the server's certificate at all,
// which is what decides whether naming a certificate authority means anything.
func (m TLSMode) verifies() bool {
	return m == TLSVerifyCA || m == TLSVerifyIdentity
}

// DefaultTLSMode is what an absent "tls:" means.
//
// It follows env for the same reason the guard does. Production is where a
// credential crossing the wire in clear text costs the most, and it is also
// where the managed databases that refuse plain connections outright live, so
// "required" is both the safer default and usually the working one. Anywhere
// else the cost of being wrong is a connection that will not open on a
// developer's laptop, which is why those get "preferred".
func DefaultTLSMode(env Env) TLSMode {
	if env == EnvProd {
		return TLSRequired
	}
	return TLSPreferred
}

// Tunnel describes an SSH bastion to reach a datasource through.
type Tunnel struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Identity string `yaml:"identity"`
}

// DataSource is a single MySQL/MariaDB target. Passwords are never stored
// here; they live in the OS keychain keyed by Name.
type DataSource struct {
	Name     string  `yaml:"name"`
	Env      Env     `yaml:"env"`
	Host     string  `yaml:"host"`
	Port     int     `yaml:"port"`
	User     string  `yaml:"user"`
	Database string  `yaml:"database"`
	Tunnel   *Tunnel `yaml:"tunnel"`

	// TLS is how much the connection must prove about the server. Empty means
	// DefaultTLSMode for this datasource's env.
	TLS TLSMode `yaml:"tls"`
	// TLSCA is a PEM file of roots to verify against instead of the system
	// store, for an instance behind a private certificate authority. It is
	// only meaningful under a mode that verifies, and is refused under any
	// other rather than read and ignored.
	TLSCA string `yaml:"tls_ca"`
}

// Defaults holds tunables shared by every datasource.
type Defaults struct {
	AutoLimit  int `yaml:"auto_limit"`
	FetchChunk int `yaml:"fetch_chunk"`
	BufferMax  int `yaml:"buffer_max"`
}

// Config is the root of the configuration file.
type Config struct {
	DataSources []DataSource `yaml:"datasources"`
	Defaults    Defaults     `yaml:"defaults"`

	// Keymap chooses the keyboard preset and overrides individual bindings.
	// See the Keymap type for the accepted forms.
	Keymap Keymap `yaml:"keymap"`
}

// Default values applied when the corresponding key is absent.
const (
	DefaultPort       = 3306
	DefaultTunnelPort = 22
	DefaultAutoLimit  = 1000
	DefaultFetchChunk = 500
	DefaultBufferMax  = 50000
)

// Parse reads and validates YAML configuration from r.
//
// Unknown keys are rejected rather than ignored: a typo such as "hots"
// would otherwise surface much later as a confusing "host is required".
func Parse(r io.Reader) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Defaults.AutoLimit == 0 {
		c.Defaults.AutoLimit = DefaultAutoLimit
	}
	if c.Defaults.FetchChunk == 0 {
		c.Defaults.FetchChunk = DefaultFetchChunk
	}
	if c.Defaults.BufferMax == 0 {
		c.Defaults.BufferMax = DefaultBufferMax
	}
	for i := range c.DataSources {
		ds := &c.DataSources[i]
		if ds.Port == 0 {
			ds.Port = DefaultPort
		}
		if ds.Tunnel != nil && ds.Tunnel.Port == 0 {
			ds.Tunnel.Port = DefaultTunnelPort
		}
		if ds.TLS == "" {
			ds.TLS = DefaultTLSMode(ds.Env)
		}
	}
}

func (c *Config) validate() error {
	if len(c.DataSources) == 0 {
		return errors.New("no datasources defined")
	}

	seen := make(map[string]struct{}, len(c.DataSources))
	for i := range c.DataSources {
		ds := &c.DataSources[i]
		if err := ds.validate(i); err != nil {
			return err
		}
		if _, dup := seen[ds.Name]; dup {
			return fmt.Errorf("duplicate datasource name %q", ds.Name)
		}
		seen[ds.Name] = struct{}{}
	}
	return nil
}

func (d *DataSource) validate(index int) error {
	if d.Name == "" {
		return fmt.Errorf("datasource #%d: name is required", index)
	}
	switch d.Env {
	case EnvProd, EnvStage, EnvDev:
	default:
		return fmt.Errorf("datasource %q: env must be one of %q, %q, %q (got %q)",
			d.Name, EnvProd, EnvStage, EnvDev, d.Env)
	}
	if d.Host == "" {
		return fmt.Errorf("datasource %q: host is required", d.Name)
	}
	if d.User == "" {
		return fmt.Errorf("datasource %q: user is required", d.Name)
	}
	switch d.TLS {
	case TLSDisabled, TLSPreferred, TLSRequired, TLSVerifyCA, TLSVerifyIdentity:
	default:
		return fmt.Errorf("datasource %q: tls must be one of %q, %q, %q, %q, %q (got %q)",
			d.Name, TLSDisabled, TLSPreferred, TLSRequired, TLSVerifyCA, TLSVerifyIdentity, d.TLS)
	}
	// A CA named under a mode that checks no certificate would be read and
	// then ignored, leaving the operator believing the server was verified
	// against it. Refusing is the same bargain the unknown-key rule makes.
	if d.TLSCA != "" && !d.TLS.verifies() {
		return fmt.Errorf("datasource %q: tls_ca needs tls: %q or %q; %q verifies no certificate",
			d.Name, TLSVerifyCA, TLSVerifyIdentity, d.TLS)
	}
	if d.Tunnel != nil {
		if err := d.Tunnel.validate(d.Name); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tunnel) validate(dsName string) error {
	if t.Host == "" {
		return fmt.Errorf("datasource %q: tunnel host is required", dsName)
	}
	if t.User == "" {
		return fmt.Errorf("datasource %q: tunnel user is required", dsName)
	}
	return nil
}
