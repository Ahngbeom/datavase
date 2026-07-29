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

// Event renders the binding as an event, which is how tests and the round
// trip from configuration reach Lookup.
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
	tcell.KeyCtrlN: 'n',
	tcell.KeyCtrlP: 'p',
	tcell.KeyCtrlQ: 'q',
	tcell.KeyCtrlR: 'r',
	tcell.KeyCtrlV: 'v',
	tcell.KeyCtrlX: 'x',
	tcell.KeyCtrlY: 'y',
	tcell.KeyCtrlZ: 'z',

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

// modifierNames are accepted in configuration, in the order they are printed.
var modifierNames = map[string]tcell.ModMask{
	"ctrl":    tcell.ModCtrl,
	"control": tcell.ModCtrl,
	"cmd":     tcell.ModMeta,
	"command": tcell.ModMeta,
	"super":   tcell.ModMeta,
	"meta":    tcell.ModMeta,
	"shift":   tcell.ModShift,
	"alt":     tcell.ModAlt,
	"option":  tcell.ModAlt,
}

// namedKeys are the non-character keys configuration can refer to.
var namedKeys = map[string]tcell.Key{
	"enter":     tcell.KeyEnter,
	"return":    tcell.KeyEnter,
	"tab":       tcell.KeyTab,
	"backtab":   tcell.KeyBacktab,
	"escape":    tcell.KeyEscape,
	"esc":       tcell.KeyEscape,
	"backspace": tcell.KeyBackspace2,
	"delete":    tcell.KeyDelete,
	"insert":    tcell.KeyInsert,
	"home":      tcell.KeyHome,
	"end":       tcell.KeyEnd,
	"pageup":    tcell.KeyPgUp,
	"pagedown":  tcell.KeyPgDn,
	"up":        tcell.KeyUp,
	"down":      tcell.KeyDown,
	"left":      tcell.KeyLeft,
	"right":     tcell.KeyRight,
}

// functionKeys covers f1 through f12.
var functionKeys = map[string]tcell.Key{
	"f1": tcell.KeyF1, "f2": tcell.KeyF2, "f3": tcell.KeyF3, "f4": tcell.KeyF4,
	"f5": tcell.KeyF5, "f6": tcell.KeyF6, "f7": tcell.KeyF7, "f8": tcell.KeyF8,
	"f9": tcell.KeyF9, "f10": tcell.KeyF10, "f11": tcell.KeyF11, "f12": tcell.KeyF12,
}

// ParseBinding reads a specification such as "ctrl+shift+enter".
func ParseBinding(spec string) (Binding, error) {
	parts := strings.Split(spec, "+")

	var (
		mods tcell.ModMask
		key  string
	)
	for i, part := range parts {
		p := strings.ToLower(strings.TrimSpace(part))
		if p == "" {
			return Binding{}, fmt.Errorf("key binding %q has an empty part", spec)
		}

		// Everything but the last part must be a modifier.
		if i < len(parts)-1 {
			m, ok := modifierNames[p]
			if !ok {
				return Binding{}, fmt.Errorf("key binding %q: unknown modifier %q", spec, p)
			}
			mods |= m
			continue
		}
		key = p
	}

	if key == "" {
		return Binding{}, fmt.Errorf("key binding %q is empty", spec)
	}
	// A lone modifier is not a binding.
	if _, isMod := modifierNames[key]; isMod {
		return Binding{}, fmt.Errorf("key binding %q names only a modifier", spec)
	}

	if k, ok := namedKeys[key]; ok {
		return Binding{Key: k, Mods: mods}, nil
	}
	if k, ok := functionKeys[key]; ok {
		return Binding{Key: k, Mods: mods}, nil
	}
	if key == "space" {
		return Binding{Key: tcell.KeyRune, Rune: ' ', Mods: mods}, nil
	}
	if r := []rune(key); len(r) == 1 {
		return Binding{Key: tcell.KeyRune, Rune: unicode.ToLower(r[0]), Mods: mods}, nil
	}

	return Binding{}, fmt.Errorf("key binding %q: unknown key %q", spec, key)
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
