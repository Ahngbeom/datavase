package cli

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

func TestKeysListsActionsAndTheirBindings(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"keys"}); code != 0 {
		t.Fatalf("Run(keys) = %d, want 0; stderr = %q", code, h.err)
	}

	out := h.out.String()
	for _, want := range []string{
		"run the statement under the cursor",
		"cancel the running statement",
		"comment or uncomment the selected lines",
		"F5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("keys output is missing %q:\n%s", want, out)
		}
	}
}

// Nothing is marked unbuilt while every action works; the note would be a
// lie, and a table that lies is worse than no table.
func TestKeysDoesNotMarkImplementedActions(t *testing.T) {
	h := newHarness(t)
	h.app.Run([]string{"keys"})

	out := h.out.String()
	if strings.Contains(out, "not built yet") {
		t.Errorf("an implemented action is marked unbuilt:\n%s", out)
	}
	// And the table is complete.
	for _, want := range []string{"find in the editor", "jump to a table", "open the command palette"} {
		if !strings.Contains(out, want) {
			t.Errorf("the table is missing %q:\n%s", want, out)
		}
	}
}

func TestKeysGhosttySnippetIsPasteable(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"keys", "--ghostty"}); code != 0 {
		t.Fatalf("Run(keys --ghostty) = %d, want 0; stderr = %q", code, h.err)
	}

	out := h.out.String()
	if !strings.Contains(out, `keybind = cmd+enter=text:\x1b[13;9u`) {
		t.Errorf("ghostty output does not carry the ⌘↩ binding:\n%s", out)
	}
	// Comments are fine, but every non-comment line has to be a keybind.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, "keybind = ") {
			t.Errorf("non-keybind line in ghostty output: %q", line)
		}
	}
}

func TestKeysTmuxSnippet(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"keys", "--tmux"}); code != 0 {
		t.Fatalf("Run(keys --tmux) = %d, want 0", code)
	}
	if !strings.Contains(h.out.String(), "extended-keys on") {
		t.Errorf("tmux output is missing the extended-keys setting:\n%s", h.out)
	}
}

func TestKeysIterm2Advice(t *testing.T) {
	h := newHarness(t)

	if code := h.app.Run([]string{"keys", "--iterm2"}); code != 0 {
		t.Fatalf("Run(keys --iterm2) = %d, want 0", code)
	}
	if !strings.Contains(h.out.String(), "CSI u") {
		t.Errorf("iTerm2 output does not name the setting:\n%s", h.out)
	}
}

// A custom binding has to show up in the table, or the table is a lie.
func TestKeysReflectsConfiguredOverrides(t *testing.T) {
	h := newHarness(t)
	h.app.Config.Keymap = config.Keymap{Actions: map[string][]string{"run": {"f8"}}}

	if code := h.app.Run([]string{"keys"}); code != 0 {
		t.Fatalf("Run(keys) = %d, want 0; stderr = %q", code, h.err)
	}

	out := h.out.String()
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "run the statement under the cursor") {
			continue
		}
		if !strings.Contains(line, "F8") {
			t.Errorf("the overridden binding is not shown: %q", line)
		}
		if strings.Contains(line, "F5") {
			t.Errorf("the replaced default is still shown: %q", line)
		}
		return
	}
	t.Errorf("the run action was not listed:\n%s", out)
}

// A bad keymap must fail loudly here rather than leaving the user with keys
// that quietly do nothing.
func TestKeysRejectsAnInvalidKeymap(t *testing.T) {
	h := newHarness(t)
	h.app.Config.Keymap = config.Keymap{Actions: map[string][]string{"runn": {"f8"}}}

	if code := h.app.Run([]string{"keys"}); code == 0 {
		t.Fatal("Run(keys) = 0, want a non-zero exit code")
	}
	if !strings.Contains(h.err.String(), "runn") {
		t.Errorf("stderr does not name the bad action:\n%s", h.err)
	}
}

// The key reference is what a user consults when a chord does not work, so it
// has to show the keyboard they actually configured, not the built-in one.
func TestKeysReflectsTheConfiguredPreset(t *testing.T) {
	h := newHarness(t)
	h.app.Config.Keymap = config.Keymap{Preset: "vscode"}

	if code := h.app.Run([]string{"keys"}); code != 0 {
		t.Fatalf("Run(keys) = %d, want 0; stderr = %q", code, h.err)
	}

	// VS Code deletes a line with ⌘⇧K; DataGrip uses ⌘Y.
	out := h.out.String()
	if !strings.Contains(out, "⇧K") {
		t.Errorf("the VS Code line-delete key is missing:\n%s", out)
	}
}

func TestKeysRejectsAnUnknownPreset(t *testing.T) {
	h := newHarness(t)
	h.app.Config.Keymap = config.Keymap{Preset: "emacs"}

	if code := h.app.Run([]string{"keys"}); code == 0 {
		t.Error("Run(keys) = 0, want a failure for an unknown preset")
	}
	if !strings.Contains(h.err.String(), "emacs") {
		t.Errorf("stderr = %q, want it to name the bad preset", h.err)
	}
}

// `dv keys` is what someone runs from outside the application when they
// cannot work out what a key does — including, on the vim keyboard, the vim
// keys themselves.
func TestKeysListsTheVimCommandsOnTheVimPreset(t *testing.T) {
	h := newHarness(t)
	h.app.Config.Keymap = config.Keymap{Preset: "vim"}

	if code := h.app.Run([]string{"keys"}); code != 0 {
		t.Fatalf("Run(keys) = %d, want 0; stderr = %q", code, h.err)
	}

	out := h.out.String()
	for _, want := range []string{"dd", "cw", "gg", "insert before the cursor"} {
		if !strings.Contains(out, want) {
			t.Errorf("the vim reference is missing %q:\n%s", want, out)
		}
	}
}

// And it must not appear on the keyboards that are not modal, where "dd" is
// just two letters.
func TestKeysOmitsTheVimCommandsElsewhere(t *testing.T) {
	h := newHarness(t)
	h.app.Config.Keymap = config.Keymap{Preset: "datagrip"}

	h.app.Run([]string{"keys"})

	if strings.Contains(h.out.String(), "insert before the cursor") {
		t.Errorf("the vim reference is shown on a non-modal keyboard:\n%s", h.out)
	}
}
