package keymap

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func lookup(t *testing.T, ev *tcell.EventKey) Action {
	t.Helper()
	return Default().Lookup(ev)
}

func key(k tcell.Key, mods tcell.ModMask) *tcell.EventKey {
	return tcell.NewEventKey(k, 0, mods)
}

func runeKey(r rune, mods tcell.ModMask) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, mods)
}

// The whole point of the rework: DataGrip's run key must work.
func TestRunIsBoundToCtrlEnter(t *testing.T) {
	if got := lookup(t, key(tcell.KeyEnter, tcell.ModCtrl)); got != ActionRun {
		t.Errorf("Ctrl+Enter = %v, want ActionRun", got)
	}
}

// "⌘ and Ctrl both work" is implemented by binding ModMeta alongside ModCtrl.
func TestCommandAndControlAreEquivalent(t *testing.T) {
	pairs := []struct {
		name string
		ctrl *tcell.EventKey
		meta *tcell.EventKey
		want Action
	}{
		{
			name: "run",
			ctrl: key(tcell.KeyEnter, tcell.ModCtrl),
			meta: key(tcell.KeyEnter, tcell.ModMeta),
			want: ActionRun,
		},
		{
			name: "run all",
			ctrl: key(tcell.KeyEnter, tcell.ModCtrl|tcell.ModShift),
			meta: key(tcell.KeyEnter, tcell.ModMeta|tcell.ModShift),
			want: ActionRunAll,
		},
		{
			name: "select all",
			ctrl: runeKey('a', tcell.ModCtrl),
			meta: runeKey('a', tcell.ModMeta),
			want: ActionSelectAll,
		},
		{
			name: "toggle comment",
			ctrl: runeKey('/', tcell.ModCtrl),
			meta: runeKey('/', tcell.ModMeta),
			want: ActionToggleComment,
		},
		{
			name: "duplicate line",
			ctrl: runeKey('d', tcell.ModCtrl),
			meta: runeKey('d', tcell.ModMeta),
			want: ActionDuplicateLine,
		},
		{
			name: "find",
			ctrl: runeKey('f', tcell.ModCtrl),
			meta: runeKey('f', tcell.ModMeta),
			want: ActionFind,
		},
		{
			name: "refresh schema",
			ctrl: runeKey('r', tcell.ModCtrl),
			meta: runeKey('r', tcell.ModMeta),
			want: ActionRefreshSchema,
		},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			if got := lookup(t, p.ctrl); got != p.want {
				t.Errorf("Ctrl form = %v, want %v", got, p.want)
			}
			if got := lookup(t, p.meta); got != p.want {
				t.Errorf("Cmd form = %v, want %v", got, p.want)
			}
		})
	}
}

// The same logical combination arrives as different events depending on
// whether the terminal speaks the extended keyboard protocol. Both encodings
// have to resolve to the same action or the key silently stops working when
// the user changes terminal.
func TestLegacyAndExtendedEncodingsAgree(t *testing.T) {
	tests := []struct {
		name     string
		extended *tcell.EventKey
		legacy   *tcell.EventKey
		want     Action
	}{
		{
			name:     "ctrl+enter",
			extended: key(tcell.KeyEnter, tcell.ModCtrl),
			legacy:   key(tcell.KeyCtrlJ, tcell.ModCtrl),
			want:     ActionRun,
		},
		{
			name:     "ctrl+slash",
			extended: runeKey('/', tcell.ModCtrl),
			legacy:   key(tcell.KeyCtrlUnderscore, tcell.ModCtrl),
			want:     ActionToggleComment,
		},
		{
			name:     "ctrl+space",
			extended: runeKey(' ', tcell.ModCtrl),
			legacy:   key(tcell.KeyNUL, tcell.ModCtrl),
			want:     ActionComplete,
		},
		{
			name:     "ctrl+y",
			extended: runeKey('y', tcell.ModCtrl),
			legacy:   key(tcell.KeyCtrlY, tcell.ModCtrl),
			want:     ActionDeleteLine,
		},
		{
			name:     "ctrl+d",
			extended: runeKey('d', tcell.ModCtrl),
			legacy:   key(tcell.KeyCtrlD, tcell.ModCtrl),
			want:     ActionDuplicateLine,
		},
		{
			name:     "ctrl+a",
			extended: runeKey('a', tcell.ModCtrl),
			legacy:   key(tcell.KeyCtrlA, tcell.ModCtrl),
			want:     ActionSelectAll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookup(t, tt.extended); got != tt.want {
				t.Errorf("extended encoding = %v, want %v", got, tt.want)
			}
			if got := lookup(t, tt.legacy); got != tt.want {
				t.Errorf("legacy encoding = %v, want %v", got, tt.want)
			}
		})
	}
}

// Terminals without the extended protocol cannot express Ctrl+Shift+X, so
// every such binding needs a function-key fallback.
func TestFallbackKeys(t *testing.T) {
	tests := []struct {
		name string
		ev   *tcell.EventKey
		want Action
	}{
		{name: "F5 runs", ev: key(tcell.KeyF5, tcell.ModNone), want: ActionRun},
		{name: "Shift+F5 runs everything", ev: key(tcell.KeyF5, tcell.ModShift), want: ActionRunAll},
		{name: "F10 quits", ev: key(tcell.KeyF10, tcell.ModNone), want: ActionQuit},
		{name: "F1 opens help", ev: key(tcell.KeyF1, tcell.ModNone), want: ActionHelp},
		{name: "Tab moves focus", ev: key(tcell.KeyTab, tcell.ModNone), want: ActionNextPane},
		{name: "Shift+Tab moves back", ev: key(tcell.KeyBacktab, tcell.ModNone), want: ActionPrevPane},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookup(t, tt.ev); got != tt.want {
				t.Errorf("%v = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCancelBindings(t *testing.T) {
	if got := lookup(t, key(tcell.KeyF2, tcell.ModCtrl)); got != ActionCancel {
		t.Errorf("Ctrl+F2 = %v, want ActionCancel", got)
	}
	if got := lookup(t, key(tcell.KeyF2, tcell.ModMeta)); got != ActionCancel {
		t.Errorf("Cmd+F2 = %v, want ActionCancel", got)
	}
}

// Ctrl+C is deliberately its own action: whether it copies or cancels
// depends on runtime state the keymap cannot see.
func TestCtrlCIsItsOwnAction(t *testing.T) {
	for _, ev := range []*tcell.EventKey{
		runeKey('c', tcell.ModCtrl),
		key(tcell.KeyCtrlC, tcell.ModCtrl),
		runeKey('c', tcell.ModMeta),
	} {
		if got := Default().Lookup(ev); got != ActionCopyOrCancel {
			t.Errorf("%v = %v, want ActionCopyOrCancel", ev.Name(), got)
		}
	}
}

// Plain typing must never be swallowed by the keymap.
func TestUnmodifiedRunesAreNotActions(t *testing.T) {
	for _, r := range []rune{'a', 'z', '/', ' ', '1', '한'} {
		if got := lookup(t, runeKey(r, tcell.ModNone)); got != ActionNone {
			t.Errorf("plain %q = %v, want ActionNone", r, got)
		}
	}
}

// Enter without a modifier inserts a newline; only the modified form runs.
func TestPlainEnterIsNotRun(t *testing.T) {
	if got := lookup(t, key(tcell.KeyEnter, tcell.ModNone)); got != ActionNone {
		t.Errorf("plain Enter = %v, want ActionNone", got)
	}
}

// Alt is a distinct modifier; Alt+Enter must not run the statement.
func TestAltDoesNotStandInForCtrl(t *testing.T) {
	if got := lookup(t, key(tcell.KeyEnter, tcell.ModAlt)); got != ActionNone {
		t.Errorf("Alt+Enter = %v, want ActionNone", got)
	}
}

// The help screen is generated from these, so every action a user can reach
// must be able to describe itself.
func TestEveryBoundActionHasANameAndBindings(t *testing.T) {
	m := Default()

	for _, a := range AllActions() {
		t.Run(a.String(), func(t *testing.T) {
			if a.String() == "" || strings.HasPrefix(a.String(), "Action(") {
				t.Errorf("action %d has no readable name", int(a))
			}
			if len(m.Bindings(a)) == 0 {
				t.Errorf("action %q has no default binding", a)
			}
		})
	}
}

// Binding.Label is what the help screen prints.
func TestBindingLabel(t *testing.T) {
	tests := []struct {
		binding Binding
		mac     string
		other   string
	}{
		{
			binding: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl},
			mac:     "^↩",
			other:   "Ctrl+↩",
		},
		{
			binding: Binding{Key: tcell.KeyEnter, Mods: tcell.ModMeta},
			mac:     "⌘↩",
			other:   "Super+↩",
		},
		{
			binding: Binding{Key: tcell.KeyRune, Rune: 'a', Mods: tcell.ModCtrl},
			mac:     "^A",
			other:   "Ctrl+A",
		},
		{
			binding: Binding{Key: tcell.KeyF5},
			mac:     "F5",
			other:   "F5",
		},
		{
			binding: Binding{Key: tcell.KeyEnter, Mods: tcell.ModMeta | tcell.ModShift},
			mac:     "⌘⇧↩",
			other:   "Super+Shift+↩",
		},
	}

	for _, tt := range tests {
		t.Run(tt.other, func(t *testing.T) {
			if got := tt.binding.Label(true); got != tt.mac {
				t.Errorf("Label(mac) = %q, want %q", got, tt.mac)
			}
			if got := tt.binding.Label(false); got != tt.other {
				t.Errorf("Label(other) = %q, want %q", got, tt.other)
			}
		})
	}
}

// Every bound action does something today, so nothing may be marked
// reserved — a key that announces itself as unbuilt while working would be
// worse than either state alone.
func TestNoBoundActionIsMarkedReserved(t *testing.T) {
	for _, a := range AllActions() {
		if a.Reserved() {
			t.Errorf("%v.Reserved() = true, but it is bound and implemented", a)
		}
	}
}

// The mechanism stays available for keys bound ahead of their feature.
func TestReservedIsFalseForImplementedActions(t *testing.T) {
	for _, a := range []Action{ActionRun, ActionFind, ActionSearchHistory, ActionComplete} {
		if a.Reserved() {
			t.Errorf("%v.Reserved() = true, want false", a)
		}
	}
}

func TestCopyResultIsBoundToCommandShiftC(t *testing.T) {
	m := Default()
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModMeta|tcell.ModShift),
		tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModCtrl|tcell.ModShift),
		tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone),
	} {
		if got := m.Lookup(ev); got != ActionCopyResult {
			t.Errorf("Lookup(%v) = %v, want ActionCopyResult", ev.Name(), got)
		}
	}
	// Plain ⌘C must still be the copy-or-cancel key.
	if got := m.Lookup(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModMeta)); got != ActionCopyOrCancel {
		t.Errorf("⌘C = %v, want ActionCopyOrCancel", got)
	}
}
