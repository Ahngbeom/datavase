// Package vim is the modal input model, as a pure state machine.
//
// It knows nothing about editors, terminals or SQL: keys go in, commands come
// out. Keeping it that way is what makes the interesting part — which
// half-typed sequences mean what — testable without a screen.
//
// The scope is deliberately the practical subset: normal, insert and visual
// modes with the common motions, operators and edits. Counts (3dd), search
// (/), marks, named registers and dot-repeat are out.
package vim

// Kind is what a command does.
type Kind int

const (
	// KindNone is the zero command, returned alongside any outcome that is
	// not OutcomeExecute.
	KindNone Kind = iota
	KindMove
	KindDelete
	KindChange
	KindYank
	KindPaste
	KindUndo
	KindRedo
	// KindInsert enters insert mode at At.
	KindInsert
	// KindVisual starts or ends a selection; Linewise says which kind.
	KindVisual
	// KindEscape returns to normal mode, collapsing any selection.
	KindEscape
)

// Motion is where a command reaches.
type Motion int

const (
	MotionNone Motion = iota
	MotionLeft
	MotionRight
	MotionUp
	MotionDown
	MotionWordForward
	MotionWordBackward
	MotionWordEnd
	MotionLineStart
	MotionFirstNonBlank
	MotionLineEnd
	MotionFileStart
	MotionFileEnd
)

// Place is where an insertion or a put happens.
type Place int

const (
	PlaceNone Place = iota
	PlaceBefore
	PlaceAfter
	PlaceLineStart
	PlaceLineEnd
	PlaceOpenBelow
	PlaceOpenAbove
)

// Command is a completed instruction for the editor.
//
// It is a comparable value with no pointers, so tests can state the whole
// expected command in one literal.
type Command struct {
	Kind   Kind
	Motion Motion
	// At is where an insert or a put lands.
	At Place
	// Linewise means whole lines: dd, yy, V.
	Linewise bool
	// Selection means the operator applies to the visual selection already on
	// screen rather than to a motion from the cursor.
	Selection bool
}

// Outcome says what the state machine did with a key.
//
// Three outcomes rather than a bool: "the editor should type this" and "I am
// waiting for the rest of a sequence" are different things, and collapsing
// them is how a half-typed operator ends up inserting a letter.
type Outcome int

const (
	// OutcomePass hands the key to the editor. Only insert mode does this.
	OutcomePass Outcome = iota
	// OutcomePending consumes the key while a sequence is incomplete.
	OutcomePending
	// OutcomeExecute consumes the key and yields a command.
	OutcomeExecute
)

func (o Outcome) String() string {
	switch o {
	case OutcomePass:
		return "pass"
	case OutcomePending:
		return "pending"
	case OutcomeExecute:
		return "execute"
	default:
		return "unknown"
	}
}

// Mode is the input mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeVisual
	ModeVisualLine
)

// String names the mode as the status bar shows it.
func (m Mode) String() string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	case ModeVisualLine:
		return "V-LINE"
	default:
		return "NORMAL"
	}
}
