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

func TestParseBinding(t *testing.T) {
	tests := []struct {
		in   string
		want Binding
	}{
		{in: "ctrl+enter", want: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl}},
		{in: "cmd+enter", want: Binding{Key: tcell.KeyEnter, Mods: tcell.ModMeta}},
		{in: "ctrl+shift+enter", want: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl | tcell.ModShift}},
		{in: "f5", want: Binding{Key: tcell.KeyF5}},
		{in: "shift+f5", want: Binding{Key: tcell.KeyF5, Mods: tcell.ModShift}},
		{in: "ctrl+a", want: Binding{Key: tcell.KeyRune, Rune: 'a', Mods: tcell.ModCtrl}},
		{in: "ctrl+/", want: Binding{Key: tcell.KeyRune, Rune: '/', Mods: tcell.ModCtrl}},
		{in: "ctrl+space", want: Binding{Key: tcell.KeyRune, Rune: ' ', Mods: tcell.ModCtrl}},
		{in: "tab", want: Binding{Key: tcell.KeyTab}},
		{in: "escape", want: Binding{Key: tcell.KeyEscape}},
		{in: "backspace", want: Binding{Key: tcell.KeyBackspace2}},
		// Case and spacing are the user's business, not ours.
		{in: "Ctrl+Enter", want: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl}},
		{in: " ctrl + enter ", want: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl}},
		// Aliases people will reasonably type.
		{in: "super+a", want: Binding{Key: tcell.KeyRune, Rune: 'a', Mods: tcell.ModMeta}},
		{in: "command+a", want: Binding{Key: tcell.KeyRune, Rune: 'a', Mods: tcell.ModMeta}},
		{in: "control+a", want: Binding{Key: tcell.KeyRune, Rune: 'a', Mods: tcell.ModCtrl}},
		{in: "ctrl+return", want: Binding{Key: tcell.KeyEnter, Mods: tcell.ModCtrl}},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseBinding(tt.in)
			if err != nil {
				t.Fatalf("ParseBinding(%q) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseBinding(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseBindingRejectsNonsense(t *testing.T) {
	tests := []struct {
		in      string
		wantErr string
	}{
		{in: "", wantErr: "empty"},
		{in: "ctrl+", wantErr: "empty"},
		{in: "ctrl", wantErr: "modifier"},
		{in: "hyperspace+a", wantErr: "hyperspace"},
		{in: "ctrl+notakey", wantErr: "notakey"},
		{in: "ctrl+f99", wantErr: "f99"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := ParseBinding(tt.in)
			if err == nil {
				t.Fatalf("ParseBinding(%q) error = nil, want an error", tt.in)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseBinding(%q) error = %q, want it to mention %q", tt.in, err, tt.wantErr)
			}
		})
	}
}

// A binding parsed from config has to resolve through Lookup; otherwise the
// customisation silently does nothing.
func TestParsedBindingsRoundTripThroughLookup(t *testing.T) {
	m := Default()

	for _, spec := range []string{"ctrl+enter", "cmd+enter", "f5"} {
		t.Run(spec, func(t *testing.T) {
			b, err := ParseBinding(spec)
			if err != nil {
				t.Fatalf("ParseBinding(%q) error = %v", spec, err)
			}
			if got := m.Lookup(b.Event()); got != ActionRun {
				t.Errorf("Lookup(%q) = %v, want ActionRun", spec, got)
			}
		})
	}
}

func TestApplyOverridesReplacesBindings(t *testing.T) {
	m := Default()
	if err := m.Apply(map[string][]string{"run": {"f8"}}); err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	if got := m.Lookup(key(tcell.KeyF8, tcell.ModNone)); got != ActionRun {
		t.Errorf("F8 = %v, want ActionRun after override", got)
	}
	// Replacing, not adding: the defaults for that action are gone.
	if got := m.Lookup(key(tcell.KeyF5, tcell.ModNone)); got == ActionRun {
		t.Error("F5 still runs; Apply must replace an action's bindings, not extend them")
	}
	// Other actions are untouched.
	if got := m.Lookup(key(tcell.KeyF1, tcell.ModNone)); got != ActionHelp {
		t.Errorf("F1 = %v, want ActionHelp", got)
	}
}

// A typo in an action name is a silent no-op unless we reject it.
func TestApplyRejectsUnknownActionNames(t *testing.T) {
	err := Default().Apply(map[string][]string{"runn": {"f8"}})
	if err == nil {
		t.Fatal("Apply() error = nil, want an error about the unknown action")
	}
	if !strings.Contains(err.Error(), "runn") {
		t.Errorf("Apply() error = %q, want it to name the unknown action", err)
	}
	// The message should help, so it lists what is valid.
	if !strings.Contains(err.Error(), "run") {
		t.Errorf("Apply() error = %q, want it to list valid action names", err)
	}
}

func TestApplyRejectsUnparsableBindings(t *testing.T) {
	err := Default().Apply(map[string][]string{"run": {"ctrl+nope"}})
	if err == nil {
		t.Fatal("Apply() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "run") || !strings.Contains(err.Error(), "nope") {
		t.Errorf("Apply() error = %q, want it to name both the action and the bad binding", err)
	}
}

// Applying a bad override must leave the map usable rather than half-updated.
func TestApplyIsAtomic(t *testing.T) {
	m := Default()
	m.Apply(map[string][]string{"run": {"f8"}, "quit": {"ctrl+nope"}})

	if got := m.Lookup(key(tcell.KeyF5, tcell.ModNone)); got != ActionRun {
		t.Errorf("F5 = %v, want ActionRun; a rejected Apply must change nothing", got)
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
	for _, a := range []Action{ActionRun, ActionFind, ActionCommandPalette, ActionGoToTable, ActionComplete} {
		if a.Reserved() {
			t.Errorf("%v.Reserved() = true, want false", a)
		}
	}
}
