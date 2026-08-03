// Package vim is the modal input model, as a pure state machine.
//
// It knows nothing about editors, terminals or SQL: keys go in, commands come
// out. Keeping it that way is what makes the interesting part — which
// half-typed sequences mean what — testable without a screen.
//
// The scope is deliberately the practical subset: normal, insert and visual
// modes with the common motions, operators and edits. Counts (3dd), marks,
// named registers and dot-repeat are out.
//
// Search is here only as far as the key that starts it. The pattern is typed
// into the interface, which owns both the field it goes in and the text it is
// looked for in; carrying it through Feed would put every keystroke of a
// search term past the arrow-key and operator handling below, where it would
// be read as a motion. That also means "d/foo" is not supported — searching
// moves the cursor, it does not give an operator something to reach.
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
	// KindSearch asks for a pattern; Backward says which way from the cursor.
	KindSearch
	// KindSearchNext repeats the last search the way it was going, and
	// KindSearchPrev repeats it the other way.
	KindSearchNext
	KindSearchPrev
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
	// The find motions carry the character they were given in Command.Target.
	// MotionTill stops one short of it, which is what makes "ct," change up to
	// a comma and leave the comma in place.
	MotionFindForward
	MotionFindBackward
	MotionTillForward
	MotionTillBackward
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
// Object is a region an operator can act on without a motion reaching it:
// the word under the caret, or whatever a pair of brackets or quotes
// encloses.
//
// These are what a SQL editor is reached for most — ci( replaces an IN list,
// ci' a string literal — because the interesting region is almost always
// delimited rather than a number of words away.
type Object int

const (
	// ObjectNone means the operator took a motion instead.
	ObjectNone Object = iota
	ObjectWord
	ObjectParen
	ObjectBracket
	ObjectBrace
	ObjectAngle
	ObjectSingleQuote
	ObjectDoubleQuote
	ObjectBacktick
)

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
	// Backward reverses a search: "?" rather than "/".
	Backward bool
	// Count is how many times the motion applies, and is never zero — an
	// untyped count is one, so the caller can multiply without checking.
	Count int
	// Target is the character a find motion was given.
	Target rune
	// Object is the region an operator applies to, and Around says whether
	// its delimiters go with it: "i" is inside them, "a" takes them too.
	Object Object
	Around bool
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
