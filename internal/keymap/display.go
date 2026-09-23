package keymap

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// DisplayBindings returns an action's bindings with duplicates collapsed,
// for the help screen.
//
// Some bindings exist only so a key keeps working on terminals that cannot
// report it properly — Ctrl+J standing in for Ctrl+Enter, for instance. They
// render identically to the binding they stand for, and listing both makes
// the table look broken. They stay bound; they just stop being advertised.
func (m *Map) DisplayBindings(a Action) []Binding {
	bindings := m.Bindings(a)

	out := make([]Binding, 0, len(bindings))
	seen := make(map[string]bool, len(bindings))
	for _, b := range bindings {
		label := b.Label(false)
		if seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, b)
	}
	return out
}

// PreferredBinding is the one to put in front of someone when there is room
// for exactly one, and whether the action has any.
//
// Where the Apple glyphs are in use, that is whatever the map ranks first —
// ⌘, the key a Mac user reaches for. Where they are not, a ⌘ binding is
// spelled "Super", and a Mac inside tmux gets told to press a key its
// keyboard does not have and tmux would not forward anyway. So the Ctrl
// twin is preferred there: on that Mac it is the half of the pair that can
// arrive, and on Linux it is the half the terminal is likelier to deliver.
func (m *Map) PreferredBinding(a Action, mac bool) (Binding, bool) {
	bindings := m.DisplayBindings(a)
	if len(bindings) == 0 {
		return Binding{}, false
	}
	if !mac {
		for _, b := range bindings {
			if b.Mods&tcell.ModCtrl != 0 {
				return b, true
			}
		}
	}
	return bindings[0], true
}

// labelWidth is how many terminal cells a key label occupies.
//
// Glyphs such as ⌘ and ⇥ are wider than one cell in most fonts, so counting
// runes would leave the description column ragged.
func labelWidth(label string) int {
	return runewidth.StringWidth(label)
}

// PadLabel right-pads a label to the given display width.
func PadLabel(label string, width int) string {
	if pad := width - labelWidth(label); pad > 0 {
		return label + spaces(pad)
	}
	return label
}

func spaces(n int) string {
	const blanks = "                                                                "
	if n <= len(blanks) {
		return blanks[:n]
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = ' '
	}
	return string(out)
}
