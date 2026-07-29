package config

import (
	"strings"
	"testing"
)

// The keymap section used to be a bare action-to-keys mapping. Anyone already
// using that form must keep working — a configuration file that silently
// stops applying is worse than one that fails to load.
func TestKeymapAcceptsTheOldMapForm(t *testing.T) {
	cfg := mustParseKeymap(t, `
keymap:
  run: ["f8"]
  cancel: ["f9", "ctrl+c"]
`)

	if cfg.Keymap.Preset != "" {
		t.Errorf("Preset = %q, want it unset", cfg.Keymap.Preset)
	}
	assertActions(t, cfg.Keymap.Actions, map[string][]string{
		"run":    {"f8"},
		"cancel": {"f9", "ctrl+c"},
	})
}

func TestKeymapAcceptsThePresetForm(t *testing.T) {
	cfg := mustParseKeymap(t, `
keymap:
  preset: vim
  actions:
    run: ["f8"]
`)

	if cfg.Keymap.Preset != "vim" {
		t.Errorf("Preset = %q, want %q", cfg.Keymap.Preset, "vim")
	}
	assertActions(t, cfg.Keymap.Actions, map[string][]string{"run": {"f8"}})
}

func TestKeymapAcceptsAPresetWithNoOverrides(t *testing.T) {
	cfg := mustParseKeymap(t, `
keymap:
  preset: vscode
`)

	if cfg.Keymap.Preset != "vscode" {
		t.Errorf("Preset = %q, want %q", cfg.Keymap.Preset, "vscode")
	}
	if len(cfg.Keymap.Actions) != 0 {
		t.Errorf("Actions = %v, want none", cfg.Keymap.Actions)
	}
}

func TestKeymapWhenAbsent(t *testing.T) {
	cfg := mustParseKeymap(t, "")

	if cfg.Keymap.Preset != "" || len(cfg.Keymap.Actions) != 0 {
		t.Errorf("Keymap = %+v, want it empty", cfg.Keymap)
	}
}

// Once the section names a preset it is the structured form, so a stray key
// there is a typo rather than an action. Saying so beats applying nothing.
func TestKeymapRejectsAnUnknownKeyBesideAPreset(t *testing.T) {
	err := parseKeymapErr(t, `
keymap:
  preset: vim
  action:
    run: ["f8"]
`)
	if err == nil {
		t.Fatal("Parse() error = nil, want an error for the misspelled key")
	}
	for _, want := range []string{"action", "preset", "actions"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestKeymapRejectsANonMapping(t *testing.T) {
	if err := parseKeymapErr(t, "keymap: [run]\n"); err == nil {
		t.Fatal("Parse() error = nil, want an error for a sequence")
	}
}

// mustParseKeymap parses a document with the keymap section under test.
func mustParseKeymap(t *testing.T, keymapYAML string) *Config {
	t.Helper()

	cfg, err := Parse(strings.NewReader(minimalConfig + keymapYAML))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	return cfg
}

func parseKeymapErr(t *testing.T, keymapYAML string) error {
	t.Helper()

	_, err := Parse(strings.NewReader(minimalConfig + keymapYAML))
	return err
}

const minimalConfig = `
datasources:
  - name: local
    env: dev
    host: 127.0.0.1
    user: root
`

func assertActions(t *testing.T, got, want map[string][]string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("Actions = %v, want %v", got, want)
	}
	for name, keys := range want {
		gotKeys := got[name]
		if len(gotKeys) != len(keys) {
			t.Errorf("Actions[%q] = %v, want %v", name, gotKeys, keys)
			continue
		}
		for i := range keys {
			if gotKeys[i] != keys[i] {
				t.Errorf("Actions[%q] = %v, want %v", name, gotKeys, keys)
				break
			}
		}
	}
}
