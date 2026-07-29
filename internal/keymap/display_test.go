package keymap

import (
	"strings"
	"testing"
)

// Ctrl+Enter and its Ctrl+J stand-in render identically, so listing both
// shows the same key twice and looks like a bug in the table.
func TestDisplayBindingsHaveNoDuplicateLabels(t *testing.T) {
	m := Default()

	for _, a := range AllActions() {
		t.Run(a.String(), func(t *testing.T) {
			seen := make(map[string]bool)
			for _, b := range m.DisplayBindings(a) {
				label := b.Label(false)
				if seen[label] {
					t.Errorf("action %q lists %q more than once", a, label)
				}
				seen[label] = true
			}
		})
	}
}

// Every action still has something to show.
func TestDisplayBindingsAreNeverEmpty(t *testing.T) {
	m := Default()

	for _, a := range AllActions() {
		if len(m.DisplayBindings(a)) == 0 {
			t.Errorf("action %q has nothing to display", a)
		}
	}
}

// Hiding a duplicate must not unbind it: Ctrl+J still has to run.
func TestHiddenBindingsStillResolve(t *testing.T) {
	m := Default()

	shown := len(m.DisplayBindings(ActionRun))
	bound := len(m.Bindings(ActionRun))
	if bound <= shown {
		t.Fatalf("ActionRun shows %d of %d bindings; the stand-in should be hidden but kept", shown, bound)
	}
}

// The advice is about the terminal failing to report a modified Enter, so it
// has to name the Ctrl form. Naming the ⌘ form would send the reader to the
// wrong fix — ⌘ depends on terminal configuration, not on TERM.
func TestTerminalAdviceNamesTheControlBinding(t *testing.T) {
	got := TerminalAdvice("screen-256color", Default())

	if strings.Contains(got, "Super") || strings.Contains(got, "⌘") {
		t.Errorf("advice = %q, want it to name the Ctrl binding, not the ⌘ one", got)
	}
	if !strings.Contains(got, "Ctrl+↩") {
		t.Errorf("advice = %q, want it to name Ctrl+↩", got)
	}
}

// Labels are rendered into a fixed-width column, so their display width has
// to account for glyphs like ⌘ and ⇥ that occupy more than one cell.
func TestLabelWidthCountsDisplayCells(t *testing.T) {
	tests := []struct {
		label string
		want  int
	}{
		{label: "F5", want: 2},
		{label: "Ctrl+A", want: 6},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := LabelWidth(tt.label); got != tt.want {
				t.Errorf("LabelWidth(%q) = %d, want %d", tt.label, got, tt.want)
			}
		})
	}

	// The exact cell count for ⌘ varies by font and terminal; what matters
	// is that it is not counted as a plain single-width character, which is
	// what breaks column alignment.
	if got := LabelWidth("⌘↩"); got < 2 {
		t.Errorf("LabelWidth(%q) = %d, want at least 2", "⌘↩", got)
	}
}
