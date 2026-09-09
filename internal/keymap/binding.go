package keymap

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Binding is one key combination in a canonical form.
//
// Canonical means: a combination that can be written as a rune is stored as
// KeyRune plus a lower-case rune, never as one of tcell's control-code
// constants. Terminals disagree about which form they send — an extended
// terminal reports Ctrl+A as KeyRune 'a' with ModCtrl, a legacy one as
// KeyCtrlA — and folding both into one shape is what lets a single
// registration serve every terminal.
type Binding struct {
	Key  tcell.Key
	Rune rune // only meaningful when Key is tcell.KeyRune
	Mods tcell.ModMask
}

// Event renders the binding as an event, which is how tests reach Lookup
// without constructing a tcell.EventKey by hand.
func (b Binding) Event() *tcell.EventKey {
	return tcell.NewEventKey(b.Key, b.Rune, b.Mods)
}

// normalize folds an incoming event into the canonical form.
func normalize(ev *tcell.EventKey) Binding {
	k, r, mods := ev.Key(), ev.Rune(), ev.Modifiers()

	// Alt is reported both as a modifier and, on some terminals, by way of an
	// escape prefix. Nothing here binds Alt, so it is kept as-is.
	if k == tcell.KeyRune {
		return Binding{Key: tcell.KeyRune, Rune: unicode.ToLower(r), Mods: mods}
	}

	// Legacy control codes: fold back to the letter they stand for.
	if letter, ok := controlCodeRune[k]; ok {
		return Binding{Key: tcell.KeyRune, Rune: letter, Mods: mods | tcell.ModCtrl}
	}

	// Named keys that alias each other.
	if alias, ok := keyAliases[k]; ok {
		k = alias
	}
	return Binding{Key: k, Mods: mods}
}

// controlCodeRune maps tcell's legacy control-code keys back to the letter
// that produced them.
//
// Only keys datavase binds are listed. Notably absent are KeyTab (9),
// KeyEnter (13) and KeyEscape (27), which are real keys in their own right
// and must not be folded into Ctrl+I, Ctrl+M and Ctrl+[.
var controlCodeRune = map[tcell.Key]rune{
	tcell.KeyCtrlA: 'a',
	tcell.KeyCtrlB: 'b',
	tcell.KeyCtrlC: 'c',
	tcell.KeyCtrlD: 'd',
	tcell.KeyCtrlF: 'f',
	tcell.KeyCtrlG: 'g',
	tcell.KeyCtrlN: 'n',
	tcell.KeyCtrlQ: 'q',
	tcell.KeyCtrlR: 'r',
	tcell.KeyCtrlS: 's',
	tcell.KeyCtrlV: 'v',
	tcell.KeyCtrlX: 'x',
	tcell.KeyCtrlY: 'y',

	// Ctrl+Space arrives as NUL on terminals without the extended protocol.
	tcell.KeyNUL: ' ',
	// Ctrl+/ arrives as the unit separator, 0x1F.
	tcell.KeyCtrlUnderscore: '/',
	// Ctrl+Enter degrades to a line feed, which is Ctrl+J.
	tcell.KeyCtrlJ: enterStandIn,
}

// enterStandIn is the rune Ctrl+J folds to. It is not a character anyone can
// type, so it cannot collide with a real binding; Default registers Ctrl+Enter
// under this rune as well as under KeyEnter.
const enterStandIn = '\ue000'

// keyAliases collapses keys tcell reports inconsistently across terminals.
var keyAliases = map[tcell.Key]tcell.Key{
	tcell.KeyBackspace: tcell.KeyBackspace2,
}

// keyLabels are how named keys print on the help screen.
var keyLabels = map[tcell.Key]string{
	tcell.KeyEnter:      "↩",
	tcell.KeyTab:        "⇥",
	tcell.KeyBacktab:    "⇧⇥",
	tcell.KeyEscape:     "Esc",
	tcell.KeyBackspace2: "⌫",
	tcell.KeyDelete:     "Del",
	tcell.KeyUp:         "↑",
	tcell.KeyDown:       "↓",
	tcell.KeyLeft:       "←",
	tcell.KeyRight:      "→",
	tcell.KeyPgUp:       "PgUp",
	tcell.KeyPgDn:       "PgDn",
	tcell.KeyHome:       "Home",
	tcell.KeyEnd:        "End",
}

// Label renders the binding for display. On macOS the familiar glyphs are
// used, since that is what DataGrip and every other Mac application shows.
func (b Binding) Label(mac bool) string {
	var sb strings.Builder

	if mac {
		// Glyph order follows the Apple convention: ⌃⌥⇧⌘.
		if b.Mods&tcell.ModCtrl != 0 {
			sb.WriteString("^")
		}
		if b.Mods&tcell.ModAlt != 0 {
			sb.WriteString("⌥")
		}
		if b.Mods&tcell.ModMeta != 0 {
			sb.WriteString("⌘")
		}
		if b.Mods&tcell.ModShift != 0 {
			sb.WriteString("⇧")
		}
	} else {
		var names []string
		if b.Mods&tcell.ModCtrl != 0 {
			names = append(names, "Ctrl")
		}
		if b.Mods&tcell.ModMeta != 0 {
			names = append(names, "Super")
		}
		if b.Mods&tcell.ModAlt != 0 {
			names = append(names, "Alt")
		}
		if b.Mods&tcell.ModShift != 0 {
			names = append(names, "Shift")
		}
		if len(names) > 0 {
			sb.WriteString(strings.Join(names, "+"))
			sb.WriteString("+")
		}
	}

	sb.WriteString(b.keyLabel())
	return sb.String()
}

func (b Binding) keyLabel() string {
	if label, ok := keyLabels[b.Key]; ok {
		return label
	}
	if b.Key >= tcell.KeyF1 && b.Key <= tcell.KeyF12 {
		return fmt.Sprintf("F%d", int(b.Key-tcell.KeyF1)+1)
	}
	if b.Key == tcell.KeyRune {
		switch b.Rune {
		case ' ':
			return "Space"
		case enterStandIn:
			return "↩"
		}
		return strings.ToUpper(string(b.Rune))
	}
	return tcell.KeyNames[b.Key]
}

// sortBindings gives the help screen a stable, sensible order: the most
// idiomatic form first, fallbacks after.
//
// The order follows what a Mac user reaches for, since that is the platform
// whose conventions this keymap was built around: ⌘, then ⌥, then Ctrl, then
// bare keys.
func sortBindings(bindings []Binding) {
	rank := func(b Binding) int {
		switch {
		case b.Mods&tcell.ModMeta != 0:
			return 0
		case b.Mods&tcell.ModAlt != 0:
			return 1 // ⌥ is the Mac spelling of word movement
		case b.Mods&tcell.ModCtrl != 0:
			return 2
		default:
			return 3 // bare function keys are fallbacks
		}
	}
	sort.SliceStable(bindings, func(i, j int) bool {
		return rank(bindings[i]) < rank(bindings[j])
	})
}
