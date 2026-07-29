package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// In ASCII, Tab is Ctrl+I, Enter is Ctrl+M and Escape is Ctrl+[ — the same
// byte. Folding control codes back to letters must skip these three, or
// pressing Tab would fire whatever Ctrl+I is bound to. Nothing binds those
// combinations today, which is exactly why this needs pinning: the breakage
// would appear later, far from the change that caused it.
func TestKeysThatShareAControlCodeAreNotFolded(t *testing.T) {
	tests := []struct {
		name string
		ev   *tcell.EventKey
		want Binding
	}{
		{
			name: "Tab stays Tab, not Ctrl+I",
			ev:   tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone),
			want: Binding{Key: tcell.KeyTab},
		},
		{
			name: "Enter stays Enter, not Ctrl+M",
			ev:   tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
			want: Binding{Key: tcell.KeyEnter},
		},
		{
			name: "Escape stays Escape, not Ctrl+[",
			ev:   tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
			want: Binding{Key: tcell.KeyEscape},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalize(tt.ev); got != tt.want {
				t.Errorf("normalize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Tab must keep moving focus and Enter must keep inserting a newline.
func TestTabAndEnterKeepTheirOwnActions(t *testing.T) {
	m := Default()

	if got := m.Lookup(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)); got != ActionNextPane {
		t.Errorf("Tab = %v, want ActionNextPane", got)
	}
	if got := m.Lookup(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); got != ActionNone {
		t.Errorf("Enter = %v, want ActionNone so the editor inserts a newline", got)
	}
	if got := m.Lookup(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != ActionNone {
		t.Errorf("Escape = %v, want ActionNone", got)
	}
}

// Shift only matters where a binding asks for it; a capital letter typed with
// Shift must not resolve to the unshifted action.
func TestShiftIsPartOfTheBinding(t *testing.T) {
	m := Default()

	if got := m.Lookup(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModCtrl)); got != ActionSelectAll {
		t.Fatalf("Ctrl+A = %v, want ActionSelectAll", got)
	}
	if got := m.Lookup(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModCtrl|tcell.ModShift)); got != ActionCommandPalette {
		t.Errorf("Ctrl+Shift+A = %v, want ActionCommandPalette", got)
	}
}

// Terminals differ on whether a Ctrl'd letter arrives upper- or lower-case.
func TestCaseDoesNotChangeTheAction(t *testing.T) {
	m := Default()

	lower := m.Lookup(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModCtrl))
	upper := m.Lookup(tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModCtrl))

	if lower != ActionDuplicateLine {
		t.Fatalf("Ctrl+d = %v, want ActionDuplicateLine", lower)
	}
	if upper != lower {
		t.Errorf("Ctrl+D = %v, want the same action as Ctrl+d (%v)", upper, lower)
	}
}

// The Ctrl+J fallback exists for terminals that cannot report Ctrl+Enter.
func TestCtrlJRunsTheStatement(t *testing.T) {
	m := Default()

	if got := m.Lookup(tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl)); got != ActionRun {
		t.Errorf("Ctrl+J = %v, want ActionRun", got)
	}
}

// No two actions may claim the same binding: the second registration would
// silently win and one of the keys would appear broken.
func TestDefaultBindingsDoNotCollide(t *testing.T) {
	m := Default()
	seen := make(map[Binding]Action)

	for _, a := range AllActions() {
		for _, b := range m.Bindings(a) {
			if other, taken := seen[b]; taken && other != a {
				t.Errorf("binding %s is claimed by both %v and %v", b.Label(false), other, a)
			}
			seen[b] = a
		}
	}
}

// Every registered binding must survive a round trip through an event, or
// the table and the lookup path disagree.
func TestEveryDefaultBindingResolvesToItsAction(t *testing.T) {
	m := Default()

	for _, a := range AllActions() {
		for _, b := range m.Bindings(a) {
			if got := m.Lookup(b.Event()); got != a {
				t.Errorf("Lookup(%s) = %v, want %v", b.Label(false), got, a)
			}
		}
	}
}

// Bindings are ordered for the help screen: ⌘ first, then Ctrl, then the
// bare-function-key fallbacks.
func TestBindingsAreOrderedForDisplay(t *testing.T) {
	bindings := Default().Bindings(ActionRun)
	if len(bindings) < 3 {
		t.Fatalf("ActionRun has %d bindings, want at least 3", len(bindings))
	}

	if bindings[0].Mods&tcell.ModMeta == 0 {
		t.Errorf("first binding is %s, want the ⌘ form first", bindings[0].Label(false))
	}
	last := bindings[len(bindings)-1]
	if last.Mods != tcell.ModNone {
		t.Errorf("last binding is %s, want a bare fallback key last", last.Label(false))
	}
}
