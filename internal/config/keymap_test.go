package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

// Changing what an absent preset means changes the keyboard under anyone who
// hand-wrote a config without one. The interface can only say so if the
// parser keeps "absent" and "written out as empty" apart.
func TestAnAbsentPresetIsDistinguishableFromAnEmptyOne(t *testing.T) {
	var absent Keymap
	if err := yaml.Unmarshal([]byte("actions:\n  run: [\"f5\"]\n"), &absent); err != nil {
		t.Fatalf("unmarshalling a keymap with no preset: %v", err)
	}
	if absent.PresetSet {
		t.Error("a keymap section with no preset reports that one was set")
	}

	var stated Keymap
	if err := yaml.Unmarshal([]byte("preset: \"\"\n"), &stated); err != nil {
		t.Fatalf("unmarshalling an empty preset: %v", err)
	}
	if !stated.PresetSet {
		t.Error("a preset written out as empty reports that none was set")
	}
}

// The bare action-to-keys form predates presets, so it can never have named
// one — and someone using it is exactly who a changed default would surprise.
func TestTheOlderKeymapFormNamesNoPreset(t *testing.T) {
	var k Keymap
	if err := yaml.Unmarshal([]byte("run: [\"ctrl+enter\", \"f5\"]\n"), &k); err != nil {
		t.Fatalf("unmarshalling the older form: %v", err)
	}
	if k.PresetSet {
		t.Error("the older form reports that it named a preset")
	}
}

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
