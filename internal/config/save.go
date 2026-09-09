package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Empty is the configuration of a machine that has none: no datasources,
// the defaults in force.
func Empty() *Config {
	c := &Config{}
	c.applyDefaults()
	return c
}

// Write serialises c as YAML.
func Write(w io.Writer, c *Config) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encoding the configuration: %w", err)
	}
	return enc.Close()
}

// Save replaces the file at path with c.
//
// The write goes to a temporary file beside it and is renamed into place:
// a crash or a full disk midway leaves the previous file intact rather than
// a truncated one that the next run cannot parse. Owner-only, because the
// file names hosts and accounts inside someone's network.
func Save(path string, c *Config) error {
	if err := c.validate(); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := Write(tmp, c); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
