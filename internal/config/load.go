package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath returns the configuration file location, honouring
// XDG_CONFIG_HOME when it is set.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "datavase", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".config", "datavase", "config.yaml"), nil
}

// Load reads and validates the configuration file at path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config %s: %w", path, err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return cfg, nil
}

// Find returns the datasource with the given name.
func (c *Config) Find(name string) (*DataSource, error) {
	for i := range c.DataSources {
		if c.DataSources[i].Name == name {
			return &c.DataSources[i], nil
		}
	}
	return nil, fmt.Errorf("unknown datasource %q; configured: %s",
		name, strings.Join(c.Names(), ", "))
}

// Names lists the configured datasource names in file order.
func (c *Config) Names() []string {
	names := make([]string, len(c.DataSources))
	for i := range c.DataSources {
		names[i] = c.DataSources[i].Name
	}
	return names
}
