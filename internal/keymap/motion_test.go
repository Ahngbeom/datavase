package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// macOS sends Option for word movement; everywhere else it is Ctrl. Both have
// to reach the same action or the key silently stops working when the user
// changes platform or terminal.
func TestWordMovementAcceptsAltAndCtrl(t *testing.T) {
	tests := []struct {
		name string
		alt  *tcell.EventKey
		ctrl *tcell.EventKey
		want Action
	}{
		{
			name: "word left",
			alt:  key(tcell.KeyLeft, tcell.ModAlt),
			ctrl: key(tcell.KeyLeft, tcell.ModCtrl),
			want: ActionWordLeft,
		},
		{
			name: "word right",
			alt:  key(tcell.KeyRight, tcell.ModAlt),
			ctrl: key(tcell.KeyRight, tcell.ModCtrl),
			want: ActionWordRight,
		},
		{
			name: "select word left",
			alt:  key(tcell.KeyLeft, tcell.ModAlt|tcell.ModShift),
			ctrl: key(tcell.KeyLeft, tcell.ModCtrl|tcell.ModShift),
			want: ActionSelectWordLeft,
		},
		{
			name: "select word right",
			alt:  key(tcell.KeyRight, tcell.ModAlt|tcell.ModShift),
			ctrl: key(tcell.KeyRight, tcell.ModCtrl|tcell.ModShift),
			want: ActionSelectWordRight,
		},
		{
			name: "delete word left",
			alt:  key(tcell.KeyBackspace2, tcell.ModAlt),
			ctrl: key(tcell.KeyBackspace2, tcell.ModCtrl),
			want: ActionDeleteWordLeft,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookup(t, tt.alt); got != tt.want {
				t.Errorf("Alt form = %v, want %v", got, tt.want)
			}
			if got := lookup(t, tt.ctrl); got != tt.want {
				t.Errorf("Ctrl form = %v, want %v", got, tt.want)
			}
		})
	}
}

// Cursor movement is the one place ⌘ and Ctrl mean different things: on a Mac
// ⌘← goes to the start of the line while Ctrl← moves by word. Collapsing them
// the way every other binding does would break the muscle memory this whole
// keymap exists to serve.
func TestCommandIsLineMovementNotWordMovement(t *testing.T) {
	tests := []struct {
		name string
		ev   *tcell.EventKey
		want Action
	}{
		{name: "cmd left", ev: key(tcell.KeyLeft, tcell.ModMeta), want: ActionLineStart},
		{name: "cmd right", ev: key(tcell.KeyRight, tcell.ModMeta), want: ActionLineEnd},
		{
			name: "cmd shift left",
			ev:   key(tcell.KeyLeft, tcell.ModMeta|tcell.ModShift),
			want: ActionSelectLineStart,
		},
		{
			name: "cmd shift right",
			ev:   key(tcell.KeyRight, tcell.ModMeta|tcell.ModShift),
			want: ActionSelectLineEnd,
		},
		{
			name: "cmd backspace",
			ev:   key(tcell.KeyBackspace2, tcell.ModMeta),
			want: ActionDeleteToLineStart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookup(t, tt.ev); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}

	// And the Ctrl forms must not have been taken over by line movement.
	if got := lookup(t, key(tcell.KeyLeft, tcell.ModCtrl)); got != ActionWordLeft {
		t.Errorf("Ctrl+Left = %v, want ActionWordLeft", got)
	}
}

// A MacBook has no Home key, but everywhere else it is the obvious binding.
func TestHomeAndEndMoveByLine(t *testing.T) {
	tests := []struct {
		name string
		ev   *tcell.EventKey
		want Action
	}{
		{name: "home", ev: key(tcell.KeyHome, tcell.ModNone), want: ActionLineStart},
		{name: "end", ev: key(tcell.KeyEnd, tcell.ModNone), want: ActionLineEnd},
		{
			name: "shift home",
			ev:   key(tcell.KeyHome, tcell.ModShift),
			want: ActionSelectLineStart,
		},
		{
			name: "shift end",
			ev:   key(tcell.KeyEnd, tcell.ModShift),
			want: ActionSelectLineEnd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookup(t, tt.ev); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// Unmodified arrows belong to the editor, which moves one character at a time.
func TestPlainArrowsAreNotActions(t *testing.T) {
	for _, k := range []tcell.Key{tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown} {
		if got := lookup(t, key(k, tcell.ModNone)); got != ActionNone {
			t.Errorf("plain %v = %v, want ActionNone", tcell.KeyNames[k], got)
		}
	}
	// Shift alone extends a character-wise selection, also the editor's job.
	if got := lookup(t, key(tcell.KeyLeft, tcell.ModShift)); got != ActionNone {
		t.Errorf("Shift+Left = %v, want ActionNone", got)
	}
}

// Plain Backspace deletes one character; only the modified forms are ours.
func TestPlainBackspaceIsNotAnAction(t *testing.T) {
	for _, k := range []tcell.Key{tcell.KeyBackspace, tcell.KeyBackspace2} {
		if got := lookup(t, key(k, tcell.ModNone)); got != ActionNone {
			t.Errorf("plain backspace (%v) = %v, want ActionNone", k, got)
		}
	}
}

// Terminals disagree on which of the two backspace keys they send.
func TestBothBackspaceEncodingsResolve(t *testing.T) {
	for _, k := range []tcell.Key{tcell.KeyBackspace, tcell.KeyBackspace2} {
		if got := lookup(t, key(k, tcell.ModAlt)); got != ActionDeleteWordLeft {
			t.Errorf("Alt+backspace (%v) = %v, want ActionDeleteWordLeft", k, got)
		}
	}
}
