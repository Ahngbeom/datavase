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
			// The machine guarantees a count of at least one, and none of
			// these sequences types one. Stated here rather than repeated
			// down a table that is about which command each sequence makes.
			if tt.want.Count == 0 {
				tt.want.Count = 1
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

			// The reference writes the character a find motion takes as
			// "{char}", the way vim's own documentation does. Standing a real
			// one in keeps the entry readable without letting it advertise a
			// sequence that does not complete.
			keys := strings.ReplaceAll(entry.Keys, "{char}", ",")

			s := New()
			_, out := feed(t, s, keys)
			if out != OutcomeExecute {
				t.Errorf("the reference lists %q, but typing it does nothing (%v)", entry.Keys, out)
			}
		}
	}
}

// Search opens a prompt rather than collecting the pattern here.
//
// The pattern belongs to the interface, which owns the field it is typed into
// and the text it is looked for in. Collecting it in this state machine would
// put every key of it through Feed, where an arrow key resolves as a motion
// and consumes a waiting operator — deleting a character in the middle of
// typing a search term.
func TestSearchKeysAskTheInterfaceForAPattern(t *testing.T) {
	tests := []struct {
		keys string
		want Command
	}{
		{"/", Command{Kind: KindSearch}},
		{"?", Command{Kind: KindSearch, Backward: true}},
		{"n", Command{Kind: KindSearchNext}},
		{"N", Command{Kind: KindSearchPrev}},
	}

	for _, tc := range tests {
		t.Run(tc.keys, func(t *testing.T) {
			cmd, out := feed(t, New(), tc.keys)
			if out != OutcomeExecute {
				t.Fatalf("outcome = %v, want %v", out, OutcomeExecute)
			}
			// The machine guarantees a count of at least one on anything it
			// runs; none of these sequences types one.
			if tc.want.Count == 0 {
				tc.want.Count = 1
			}
			if cmd != tc.want {
				t.Errorf("command = %+v, want %+v", cmd, tc.want)
			}
		})
	}
}

// Searching from a selection extends it, so the mode has to survive the key.
func TestSearchDoesNotLeaveVisualMode(t *testing.T) {
	s := New()
	feed(t, s, "v")

	cmd, out := feed(t, s, "/")
	if out != OutcomeExecute || cmd.Kind != KindSearch {
		t.Fatalf("got %+v/%v, want a search command", cmd, out)
	}
	if got := s.Mode(); got != ModeVisual {
		t.Errorf("Mode() = %v after searching, want %v", got, ModeVisual)
	}
}

// "d/" is not supported, but the key must still do something: an operator that
// swallows the next key and leaves no trace is the state this package's
// pending display exists to prevent.
func TestSearchAfterAnOperatorDropsTheOperatorRatherThanTheKey(t *testing.T) {
	s := New()
	feed(t, s, "d")

	cmd, out := feed(t, s, "/")
	if out != OutcomeExecute || cmd.Kind != KindSearch {
		t.Fatalf("got %+v/%v, want the search to open anyway", cmd, out)
	}
	if got := s.Pending(); got != "" {
		t.Errorf("Pending() = %q, want the abandoned operator to be gone", got)
	}

	// And the dropped operator must not reach the next motion.
	cmd, _ = feed(t, s, "w")
	if cmd.Kind != KindMove {
		t.Errorf("the next motion became %v; the operator outlived the search", cmd.Kind)
	}
}

// Insert mode types these characters. A search key that fired while writing a
// comment would be unusable.
func TestSearchKeysAreOrdinaryTextInInsertMode(t *testing.T) {
	s := New()
	feed(t, s, "i")

	for _, key := range []string{"/", "?", "n", "N"} {
		if _, out := feed(t, s, key); out != OutcomePass {
			t.Errorf("%q in insert mode gave %v, want %v", key, out, OutcomePass)
		}
	}
}

// A count is the first thing a vim user's hands reach for, and pressing "d"
// three times instead is the moment the editor stops feeling like vim.
func TestACountRepeatsTheMotion(t *testing.T) {
	tests := []struct {
		keys      string
		wantCount int
		wantWhat  Motion
	}{
		{"3j", 3, MotionDown},
		{"12l", 12, MotionRight},
		{"5w", 5, MotionWordForward},
		// No count typed is one, not zero: the caller multiplies by it.
		{"j", 1, MotionDown},
	}

	for _, tt := range tests {
		t.Run(tt.keys, func(t *testing.T) {
			cmd, out := feed(t, New(), tt.keys)
			if out != OutcomeExecute {
				t.Fatalf("outcome = %v, want OutcomeExecute", out)
			}
			if cmd.Motion != tt.wantWhat {
				t.Errorf("motion = %v, want %v", cmd.Motion, tt.wantWhat)
			}
			if cmd.Count != tt.wantCount {
				t.Errorf("count = %d, want %d", cmd.Count, tt.wantCount)
			}
		})
	}
}

// "0" is the start of the line, and only a digit when something is already
// being counted. Getting this wrong makes the most common motion in the file
// unreachable.
func TestZeroIsTheLineStartUntilACountIsUnderway(t *testing.T) {
	cmd, out := feed(t, New(), "0")
	if out != OutcomeExecute || cmd.Motion != MotionLineStart {
		t.Errorf("bare 0 gave %v/%v, want the line start", out, cmd.Motion)
	}

	cmd, out = feed(t, New(), "10j")
	if out != OutcomeExecute || cmd.Count != 10 {
		t.Errorf("\"10j\" gave count %d (%v), want 10", cmd.Count, out)
	}
}

// The status bar shows what has been half-typed, and a count that does not
// appear there is indistinguishable from a keyboard that has stopped working.
func TestAHalfTypedCountShowsOnTheStatusBar(t *testing.T) {
	s := New()
	feed(t, s, "12")

	if got := s.Pending(); got != "12" {
		t.Errorf("Pending() = %q, want %q", got, "12")
	}
}

// Counts multiply around an operator, as vim's do: "2d3w" is six words.
func TestCountsOnBothSidesOfAnOperatorMultiply(t *testing.T) {
	cmd, out := feed(t, New(), "2d3w")

	if out != OutcomeExecute {
		t.Fatalf("outcome = %v, want OutcomeExecute", out)
	}
	if cmd.Kind != KindDelete || cmd.Motion != MotionWordForward {
		t.Fatalf("got %v/%v, want a word delete", cmd.Kind, cmd.Motion)
	}
	if cmd.Count != 6 {
		t.Errorf("count = %d, want 6", cmd.Count)
	}
}

func TestACountAppliesToALinewiseOperator(t *testing.T) {
	cmd, out := feed(t, New(), "3dd")

	if out != OutcomeExecute {
		t.Fatalf("outcome = %v, want OutcomeExecute", out)
	}
	if cmd.Kind != KindDelete || !cmd.Linewise || cmd.Count != 3 {
		t.Errorf("got kind=%v linewise=%v count=%d, want a 3-line delete",
			cmd.Kind, cmd.Linewise, cmd.Count)
	}
}

// f and t are how anyone moves along a line of SQL — "f," across a column
// list is the motion this editor is used for most.
func TestFindMotionsCarryTheCharacterTheyWereGiven(t *testing.T) {
	tests := []struct {
		keys string
		want Motion
	}{
		{"f,", MotionFindForward},
		{"t,", MotionTillForward},
		{"F,", MotionFindBackward},
		{"T,", MotionTillBackward},
	}

	for _, tt := range tests {
		t.Run(tt.keys, func(t *testing.T) {
			cmd, out := feed(t, New(), tt.keys)
			if out != OutcomeExecute {
				t.Fatalf("outcome = %v, want OutcomeExecute", out)
			}
			if cmd.Motion != tt.want {
				t.Errorf("motion = %v, want %v", cmd.Motion, tt.want)
			}
			if cmd.Target != ',' {
				t.Errorf("target = %q, want %q", cmd.Target, ',')
			}
		})
	}
}

// Waiting for the character is a pending state like any other, and the bar
// has to say so rather than looking like nothing happened.
func TestFindWaitsForItsCharacterAndSaysSo(t *testing.T) {
	s := New()

	if _, out := feed(t, s, "f"); out != OutcomePending {
		t.Fatalf("outcome after f = %v, want OutcomePending", out)
	}
	if got := s.Pending(); got != "f" {
		t.Errorf("Pending() = %q, want %q", got, "f")
	}
}

// The whole point of f as an operator motion: "df," deletes up to the comma.
func TestAnOperatorTakesAFindMotion(t *testing.T) {
	cmd, out := feed(t, New(), "df,")

	if out != OutcomeExecute {
		t.Fatalf("outcome = %v, want OutcomeExecute", out)
	}
	if cmd.Kind != KindDelete || cmd.Motion != MotionFindForward || cmd.Target != ',' {
		t.Errorf("got kind=%v motion=%v target=%q, want a delete to the comma",
			cmd.Kind, cmd.Motion, cmd.Target)
	}
}

// Escape has to clear a half-typed count too, or the next motion silently
// carries a multiplier nobody can see.
func TestEscapeClearsAHalfTypedCount(t *testing.T) {
	s := New()
	feed(t, s, "12")
	feed(t, s, "<esc>")

	if got := s.Pending(); got != "" {
		t.Errorf("Pending() = %q after Escape, want empty", got)
	}
	cmd, _ := feed(t, s, "j")
	if cmd.Count != 1 {
		t.Errorf("count = %d after Escape, want 1", cmd.Count)
	}
}

// ci( replaces an IN list, ci' a string literal. These are the sequences a
// SQL editor is reached for most, and none of them existed.
func TestAnOperatorTakesATextObject(t *testing.T) {
	tests := []struct {
		keys       string
		wantKind   Kind
		wantObject Object
		wantAround bool
	}{
		{"diw", KindDelete, ObjectWord, false},
		{"daw", KindDelete, ObjectWord, true},
		{"ci(", KindChange, ObjectParen, false},
		{"ca(", KindChange, ObjectParen, true},
		{"ci)", KindChange, ObjectParen, false},
		{"yi'", KindYank, ObjectSingleQuote, false},
		{`di"`, KindDelete, ObjectDoubleQuote, false},
		{"di[", KindDelete, ObjectBracket, false},
		{"di{", KindDelete, ObjectBrace, false},
	}

	for _, tt := range tests {
		t.Run(tt.keys, func(t *testing.T) {
			cmd, out := feed(t, New(), tt.keys)
			if out != OutcomeExecute {
				t.Fatalf("outcome = %v, want OutcomeExecute", out)
			}
			if cmd.Kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", cmd.Kind, tt.wantKind)
			}
			if cmd.Object != tt.wantObject {
				t.Errorf("object = %v, want %v", cmd.Object, tt.wantObject)
			}
			if cmd.Around != tt.wantAround {
				t.Errorf("around = %v, want %v", cmd.Around, tt.wantAround)
			}
		})
	}
}

// "i" and "a" are insert and append with no operator waiting, and a text
// object only in the middle of one. Getting this wrong would cost the two
// most-used keys in the editor.
func TestIAndAAreStillInsertAndAppendOnTheirOwn(t *testing.T) {
	for _, tt := range []struct {
		keys string
		want Place
	}{
		{"i", PlaceBefore},
		{"a", PlaceAfter},
	} {
		t.Run(tt.keys, func(t *testing.T) {
			cmd, out := feed(t, New(), tt.keys)
			if out != OutcomeExecute {
				t.Fatalf("outcome = %v, want OutcomeExecute", out)
			}
			if cmd.Kind != KindInsert || cmd.At != tt.want {
				t.Errorf("got kind=%v at=%v, want an insert at %v", cmd.Kind, cmd.At, tt.want)
			}
		})
	}
}

// The half-typed sequence has to reach the status bar, or a waiting operator
// is indistinguishable from a keyboard that stopped working.
func TestAHalfTypedTextObjectShowsOnTheStatusBar(t *testing.T) {
	s := New()
	if _, out := feed(t, s, "di"); out != OutcomePending {
		t.Fatalf("outcome after \"di\" = %v, want OutcomePending", out)
	}
	if got := s.Pending(); got != "di" {
		t.Errorf("Pending() = %q, want %q", got, "di")
	}
}

// A character that names no object abandons the sequence rather than doing
// something else with it.
func TestATextObjectNobodyKnowsAbandonsTheOperator(t *testing.T) {
	s := New()
	if _, out := feed(t, s, "diz"); out == OutcomeExecute {
		t.Error("\"diz\" ran something; z names no text object")
	}
	if got := s.Pending(); got != "" {
		t.Errorf("Pending() = %q after an unknown object, want empty", got)
	}
}
