package vim

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// feed sends a key sequence and reports the last outcome.
//
// Sequences are written the way they are typed — "dw", "gg" — because that is
// how the behaviour is described in every vim reference, and a test that
// spells them as event structs is a test nobody can check against one.
func feed(t *testing.T, s *State, keys string) (Command, Outcome) {
	t.Helper()

	var (
		cmd Command
		out Outcome
	)
	for _, ev := range events(t, keys) {
		cmd, out = s.Feed(ev)
	}
	return cmd, out
}

// events parses a key sequence. "<esc>" and "<c-r>" name the keys that have
// no printable spelling.
func events(t *testing.T, keys string) []*tcell.EventKey {
	t.Helper()

	var out []*tcell.EventKey
	for i := 0; i < len(keys); i++ {
		if keys[i] != '<' {
			out = append(out, tcell.NewEventKey(tcell.KeyRune, rune(keys[i]), tcell.ModNone))
			continue
		}

		end := strings.IndexByte(keys[i:], '>')
		if end < 0 {
			t.Fatalf("unterminated key name in %q", keys)
		}
		name := keys[i+1 : i+end]
		i += end

		switch name {
		case "esc":
			out = append(out, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
		case "c-r":
			out = append(out, tcell.NewEventKey(tcell.KeyCtrlR, 'r', tcell.ModCtrl))
		case "cr":
			out = append(out, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		default:
			t.Fatalf("unknown key name %q", name)
		}
	}
	return out
}

func TestStartsInNormalMode(t *testing.T) {
	if got := New().Mode(); got != ModeNormal {
		t.Errorf("Mode() = %v, want %v", got, ModeNormal)
	}
}

// Motions and the operators that take them. Each row is one complete
// sequence typed from a fresh normal mode.
func TestNormalModeSequences(t *testing.T) {
	tests := []struct {
		keys string
		want Command
	}{
		// Motions.
		{"h", Command{Kind: KindMove, Motion: MotionLeft}},
		{"j", Command{Kind: KindMove, Motion: MotionDown}},
		{"k", Command{Kind: KindMove, Motion: MotionUp}},
		{"l", Command{Kind: KindMove, Motion: MotionRight}},
		{"w", Command{Kind: KindMove, Motion: MotionWordForward}},
		{"b", Command{Kind: KindMove, Motion: MotionWordBackward}},
		{"e", Command{Kind: KindMove, Motion: MotionWordEnd}},
		{"0", Command{Kind: KindMove, Motion: MotionLineStart}},
		{"^", Command{Kind: KindMove, Motion: MotionFirstNonBlank}},
		{"$", Command{Kind: KindMove, Motion: MotionLineEnd}},
		{"gg", Command{Kind: KindMove, Motion: MotionFileStart}},
		{"G", Command{Kind: KindMove, Motion: MotionFileEnd}},
		// On their own those same motions just move; nothing is linewise
		// until an operator is waiting for them.

		// Operators taking a motion.
		{"dw", Command{Kind: KindDelete, Motion: MotionWordForward}},
		{"db", Command{Kind: KindDelete, Motion: MotionWordBackward}},
		{"d$", Command{Kind: KindDelete, Motion: MotionLineEnd}},
		{"cw", Command{Kind: KindChange, Motion: MotionWordForward}},
		{"yw", Command{Kind: KindYank, Motion: MotionWordForward}},
		// Motions that span lines make their operator linewise, which is why
		// dG removes whole lines rather than half of the first one.
		{"dgg", Command{Kind: KindDelete, Motion: MotionFileStart, Linewise: true}},
		{"dG", Command{Kind: KindDelete, Motion: MotionFileEnd, Linewise: true}},
		{"dj", Command{Kind: KindDelete, Motion: MotionDown, Linewise: true}},
		{"yk", Command{Kind: KindYank, Motion: MotionUp, Linewise: true}},

		// Doubled operators act on whole lines.
		{"dd", Command{Kind: KindDelete, Linewise: true}},
		{"cc", Command{Kind: KindChange, Linewise: true}},
		{"yy", Command{Kind: KindYank, Linewise: true}},

		// Shorthands.
		{"x", Command{Kind: KindDelete, Motion: MotionRight}},
		{"D", Command{Kind: KindDelete, Motion: MotionLineEnd}},
		{"C", Command{Kind: KindChange, Motion: MotionLineEnd}},
		{"Y", Command{Kind: KindYank, Linewise: true}},

		// Entering insert mode.
		{"i", Command{Kind: KindInsert, At: PlaceBefore}},
		{"a", Command{Kind: KindInsert, At: PlaceAfter}},
		{"I", Command{Kind: KindInsert, At: PlaceLineStart}},
		{"A", Command{Kind: KindInsert, At: PlaceLineEnd}},
		{"o", Command{Kind: KindInsert, At: PlaceOpenBelow}},
		{"O", Command{Kind: KindInsert, At: PlaceOpenAbove}},

		// Putting and undoing.
		{"p", Command{Kind: KindPaste, At: PlaceAfter}},
		{"P", Command{Kind: KindPaste, At: PlaceBefore}},
		{"u", Command{Kind: KindUndo}},
		{"<c-r>", Command{Kind: KindRedo}},
	}

	for _, tt := range tests {
		t.Run(tt.keys, func(t *testing.T) {
			s := New()

			cmd, out := feed(t, s, tt.keys)
			if out != OutcomeExecute {
				t.Fatalf("Feed(%q) outcome = %v, want %v", tt.keys, out, OutcomeExecute)
			}
			if cmd != tt.want {
				t.Errorf("Feed(%q) = %+v, want %+v", tt.keys, cmd, tt.want)
			}
		})
	}
}

// Insert mode is the only mode that lets a key through to the editor. In
// normal mode a stray letter must never become text — that is the failure
// people mean when they say "the keyboard stopped working".
func TestNormalModeSwallowsEveryKey(t *testing.T) {
	s := New()

	for _, keys := range []string{"z", "q", "!", "5"} {
		_, out := feed(t, s, keys)
		if out == OutcomePass {
			t.Errorf("Feed(%q) outcome = %v, want the key swallowed", keys, out)
		}
		if s.Mode() != ModeNormal {
			t.Fatalf("Feed(%q) left the state in %v", keys, s.Mode())
		}
	}
}

func TestInsertModePassesKeysThrough(t *testing.T) {
	s := New()

	if _, out := feed(t, s, "i"); out != OutcomeExecute {
		t.Fatalf("i did not enter insert mode")
	}
	if s.Mode() != ModeInsert {
		t.Fatalf("Mode() = %v, want %v", s.Mode(), ModeInsert)
	}

	for _, keys := range []string{"d", "w", "j", "x"} {
		if _, out := feed(t, s, keys); out != OutcomePass {
			t.Errorf("Feed(%q) in insert mode outcome = %v, want %v", keys, out, OutcomePass)
		}
	}
}

// Escape has to be the way out of everything, including a half-typed
// sequence. Without that, a mistyped "d" leaves the next keystroke doing
// something the user never asked for.
func TestEscapeAlwaysReturnsToNormal(t *testing.T) {
	tests := []string{"i", "d", "v", "V", "c", "y", "g"}

	for _, keys := range tests {
		t.Run(keys, func(t *testing.T) {
			s := New()
			feed(t, s, keys)

			cmd, out := feed(t, s, "<esc>")
			if out != OutcomeExecute || cmd.Kind != KindEscape {
				t.Errorf("Escape after %q = %+v/%v, want an escape command", keys, cmd, out)
			}
			if s.Mode() != ModeNormal {
				t.Errorf("Mode() = %v after escape, want %v", s.Mode(), ModeNormal)
			}
			if s.Pending() != "" {
				t.Errorf("Pending() = %q after escape, want it cleared", s.Pending())
			}
		})
	}
}

// A half-typed sequence has to be visible, or "I pressed d and nothing
// happened" is the whole experience.
func TestPendingReportsAHalfTypedSequence(t *testing.T) {
	tests := []struct {
		keys string
		want string
	}{
		{"", ""},
		{"d", "d"},
		{"g", "g"},
		{"c", "c"},
		{"dd", ""},
		{"dw", ""},
	}

	for _, tt := range tests {
		t.Run(tt.keys, func(t *testing.T) {
			s := New()
			feed(t, s, tt.keys)

			if got := s.Pending(); got != tt.want {
				t.Errorf("Pending() after %q = %q, want %q", tt.keys, got, tt.want)
			}
		})
	}
}

func TestOperatorWaitsForItsMotion(t *testing.T) {
	s := New()

	if _, out := feed(t, s, "d"); out != OutcomePending {
		t.Errorf("Feed(\"d\") outcome = %v, want %v", out, OutcomePending)
	}
}

// A key that cannot finish a sequence abandons it rather than waiting
// forever or applying to the wrong thing.
func TestAnUnknownMotionAbandonsTheOperator(t *testing.T) {
	s := New()

	feed(t, s, "d")
	if _, out := feed(t, s, "z"); out == OutcomeExecute {
		t.Error("dz produced a command, want the sequence abandoned")
	}
	if s.Pending() != "" {
		t.Errorf("Pending() = %q, want the sequence abandoned", s.Pending())
	}

	// And the state is usable again straight away.
	cmd, out := feed(t, s, "w")
	if out != OutcomeExecute || cmd.Kind != KindMove {
		t.Errorf("after abandoning, w = %+v/%v, want a plain motion", cmd, out)
	}
}

func TestVisualMode(t *testing.T) {
	s := New()

	cmd, out := feed(t, s, "v")
	if out != OutcomeExecute || cmd.Kind != KindVisual || cmd.Linewise {
		t.Fatalf("v = %+v/%v, want a charwise visual command", cmd, out)
	}
	if s.Mode() != ModeVisual {
		t.Fatalf("Mode() = %v, want %v", s.Mode(), ModeVisual)
	}

	// Motions extend the selection rather than starting a new one.
	cmd, out = feed(t, s, "w")
	if out != OutcomeExecute || cmd.Kind != KindMove || cmd.Motion != MotionWordForward {
		t.Errorf("w in visual mode = %+v/%v, want a motion", cmd, out)
	}
	if s.Mode() != ModeVisual {
		t.Errorf("Mode() = %v after a motion, want to stay in visual", s.Mode())
	}

	// An operator applies to the selection and ends the mode.
	cmd, out = feed(t, s, "d")
	if out != OutcomeExecute || cmd.Kind != KindDelete || !cmd.Selection {
		t.Errorf("d in visual mode = %+v/%v, want a delete of the selection", cmd, out)
	}
	if s.Mode() != ModeNormal {
		t.Errorf("Mode() = %v after an operator, want %v", s.Mode(), ModeNormal)
	}
}

func TestVisualLineMode(t *testing.T) {
	s := New()

	cmd, _ := feed(t, s, "V")
	if cmd.Kind != KindVisual || !cmd.Linewise {
		t.Fatalf("V = %+v, want a linewise visual command", cmd)
	}
	if s.Mode() != ModeVisualLine {
		t.Fatalf("Mode() = %v, want %v", s.Mode(), ModeVisualLine)
	}

	cmd, _ = feed(t, s, "y")
	if cmd.Kind != KindYank || !cmd.Selection || !cmd.Linewise {
		t.Errorf("y in visual line mode = %+v, want a linewise yank of the selection", cmd)
	}
}

// Change from visual mode has to leave the user typing, exactly as it does
// from normal mode.
func TestChangeFromVisualModeEntersInsert(t *testing.T) {
	s := New()

	feed(t, s, "v")
	cmd, _ := feed(t, s, "c")

	if cmd.Kind != KindChange || !cmd.Selection {
		t.Fatalf("c in visual mode = %+v, want a change of the selection", cmd)
	}
	if s.Mode() != ModeInsert {
		t.Errorf("Mode() = %v, want %v", s.Mode(), ModeInsert)
	}
}

// Toggling the same visual mode leaves it; switching to the other one stays.
func TestVisualModeToggles(t *testing.T) {
	s := New()

	feed(t, s, "v")
	feed(t, s, "v")
	if s.Mode() != ModeNormal {
		t.Errorf("v twice left mode %v, want %v", s.Mode(), ModeNormal)
	}

	feed(t, s, "v")
	feed(t, s, "V")
	if s.Mode() != ModeVisualLine {
		t.Errorf("v then V left mode %v, want %v", s.Mode(), ModeVisualLine)
	}
}

// After an operator that enters insert mode, the state has to say so — the
// editor asks it what to do with the next key.
func TestChangeEntersInsertMode(t *testing.T) {
	for _, keys := range []string{"cw", "cc", "C"} {
		t.Run(keys, func(t *testing.T) {
			s := New()
			feed(t, s, keys)

			if s.Mode() != ModeInsert {
				t.Errorf("Mode() after %q = %v, want %v", keys, s.Mode(), ModeInsert)
			}
		})
	}
}

func TestDeleteDoesNotEnterInsertMode(t *testing.T) {
	for _, keys := range []string{"dw", "dd", "x", "D"} {
		t.Run(keys, func(t *testing.T) {
			s := New()
			feed(t, s, keys)

			if s.Mode() != ModeNormal {
				t.Errorf("Mode() after %q = %v, want %v", keys, s.Mode(), ModeNormal)
			}
		})
	}
}

// The mode is shown in the status bar, so it needs a name a user recognises.
func TestModeNames(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeNormal, "NORMAL"},
		{ModeInsert, "INSERT"},
		{ModeVisual, "VISUAL"},
		{ModeVisualLine, "V-LINE"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

// The reference is what the help screen and `dv keys` show. Every sequence it
// lists is typed into a fresh state here, so a key that stops working — or
// one that is renamed — cannot go on being advertised.
func TestReferenceOnlyListsKeysThatWork(t *testing.T) {
	groups := Reference()
	if len(groups) == 0 {
		t.Fatal("Reference() is empty")
	}

	for _, group := range groups {
		if group.Title == "" {
			t.Error("a reference group has no title")
		}
		if len(group.Entries) == 0 {
			t.Errorf("group %q has no entries", group.Title)
		}

		for _, entry := range group.Entries {
			if entry.Description == "" {
				t.Errorf("%q has no description", entry.Keys)
			}

			s := New()
			_, out := feed(t, s, entry.Keys)
			if out != OutcomeExecute {
				t.Errorf("the reference lists %q, but typing it does nothing (%v)", entry.Keys, out)
			}
		}
	}
}
