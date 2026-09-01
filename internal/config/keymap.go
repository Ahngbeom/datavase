package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Keymap is the keyboard section of the configuration.
//
// It carries values only. Preset names and binding syntax are the keymap
// package's to validate, which is what keeps configuration free of any
// knowledge of what a key does.
type Keymap struct {
	// Preset names the keyboard to start from; empty means the default.
	Preset string `yaml:"preset"`
	// PresetSet says the configuration named a preset at all.
	//
	// The default changed once. Anyone who wrote a config without this key
	// would otherwise have had their keyboard swapped with no announcement,
	// which is the one thing a changed default must not do.
	PresetSet bool `yaml:"-"`
	// Actions overrides individual bindings by action name.
	Actions map[string][]string `yaml:"actions"`
}

// UnmarshalYAML reads both spellings of the section.
//
// The section began life as a bare action-to-keys mapping:
//
//	keymap:
//	  run: ["ctrl+enter", "f5"]
//
// and gained a preset later:
//
//	keymap:
//	  preset: vim
//	  actions:
//	    run: ["f5"]
//
// Both are accepted. Rejecting the older form would silently stop applying
// the keys of anyone already using it, which is worse than failing to load —
// they would find out by pressing a key and getting nothing.
func (k *Keymap) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: keymap must be a mapping", node.Line)
	}

	// Naming either structured key opts into the structured form. No action
	// is called "preset" or "actions", so this cannot misread the old form.
	structured := false
	for i := 0; i < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "preset", "actions":
			structured = true
		}
	}
	if !structured {
		return node.Decode(&k.Actions)
	}

	// In the structured form an unrecognised key is a typo, not an action.
	// The rest of the file rejects unknown keys, and this section should not
	// be the one place a misspelling is quietly ignored.
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]

		switch key.Value {
		case "preset":
			if err := value.Decode(&k.Preset); err != nil {
				return err
			}
			k.PresetSet = true
		case "actions":
			if err := value.Decode(&k.Actions); err != nil {
				return err
			}
		default:
			return fmt.Errorf("line %d: unknown key %q in keymap; expected \"preset\" or \"actions\"",
				key.Line, key.Value)
		}
	}
	return nil
}
