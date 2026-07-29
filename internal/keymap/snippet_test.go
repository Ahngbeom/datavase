package keymap

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// This is the check that explains why Ctrl+Enter works in one terminal and
// not another, so it has to match tcell's own rule.
func TestSupportsExtendedKeys(t *testing.T) {
	tests := []struct {
		term string
		want bool
	}{
		{term: "xterm-256color", want: true},
		{term: "xterm-ghostty", want: true},
		{term: "xterm-kitty", want: true},
		{term: "tmux-256color", want: true},
		// tmux's default TERM, and the reason Ctrl+Enter appears broken
		// inside an unconfigured tmux.
		{term: "screen-256color", want: false},
		{term: "screen", want: false},
		{term: "vt100", want: false},
		{term: "dumb", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			if got := SupportsExtendedKeys(tt.term); got != tt.want {
				t.Errorf("SupportsExtendedKeys(%q) = %v, want %v", tt.term, got, tt.want)
			}
		})
	}
}

func TestTerminalAdvice(t *testing.T) {
	m := Default()

	if got := TerminalAdvice("xterm-256color", m); got != "" {
		t.Errorf("TerminalAdvice(xterm) = %q, want no advice", got)
	}

	got := TerminalAdvice("screen-256color", m)
	if got == "" {
		t.Fatal("TerminalAdvice(screen-256color) = \"\", want an explanation")
	}
	for _, want := range []string{"screen-256color", "F5", "dv keys --tmux"} {
		if !strings.Contains(got, want) {
			t.Errorf("advice = %q, want it to mention %q", got, want)
		}
	}
}

// The snippet is copied straight into a config file, so the escape sequence
// has to be exactly what the kitty protocol specifies.
func TestGhosttySnippetEncodesCommandBindings(t *testing.T) {
	got := GhosttySnippet(Default())

	// ⌘↩ is CSI 13 ; 9 u — 13 is Enter, 9 is 1 + Super(8).
	if !strings.Contains(got, `keybind = cmd+enter=text:\x1b[13;9u`) {
		t.Errorf("snippet does not carry the ⌘↩ binding:\n%s", got)
	}
	// ⌘⇧↩ adds Shift: 1 + 1 + 8 = 10.
	if !strings.Contains(got, `keybind = cmd+shift+enter=text:\x1b[13;10u`) {
		t.Errorf("snippet does not carry the ⌘⇧↩ binding:\n%s", got)
	}
	// ⌘A is the letter's code point, 97, with Super.
	if !strings.Contains(got, `keybind = cmd+a=text:\x1b[97;9u`) {
		t.Errorf("snippet does not carry the ⌘A binding:\n%s", got)
	}
}

// Only ⌘ needs forwarding; emitting Ctrl bindings would override what the
// terminal already sends correctly.
func TestGhosttySnippetOmitsControlOnlyBindings(t *testing.T) {
	got := GhosttySnippet(Default())

	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "keybind") {
			continue
		}
		if !strings.Contains(line, "cmd+") {
			t.Errorf("snippet contains a non-⌘ binding: %s", line)
		}
	}
}

// Forwarding ⌘C at the terminal would disable its own copy for every program
// running in it — far more damage than the binding is worth.
func TestGhosttySnippetLeavesTheClipboardAlone(t *testing.T) {
	got := GhosttySnippet(Default())

	for _, forbidden := range []string{"cmd+c=", "cmd+x=", "cmd+v="} {
		if strings.Contains(got, forbidden) {
			t.Errorf("snippet rebinds %q, which would break the terminal's own clipboard:\n%s",
				forbidden, got)
		}
	}
	// And it must say why, so the omission does not read as an oversight.
	if !strings.Contains(got, "⌘C") {
		t.Errorf("snippet does not explain why the clipboard keys are absent:\n%s", got)
	}
}

// The internal stand-in rune for Ctrl+J is not a real key and must never
// reach a config file.
func TestGhosttySnippetSkipsTheEnterStandIn(t *testing.T) {
	if got := GhosttySnippet(Default()); strings.Contains(got, "\ue000") {
		t.Errorf("snippet leaked the internal stand-in rune:\n%s", got)
	}
}

func TestTmuxSnippetCarriesBothRequiredSettings(t *testing.T) {
	got := TmuxSnippet()

	for _, want := range []string{`default-terminal "tmux-256color"`, "extended-keys on"} {
		if !strings.Contains(got, want) {
			t.Errorf("tmux snippet is missing %q:\n%s", want, got)
		}
	}
}

func TestITerm2AdviceNamesTheSetting(t *testing.T) {
	got := ITerm2Advice()

	for _, want := range []string{"CSI u", "[13;9u"} {
		if !strings.Contains(got, want) {
			t.Errorf("iTerm2 advice is missing %q:\n%s", want, got)
		}
	}
}

func TestCsiUModifierArithmetic(t *testing.T) {
	tests := []struct {
		name    string
		binding Binding
		want    string
	}{
		{
			name:    "ctrl only",
			binding: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl},
			want:    `\x1b[13;5u`, // 1 + 4
		},
		{
			name:    "super only",
			binding: Binding{Key: tcell.KeyEnter, Mods: tcell.ModMeta},
			want:    `\x1b[13;9u`, // 1 + 8
		},
		{
			name:    "super and shift",
			binding: Binding{Key: tcell.KeyEnter, Mods: tcell.ModMeta | tcell.ModShift},
			want:    `\x1b[13;10u`, // 1 + 1 + 8
		},
		{
			name:    "ctrl and shift",
			binding: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl | tcell.ModShift},
			want:    `\x1b[13;6u`, // 1 + 1 + 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.binding.csiU()
			if !ok {
				t.Fatal("csiU() reported no sequence")
			}
			if got != tt.want {
				t.Errorf("csiU() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Function keys already arrive correctly, so there is nothing to configure.
func TestFunctionKeysHaveNoCsiUSequence(t *testing.T) {
	if _, ok := (Binding{Key: tcell.KeyF5}).csiU(); ok {
		t.Error("csiU() produced a sequence for F5, which needs no configuration")
	}
}

// Arrows keep the older xterm encoding even under the kitty protocol. Writing
// them as CSI-u would produce a keybind line the terminal quietly ignores.
func TestArrowsUseTheXtermEncoding(t *testing.T) {
	tests := []struct {
		name    string
		binding Binding
		want    string
	}{
		{
			name:    "cmd left",
			binding: Binding{Key: tcell.KeyLeft, Mods: tcell.ModMeta},
			want:    `\x1b[1;9D`, // 1 + Super(8)
		},
		{
			name:    "cmd right",
			binding: Binding{Key: tcell.KeyRight, Mods: tcell.ModMeta},
			want:    `\x1b[1;9C`,
		},
		{
			name:    "cmd shift left",
			binding: Binding{Key: tcell.KeyLeft, Mods: tcell.ModMeta | tcell.ModShift},
			want:    `\x1b[1;10D`, // 1 + Shift(1) + Super(8)
		},
		{
			name:    "alt left",
			binding: Binding{Key: tcell.KeyLeft, Mods: tcell.ModAlt},
			want:    `\x1b[1;3D`, // 1 + Alt(2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.binding.csiU()
			if !ok {
				t.Fatal("csiU() reported no sequence for an arrow key")
			}
			if got != tt.want {
				t.Errorf("csiU() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The line-movement bindings are ⌘-only, so they are exactly what the snippet
// exists to deliver. Leaving them out would make the file look complete while
// the keys stayed dead.
func TestGhosttySnippetCarriesLineMovement(t *testing.T) {
	got := GhosttySnippet(Default())

	for _, want := range []string{
		`keybind = cmd+left=text:\x1b[1;9D`,
		`keybind = cmd+right=text:\x1b[1;9C`,
		`keybind = cmd+shift+left=text:\x1b[1;10D`,
		`keybind = cmd+backspace=text:\x1b[127;9u`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("snippet is missing %q:\n%s", want, got)
		}
	}
}

// Word movement works through Option and Ctrl, which terminals already send;
// forwarding them would only risk overriding something that works.
func TestGhosttySnippetOmitsWordMovement(t *testing.T) {
	got := GhosttySnippet(Default())

	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "keybind") && strings.Contains(line, "alt+") {
			t.Errorf("snippet forwards an Option binding that already works: %s", line)
		}
	}
}
