package keymap

import (
	"github.com/mattn/go-runewidth"
)

// DisplayBindings returns an action's bindings with duplicates collapsed,
// for the help screen and `dv keys`.
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

// LabelWidth is how many terminal cells a key label occupies.
//
// Glyphs such as ⌘ and ⇥ are wider than one cell in most fonts, so counting
// runes would leave the description column ragged.
func LabelWidth(label string) int {
	return runewidth.StringWidth(label)
}

// PadLabel right-pads a label to the given display width.
func PadLabel(label string, width int) string {
	if pad := width - LabelWidth(label); pad > 0 {
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

// SplitByFamiliarity divides actions into the ones dv has to teach and the
// ones a reader already knows, keeping each group in the order given.
//
// The order is the caller's: the help screen groups by purpose and `dv keys`
// follows the reference order, and neither wants this to have an opinion.
func SplitByFamiliarity(actions []Action) (ours, known []Action) {
	for _, a := range actions {
		if a.Familiar() {
			known = append(known, a)
			continue
		}
		ours = append(ours, a)
	}
	return ours, known
}
