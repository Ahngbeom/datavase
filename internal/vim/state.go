package vim

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// State is the modal input state: the current mode and any half-typed
// sequence.
//
// It is not safe for concurrent use, which is fine — keys arrive one at a
// time on the interface's own goroutine.
type State struct {
	mode Mode
	// op is an operator waiting for its motion: d, c or y.
	op Kind
	// gPrefix records that "g" has been typed and is waiting for the rest.
	gPrefix bool
}

// New returns a state in normal mode, which is where vim starts.
func New() *State { return &State{mode: ModeNormal} }

// Mode reports the current input mode.
func (s *State) Mode() Mode { return s.mode }

// Pending renders the half-typed sequence for the status bar.
//
// Showing it is not a nicety: an operator that silently waits for a motion is
// indistinguishable from a keyboard that has stopped working, and that is the
// single most likely way a new user concludes the application is broken.
func (s *State) Pending() string {
	var b strings.Builder

	switch s.op {
	case KindDelete:
		b.WriteByte('d')
	case KindChange:
		b.WriteByte('c')
	case KindYank:
		b.WriteByte('y')
	}
	if s.gPrefix {
		b.WriteByte('g')
	}
	return b.String()
}

// motionKeys are the keys that move, whether or not an operator is waiting.
var motionKeys = map[rune]Motion{
	'h': MotionLeft,
	'l': MotionRight,
	'k': MotionUp,
	'j': MotionDown,
	'w': MotionWordForward,
	'b': MotionWordBackward,
	'e': MotionWordEnd,
	'0': MotionLineStart,
	'^': MotionFirstNonBlank,
	'$': MotionLineEnd,
	'G': MotionFileEnd,
}

// Feed offers a key to the state machine.
//
// The outcome says whether the editor should type the key, keep waiting, or
// run the returned command.
func (s *State) Feed(ev *tcell.EventKey) (Command, Outcome) {
	// Escape is the way out of everything, from every mode, including a
	// sequence that was only half typed.
	if ev.Key() == tcell.KeyEscape {
		s.reset()
		return Command{Kind: KindEscape}, OutcomeExecute
	}

	// Insert mode is the only mode that lets a key reach the editor.
	if s.mode == ModeInsert {
		return Command{}, OutcomePass
	}

	if ev.Key() == tcell.KeyCtrlR {
		s.clearPending()
		return Command{Kind: KindRedo}, OutcomeExecute
	}

	// Arrow keys work in every mode, because a key that moves the cursor
	// everywhere else in the interface should not stop doing so here.
	if m, ok := arrowMotion(ev.Key()); ok {
		return s.applyMotion(m)
	}

	if ev.Key() != tcell.KeyRune {
		// Swallowed rather than passed on: in normal mode a stray key must
		// never reach the editor and become text.
		return Command{}, OutcomePending
	}
	return s.feedRune(ev.Rune())
}

func arrowMotion(key tcell.Key) (Motion, bool) {
	switch key {
	case tcell.KeyLeft:
		return MotionLeft, true
	case tcell.KeyRight:
		return MotionRight, true
	case tcell.KeyUp:
		return MotionUp, true
	case tcell.KeyDown:
		return MotionDown, true
	default:
		return MotionNone, false
	}
}

func (s *State) feedRune(r rune) (Command, Outcome) {
	// "g" is a prefix in its own right, and it can follow an operator: "dgg"
	// deletes to the top of the file.
	if s.gPrefix {
		s.gPrefix = false
		if r == 'g' {
			return s.applyMotion(MotionFileStart)
		}
		return s.abandon()
	}
	if r == 'g' {
		s.gPrefix = true
		return Command{}, OutcomePending
	}

	if m, ok := motionKeys[r]; ok {
		return s.applyMotion(m)
	}

	// Search reaches the interface from normal mode, from visual mode and from
	// the middle of a half-typed operator alike. It goes before the operator
	// block below, which would otherwise abandon the sequence and swallow this
	// key along with it, leaving "/" looking like a key that stopped working.
	// The operator is dropped, because searching moves the cursor rather than
	// giving an operator something to reach.
	if cmd, ok := searchCommand(r); ok {
		s.clearPending()
		return cmd, OutcomeExecute
	}

	// An operator is waiting: only its own letter, meaning "this whole line",
	// can still complete it.
	if s.op != KindNone {
		if r == operatorRune(s.op) {
			op := s.op
			s.op = KindNone
			if op == KindChange {
				s.mode = ModeInsert
			}
			return Command{Kind: op, Linewise: true}, OutcomeExecute
		}
		return s.abandon()
	}

	if s.mode == ModeVisual || s.mode == ModeVisualLine {
		return s.feedVisual(r)
	}
	return s.feedNormal(r)
}

// searchCommand maps the keys that start or repeat a search.
func searchCommand(r rune) (Command, bool) {
	switch r {
	case '/':
		return Command{Kind: KindSearch}, true
	case '?':
		return Command{Kind: KindSearch, Backward: true}, true
	case 'n':
		return Command{Kind: KindSearchNext}, true
	case 'N':
		return Command{Kind: KindSearchPrev}, true
	default:
		return Command{}, false
	}
}

// feedNormal handles the keys that are not motions and not operator
// completions.
func (s *State) feedNormal(r rune) (Command, Outcome) {
	switch r {
	// Operators, which now wait for a motion.
	case 'd':
		s.op = KindDelete
		return Command{}, OutcomePending
	case 'c':
		s.op = KindChange
		return Command{}, OutcomePending
	case 'y':
		s.op = KindYank
		return Command{}, OutcomePending

	// Shorthands for an operator and a motion typed together.
	case 'x':
		return Command{Kind: KindDelete, Motion: MotionRight}, OutcomeExecute
	case 'D':
		return Command{Kind: KindDelete, Motion: MotionLineEnd}, OutcomeExecute
	case 'C':
		s.mode = ModeInsert
		return Command{Kind: KindChange, Motion: MotionLineEnd}, OutcomeExecute
	case 'Y':
		return Command{Kind: KindYank, Linewise: true}, OutcomeExecute

	case 'i':
		return s.insertAt(PlaceBefore)
	case 'a':
		return s.insertAt(PlaceAfter)
	case 'I':
		return s.insertAt(PlaceLineStart)
	case 'A':
		return s.insertAt(PlaceLineEnd)
	case 'o':
		return s.insertAt(PlaceOpenBelow)
	case 'O':
		return s.insertAt(PlaceOpenAbove)

	case 'p':
		return Command{Kind: KindPaste, At: PlaceAfter}, OutcomeExecute
	case 'P':
		return Command{Kind: KindPaste, At: PlaceBefore}, OutcomeExecute
	case 'u':
		return Command{Kind: KindUndo}, OutcomeExecute

	case 'v':
		return s.toggleVisual(ModeVisual)
	case 'V':
		return s.toggleVisual(ModeVisualLine)
	}

	// Everything else is swallowed. Normal mode consumes every key it does
	// not understand rather than letting it through to be typed.
	return Command{}, OutcomePending
}

// feedVisual handles the keys that act on a selection.
func (s *State) feedVisual(r rune) (Command, Outcome) {
	linewise := s.mode == ModeVisualLine

	switch r {
	case 'd', 'x':
		s.mode = ModeNormal
		return Command{Kind: KindDelete, Selection: true, Linewise: linewise}, OutcomeExecute
	case 'c':
		s.mode = ModeInsert
		return Command{Kind: KindChange, Selection: true, Linewise: linewise}, OutcomeExecute
	case 'y':
		s.mode = ModeNormal
		return Command{Kind: KindYank, Selection: true, Linewise: linewise}, OutcomeExecute

	case 'v':
		return s.toggleVisual(ModeVisual)
	case 'V':
		return s.toggleVisual(ModeVisualLine)
	}
	return Command{}, OutcomePending
}

// applyMotion turns a motion into a movement, or into the operator that was
// waiting for it.
func (s *State) applyMotion(m Motion) (Command, Outcome) {
	if s.op == KindNone {
		return Command{Kind: KindMove, Motion: m}, OutcomeExecute
	}

	op := s.op
	s.op = KindNone
	if op == KindChange {
		s.mode = ModeInsert
	}
	return Command{Kind: op, Motion: m, Linewise: spansLines(m)}, OutcomeExecute
}

// spansLines reports whether a motion makes its operator act on whole lines.
//
// This is why "dG" clears to the end of the file rather than leaving the
// first line's leading half behind, and why "dj" takes two whole lines. As a
// plain movement the same motions are not linewise — nothing is, until an
// operator is waiting for one.
func spansLines(m Motion) bool {
	switch m {
	case MotionUp, MotionDown, MotionFileStart, MotionFileEnd:
		return true
	default:
		return false
	}
}

func (s *State) insertAt(at Place) (Command, Outcome) {
	s.mode = ModeInsert
	return Command{Kind: KindInsert, At: at}, OutcomeExecute
}

// toggleVisual enters a visual mode, or leaves it when it is already the one
// in force — which is how vim's own v and V behave.
func (s *State) toggleVisual(mode Mode) (Command, Outcome) {
	if s.mode == mode {
		s.mode = ModeNormal
		return Command{Kind: KindEscape}, OutcomeExecute
	}

	s.mode = mode
	return Command{Kind: KindVisual, Linewise: mode == ModeVisualLine}, OutcomeExecute
}

// abandon drops a sequence that cannot be completed.
//
// Dropping it beats both alternatives: waiting forever leaves the user stuck,
// and applying the operator to whatever comes next deletes something they did
// not ask to delete.
func (s *State) abandon() (Command, Outcome) {
	s.clearPending()
	return Command{}, OutcomePending
}

func (s *State) clearPending() {
	s.op = KindNone
	s.gPrefix = false
}

func (s *State) reset() {
	s.clearPending()
	s.mode = ModeNormal
}

func operatorRune(op Kind) rune {
	switch op {
	case KindDelete:
		return 'd'
	case KindChange:
		return 'c'
	case KindYank:
		return 'y'
	default:
		return 0
	}
}
