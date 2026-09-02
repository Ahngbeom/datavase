package keymap

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// SupportsExtendedKeys reports whether the terminal named by TERM can report
// modified keys such as Ctrl+Enter.
//
// tcell enables the extended keyboard protocols only for terminals its
// terminfo marks XTermLike. The common trap is tmux, whose default TERM of
// "screen-256color" is not — so Ctrl+Enter silently degrades there while the
// very same tmux running as "tmux-256color" handles it.
func SupportsExtendedKeys(term string) bool {
	if term == "" {
		term = os.Getenv("TERM")
	}

	// Match how tcell decides: the terminfo name's family, not the exact
	// string, since suffixes like -256color and -direct all share behaviour.
	switch {
	case strings.HasPrefix(term, "xterm"):
		return true
	case strings.HasPrefix(term, "tmux"):
		return true
	case strings.HasPrefix(term, "screen"):
		// screen and tmux-under-screen predate the protocols and drop the
		// sequences rather than forwarding them.
		return false
	case term == "alacritty", term == "foot", term == "wezterm", term == "contour":
		return true
	default:
		return false
	}
}

// TerminalAdvice returns a one-line hint when the current terminal cannot
// deliver the primary bindings, or "" when everything works.
func TerminalAdvice(term string, m *Map) string {
	if SupportsExtendedKeys(term) {
		return ""
	}

	primary, fallback := primaryAndFallback(m, ActionRun)
	if primary == nil || fallback == nil {
		return ""
	}

	return fmt.Sprintf("%s does not work in TERM=%s — use %s, or run `dv keys --tmux`",
		primary.Label(false), term, fallback.Label(false))
}

// TerminalAdviceShort is the same warning for a line that has to share.
//
// The full sentence is nearly eighty cells, which is a whole default terminal:
// on the status bar it could not sit beside the two hints it opens with, so on
// exactly the terminals that need it the advice was the clause with no room.
// This keeps the instruction and drops the diagnosis, which the help screen
// and `dv keys` still carry in full.
func TerminalAdviceShort(term string, m *Map) string {
	if SupportsExtendedKeys(term) {
		return ""
	}

	primary, fallback := primaryAndFallback(m, ActionRun)
	if primary == nil || fallback == nil {
		return ""
	}

	// The palette's own fallback is the one key guaranteed to reach an
	// action that has no fallback of its own; without one, degrade to the
	// run-only sentence rather than name a key that cannot be pressed.
	_, palette := primaryAndFallback(m, ActionCommandPalette)
	if palette == nil {
		return fmt.Sprintf("%s does not work here — use %s",
			primary.Label(false), fallback.Label(false))
	}

	return fmt.Sprintf("%s blocked — %s, %s lists commands",
		primary.Label(false), fallback.Label(false), palette.Label(false))
}

// primaryAndFallback picks the binding this advice is about and the one that
// survives a terminal without the extended protocols.
//
// The Ctrl form is chosen deliberately: TERM is what stops a modified Enter
// from being reported, whereas ⌘ depends on the terminal forwarding it at
// all. Naming the ⌘ binding here would point the reader at the wrong fix.
func primaryAndFallback(m *Map, a Action) (primary, fallback *Binding) {
	for _, b := range m.Bindings(a) {
		b := b
		if primary == nil && b.Mods&tcell.ModCtrl != 0 && b.Mods&tcell.ModMeta == 0 {
			primary = &b
		}
		if b.Mods == tcell.ModNone {
			fallback = &b
		}
	}
	return primary, fallback
}

// GhosttySnippet returns configuration that makes Ghostty forward the ⌘
// bindings to the application.
//
// macOS terminals keep Cmd for their own menus, so the combinations have to
// be sent explicitly. The escape sequences are the kitty keyboard protocol's
// CSI-u form: CSI <key> ; <modifiers> u, where the modifier value is
// 1 + (shift 1, alt 2, ctrl 4, super 8).
func GhosttySnippet(m *Map) string {
	var b strings.Builder

	b.WriteString("# datavase — forward ⌘ bindings to the terminal application\n")
	b.WriteString("# Append to ~/.config/ghostty/config, then restart Ghostty.\n")
	b.WriteString("#\n")
	b.WriteString("# Without these, macOS keeps ⌘ for Ghostty's own menus and the\n")
	b.WriteString("# application never sees it. The Ctrl bindings work either way.\n")
	b.WriteString("#\n")
	b.WriteString("# ⌘C, ⌘X and ⌘V are deliberately absent. Forwarding them would\n")
	b.WriteString("# take Ghostty's own copy and paste away from every other program\n")
	b.WriteString("# you run in this terminal — a steep price for keys that already\n")
	b.WriteString("# work. Inside datavase use ^C, ^X and ^V.\n")
	b.WriteString("#\n")
	b.WriteString("# ⌘F2 (cancel) is absent too: function keys use a different\n")
	b.WriteString("# encoding that cannot be expressed this way. Use ^F2, or ^C with\n")
	b.WriteString("# nothing selected.\n\n")

	for _, a := range AllActions() {
		if clipboardActions[a] {
			continue
		}
		for _, binding := range m.Bindings(a) {
			if binding.Mods&tcell.ModMeta == 0 {
				continue
			}
			seq, ok := binding.csiU()
			if !ok {
				continue
			}
			b.WriteString(fmt.Sprintf("keybind = %s=text:%s  # %s\n",
				binding.ghosttySpec(), seq, a.Describe()))
		}
	}
	return b.String()
}

// clipboardActions are left out of terminal configuration snippets.
//
// Rebinding ⌘C or ⌘V at the terminal disables its own clipboard for every
// program running in it, not just datavase. The terminal already handles
// these correctly — selection copies, bracketed paste delivers — so there is
// nothing to gain and a great deal to lose.
var clipboardActions = map[Action]bool{
	ActionCopy:         true,
	ActionCut:          true,
	ActionPaste:        true,
	ActionCopyOrCancel: true,
}

// TmuxSnippet returns the settings that let tmux carry modified keys and
// mouse events through to the application.
func TmuxSnippet() string {
	return `# datavase — let tmux carry modified keys and mouse events through to it
# Append to ~/.tmux.conf, then: tmux kill-server (or restart your session).
#
# The default TERM inside tmux is "screen-256color", which predates the
# extended keyboard protocols; applications therefore never receive
# Ctrl+Enter. Switching to "tmux-256color" and passing extended keys
# through fixes it. Requires tmux 3.2 or newer.

set -g default-terminal "tmux-256color"
set -g extended-keys on
set -as terminal-features 'xterm*:extkeys'

# tmux forwards no mouse events at all until told to, so clicking a table
# or dragging to resize a pane reaches the application only with this on.
# Once it is, tmux's own copy-by-drag needs a modifier held — on most
# terminals that means holding Shift while you drag to select text.
set -g mouse on
`
}

// ITerm2Advice explains the equivalent setup for iTerm2, which is done in the
// user interface rather than a configuration file.
func ITerm2Advice() string {
	return `# datavase — iTerm2 setup

iTerm2 has no plain-text config for this; the two settings are in Preferences.

1. Report modifier keys to the application
   Preferences → Profiles → Keys → General
   set "Report modifiers using CSI u" (iTerm2 3.5 and newer).
   This alone makes Ctrl+Enter, Ctrl+Shift+… and friends work.

2. Optional: forward ⌘ bindings
   Preferences → Profiles → Keys → Key Mappings → +
   Keyboard shortcut: ⌘↩
   Action: "Send Escape Sequence"
   Esc+: [13;9u

   The value after the semicolon is 1 + Super(8) = 9. Repeat for any other
   ⌘ combination you want the application to receive.

The Ctrl bindings work without any of this.
`
}

// csiU renders the binding as an escape sequence the terminal can send, if
// one exists.
func (b Binding) csiU() (string, bool) {
	mods := b.modifierCode()

	// Arrows keep the older xterm encoding even under the kitty protocol:
	// CSI 1 ; <mods> <letter>, rather than the CSI-u form used for character
	// keys. Emitting them as CSI-u would produce a line the terminal ignores.
	if letter, ok := arrowLetter[b.Key]; ok {
		return fmt.Sprintf(`\x1b[1;%d%s`, mods, letter), true
	}

	code, ok := b.csiUCode()
	if !ok {
		return "", false
	}
	return fmt.Sprintf(`\x1b[%d;%du`, code, mods), true
}

// modifierCode is the kitty protocol's modifier value: a bit sum offset by
// one, so "no modifiers" is 1 rather than 0.
func (b Binding) modifierCode() int {
	mods := 1
	if b.Mods&tcell.ModShift != 0 {
		mods += 1
	}
	if b.Mods&tcell.ModAlt != 0 {
		mods += 2
	}
	if b.Mods&tcell.ModCtrl != 0 {
		mods += 4
	}
	if b.Mods&tcell.ModMeta != 0 {
		mods += 8
	}
	return mods
}

// arrowLetter is the final byte of an arrow key's escape sequence.
var arrowLetter = map[tcell.Key]string{
	tcell.KeyUp:    "A",
	tcell.KeyDown:  "B",
	tcell.KeyRight: "C",
	tcell.KeyLeft:  "D",
}

// csiUCode is the Unicode code point the protocol uses to name the key.
func (b Binding) csiUCode() (int, bool) {
	switch b.Key {
	case tcell.KeyEnter:
		return 13, true
	case tcell.KeyTab:
		return 9, true
	case tcell.KeyEscape:
		return 27, true
	case tcell.KeyBackspace2:
		return 127, true
	case tcell.KeyRune:
		if b.Rune == enterStandIn {
			return 0, false
		}
		return int(b.Rune), true
	}
	// Function keys use a different encoding that terminals already send
	// correctly, so there is nothing to configure for them.
	return 0, false
}

// ghosttySpec renders the binding in Ghostty's keybind syntax.
func (b Binding) ghosttySpec() string {
	var parts []string
	if b.Mods&tcell.ModMeta != 0 {
		parts = append(parts, "cmd")
	}
	if b.Mods&tcell.ModCtrl != 0 {
		parts = append(parts, "ctrl")
	}
	if b.Mods&tcell.ModAlt != 0 {
		parts = append(parts, "alt")
	}
	if b.Mods&tcell.ModShift != 0 {
		parts = append(parts, "shift")
	}

	switch b.Key {
	case tcell.KeyEnter:
		parts = append(parts, "enter")
	case tcell.KeyTab:
		parts = append(parts, "tab")
	case tcell.KeyEscape:
		parts = append(parts, "escape")
	case tcell.KeyBackspace2:
		parts = append(parts, "backspace")
	case tcell.KeyUp:
		parts = append(parts, "up")
	case tcell.KeyDown:
		parts = append(parts, "down")
	case tcell.KeyLeft:
		parts = append(parts, "left")
	case tcell.KeyRight:
		parts = append(parts, "right")
	case tcell.KeyRune:
		parts = append(parts, string(b.Rune))
	default:
		parts = append(parts, strings.ToLower(tcell.KeyNames[b.Key]))
	}
	return strings.Join(parts, "+")
}
