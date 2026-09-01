package keymap

import (
	"strings"
	"testing"
)

// bound reports whether the map has the spec bound to the action. Specs are
// written the way a user would write them in configuration, so the test also
// covers that the two spellings agree.
func bound(t *testing.T, m *Map, a Action, spec string) bool {
	t.Helper()

	want, err := ParseBinding(spec)
	if err != nil {
		t.Fatalf("ParseBinding(%q) error = %v", spec, err)
	}
	for _, b := range m.Bindings(a) {
		if b == want {
			return true
		}
	}
	return false
}

func TestPresetsListsEveryPreset(t *testing.T) {
	got := Presets()

	for _, want := range []Preset{PresetVim, PresetDataGrip, PresetVSCode} {
		var found bool
		for _, p := range got {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Presets() = %v, want it to include %q", got, want)
		}
	}
}

// Only vim changes the editor's input model; the other two are plain typing
// with different chords on top.
func TestOnlyTheVimPresetIsModal(t *testing.T) {
	tests := []struct {
		preset Preset
		modal  bool
	}{
		{PresetVim, true},
		{PresetDataGrip, false},
		{PresetVSCode, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			m, err := ForPreset(tt.preset)
			if err != nil {
				t.Fatalf("ForPreset(%q) error = %v", tt.preset, err)
			}
			if got := m.Modal(); got != tt.modal {
				t.Errorf("Modal() = %v, want %v", got, tt.modal)
			}
			if got := m.Preset(); got != tt.preset {
				t.Errorf("Preset() = %q, want %q", got, tt.preset)
			}
		})
	}
}

// The point of offering a VS Code preset is the handful of keys where the two
// tools genuinely disagree; everything else is shared.
func TestVSCodePresetUsesVSCodeKeysWhereTheyDiffer(t *testing.T) {
	code, err := ForPreset(PresetVSCode)
	if err != nil {
		t.Fatalf("ForPreset(vscode) error = %v", err)
	}
	grip, err := ForPreset(PresetDataGrip)
	if err != nil {
		t.Fatalf("ForPreset(datagrip) error = %v", err)
	}

	tests := []struct {
		action   Action
		vscode   string
		datagrip string
	}{
		{ActionDeleteLine, "cmd+shift+k", "cmd+y"},
		{ActionCommandPalette, "cmd+shift+p", "cmd+shift+a"},
		{ActionGoToTable, "cmd+p", "cmd+n"},
	}

	for _, tt := range tests {
		t.Run(tt.action.String(), func(t *testing.T) {
			if !bound(t, code, tt.action, tt.vscode) {
				t.Errorf("vscode: %s is not bound to %s", tt.action.String(), tt.vscode)
			}
			if bound(t, code, tt.action, tt.datagrip) {
				t.Errorf("vscode: %s is still bound to DataGrip's %s", tt.action.String(), tt.datagrip)
			}
			if !bound(t, grip, tt.action, tt.datagrip) {
				t.Errorf("datagrip: %s is not bound to %s", tt.action.String(), tt.datagrip)
			}
		})
	}
}

// Where the two tools agree, the preset must not quietly drift.
func TestPresetsShareTheKeysTheToolsAgreeOn(t *testing.T) {
	code, _ := ForPreset(PresetVSCode)
	grip, _ := ForPreset(PresetDataGrip)

	for _, action := range []Action{
		ActionRun, ActionCancel, ActionFind, ActionComplete,
		ActionSelectAll, ActionToggleComment, ActionHelp, ActionQuit,
	} {
		a, b := code.Bindings(action), grip.Bindings(action)
		if len(a) != len(b) {
			t.Errorf("%s: vscode has %d bindings, datagrip has %d", action.String(), len(a), len(b))
			continue
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("%s: vscode has %v, datagrip has %v", action.String(), a[i], b[i])
			}
		}
	}
}

// The vim preset changes the editor, not the application. Reaching the help
// screen or cancelling a query must not require leaving insert mode.
func TestVimPresetKeepsTheApplicationKeys(t *testing.T) {
	vim, err := ForPreset(PresetVim)
	if err != nil {
		t.Fatalf("ForPreset(vim) error = %v", err)
	}

	for _, action := range []Action{ActionRun, ActionCancel, ActionHelp, ActionQuit, ActionGoToTable} {
		if len(vim.Bindings(action)) == 0 {
			t.Errorf("%s has no binding in the vim preset", action.String())
		}
	}
}

func TestForPresetRejectsAnUnknownName(t *testing.T) {
	_, err := ForPreset("emacs")
	if err == nil {
		t.Fatal("ForPreset(\"emacs\") error = nil, want an error")
	}
	// The message has to say what is allowed, or the user is left guessing.
	for _, want := range []string{"emacs", "vim", "datagrip", "vscode"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestDefaultIsAPreset(t *testing.T) {
	if Default().Preset() == "" {
		t.Error("Default() has no preset")
	}
}

// FromConfig is the one place a configuration file becomes a key map, so it
// is where both halves — the preset and the per-action overrides — have to
// meet correctly.
func TestFromConfigAppliesOverridesOnTopOfThePreset(t *testing.T) {
	m, err := FromConfig(string(PresetVSCode), map[string][]string{"run": {"f8"}})
	if err != nil {
		t.Fatalf("FromConfig() error = %v, want nil", err)
	}

	if m.Preset() != PresetVSCode {
		t.Errorf("Preset() = %q, want %q", m.Preset(), PresetVSCode)
	}
	if !bound(t, m, ActionRun, "f8") {
		t.Error("the override was not applied")
	}
	// The override replaces rather than adds, so the preset's own key is gone.
	if bound(t, m, ActionRun, "f5") {
		t.Error("the preset's binding survived an override")
	}
	// An action the user said nothing about keeps the preset's key.
	if !bound(t, m, ActionDeleteLine, "cmd+shift+k") {
		t.Error("the preset was lost when overrides were applied")
	}
}

func TestFromConfigWithNoPresetUsesTheDefault(t *testing.T) {
	m, err := FromConfig("", nil)
	if err != nil {
		t.Fatalf("FromConfig() error = %v, want nil", err)
	}
	if m.Preset() != DefaultPreset {
		t.Errorf("Preset() = %q, want %q", m.Preset(), DefaultPreset)
	}
}

// Both halves have to fail loudly: a keyboard that is half what was asked for
// is worse than one that refuses to load.
func TestFromConfigRejectsBadInput(t *testing.T) {
	if _, err := FromConfig("emacs", nil); err == nil {
		t.Error("FromConfig() error = nil, want an error for an unknown preset")
	}
	if _, err := FromConfig("", map[string][]string{"runn": {"f8"}}); err == nil {
		t.Error("FromConfig() error = nil, want an error for an unknown action")
	}
}

// Default() is what `dv keys` prints for a session with no configuration; if
// it drifted from DefaultPreset, that reference would show a keyboard the
// session does not actually run.
func TestTheDefaultMapIsTheDefaultPreset(t *testing.T) {
	m := Default()

	if got := m.Preset(); got != DefaultPreset {
		t.Errorf("Default().Preset() = %q, want %q", got, DefaultPreset)
	}
	if got, want := m.Modal(), DefaultPreset == PresetVim; got != want {
		t.Errorf("Default().Modal() = %v, want %v", got, want)
	}
}

// Someone who has written no configuration arrives from a GUI SQL client and
// expects typing to type. A modal editor as the unasked-for default is the
// first thing that sends them back to DataGrip.
func TestSayingNothingGivesAnEditorThatTypes(t *testing.T) {
	m, err := FromConfig("", nil)
	if err != nil {
		t.Fatalf("FromConfig(\"\", nil) error = %v, want nil", err)
	}

	if got := m.Preset(); got != PresetDataGrip {
		t.Errorf("Preset() = %q, want %q", got, PresetDataGrip)
	}
	if m.Modal() {
		t.Error("the default editor is modal; typing does nothing until i is pressed")
	}
}
