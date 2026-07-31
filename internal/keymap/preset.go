package keymap

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Preset is a named starting point for the key map.
//
// Muscle memory is the whole reason this exists: someone who spends the day
// in vim and someone who spends it in DataGrip both want their fingers to
// keep working, and neither wants to write out a keymap section to get it.
type Preset string

const (
	// PresetVim gives the editor a modal input model. The application keys
	// are unchanged — only what happens inside the editor differs.
	PresetVim Preset = "vim"
	// PresetDataGrip is DataGrip's SQL-tool keyboard.
	PresetDataGrip Preset = "datagrip"
	// PresetVSCode swaps in VS Code's spelling of the few keys where the two
	// tools genuinely disagree.
	PresetVSCode Preset = "vscode"
)

// DefaultPreset is what a user who has said nothing gets.
//
// It is the modal one. That is a strong default, so the interface says which
// mode it is in at all times, the empty editor says how to start typing, and
// the help screen says how to leave — see the vim reference and the escape
// hatch there.
const DefaultPreset = PresetVim

// Presets lists the presets, most preferred first.
func Presets() []Preset {
	return []Preset{PresetVim, PresetDataGrip, PresetVSCode}
}

// ParsePreset resolves a configured name.
func ParsePreset(name string) (Preset, error) {
	p := Preset(strings.ToLower(strings.TrimSpace(name)))
	for _, known := range Presets() {
		if p == known {
			return p, nil
		}
	}
	return "", unknownPreset(name)
}

func unknownPreset(name string) error {
	names := make([]string, 0, len(Presets()))
	for _, p := range Presets() {
		names = append(names, string(p))
	}
	return fmt.Errorf("unknown keymap preset %q; valid presets are: %s",
		name, strings.Join(names, ", "))
}

// ForPreset builds the key map for a preset.
//
// Every preset starts from the same base and then rebinds what it disagrees
// with. Sharing the base is what keeps the presets from drifting apart on the
// hundred keys nobody has an opinion about.
func ForPreset(p Preset) (*Map, error) {
	m := baseMap()

	switch p {
	case PresetDataGrip, PresetVim:
		// The base is DataGrip's. Vim differs in the editor's input model,
		// which is the vim package's business, not a binding table's.

	case PresetVSCode:
		applyVSCode(m)

	default:
		return nil, unknownPreset(string(p))
	}

	m.preset = p
	return m, nil
}

// applyVSCode rebinds the keys where VS Code and DataGrip disagree.
//
// Only the disagreements are listed. Anything absent here is a key the two
// tools already spell the same way, and duplicating it would be an
// opportunity for the two lists to fall out of step.
func applyVSCode(m *Map) {
	rebind := func(a Action, bindings ...Binding) {
		m.clear(a)
		m.bind(a, bindings...)
	}

	// ⌘Y in VS Code is redo, so line deletion moves to ⌘⇧K.
	rebind(ActionDeleteLine, ctrlAndCmdRune('k', tcell.ModShift)...)

	// ⌘D in VS Code selects the next occurrence, so duplication moves to
	// ⇧⌥↓. Ctrl is offered alongside Alt for terminals that cannot report a
	// modified arrow under Option.
	rebind(ActionDuplicateLine, altAndCtrl(tcell.KeyDown, tcell.ModShift)...)

	// F3 is carried over deliberately. rebind clears an action's bindings
	// before setting the new ones, and dropping it here would leave this one
	// preset with a palette that a host application claiming ⌘⇧P can lock —
	// and the palette is how every keyless command is reached.
	rebind(ActionCommandPalette,
		append(ctrlAndCmdRune('p', tcell.ModShift),
			Binding{Key: tcell.KeyF3})...)
	rebind(ActionGoToTable, ctrlAndCmdRune('p', 0)...)
}

// Preset reports which preset the map was built from.
func (m *Map) Preset() Preset { return m.preset }

// Modal reports whether the editor uses a modal input model.
//
// This is the one thing a preset changes that a binding table cannot express,
// so it is carried on the map rather than inferred by comparing keys.
func (m *Map) Modal() bool { return m.preset == PresetVim }

// FromConfig builds the effective key map from configuration.
//
// An empty preset name means the default. Overrides are applied on top, so a
// user can take a whole keyboard and still disagree with it about one key.
//
// It takes plain values rather than a config type so that the dependency runs
// one way: configuration carries strings, this package decides what they mean.
func FromConfig(preset string, overrides map[string][]string) (*Map, error) {
	p := DefaultPreset
	if strings.TrimSpace(preset) != "" {
		parsed, err := ParsePreset(preset)
		if err != nil {
			return nil, err
		}
		p = parsed
	}

	m, err := ForPreset(p)
	if err != nil {
		return nil, err
	}
	if err := m.Apply(overrides); err != nil {
		return nil, err
	}
	return m, nil
}
