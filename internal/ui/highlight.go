package ui

import (
	"strings"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// role is what a stretch of editor text is, for colouring.
type role int

const (
	roleNone role = iota
	roleKeyword
	roleLiteral
	roleComment
)

func (r role) String() string {
	switch r {
	case roleKeyword:
		return "keyword"
	case roleLiteral:
		return "literal"
	case roleComment:
		return "comment"
	default:
		return "none"
	}
}

// span is a half-open byte range of the buffer with one role.
type span struct {
	from, to int
	role     role
}

// highlights returns the coloured spans of sql, in order.
func highlights(sql string) []span {
	toks := sqlparse.Tokenize(sql)
	out := make([]span, 0, len(toks))
	for i, tok := range toks {
		r := roleNone
		switch tok.Kind {
		case sqlparse.String, sqlparse.Number:
			r = roleLiteral
		case sqlparse.Comment:
			r = roleComment
		case sqlparse.Word:
			name := i > 0 && sqlparse.MakesAName(toks[i-1])
			if !name && (sqlparse.IsReserved(tok.Text) || statementWords[strings.ToUpper(tok.Text)]) {
				r = roleKeyword
			}
		}
		if r != roleNone {
			out = append(out, span{from: tok.Pos, to: tok.End, role: r})
		}
	}
	return out
}

// statementWords are coloured as keywords although either server lets them
// name a table. Colour is only a hint, so a column called "end" drawn in
// blue costs nothing; upper-casing it in the formatter could.
var statementWords = map[string]bool{
	"BEGIN": true, "COMMIT": true, "ROLLBACK": true, "END": true,
	"TRUNCATE": true, "TRANSACTION": true, "TABLES": true,
	"DATABASES": true, "COLUMNS": true, "VIEW": true,
}

// highlightedEditor draws the editor and then recolours its text by role.
//
// TextArea has a single text style, so the colours go on afterwards, over
// the cells it drew. That relies on wrapping being off: with it off, screen
// row r is buffer line rowOffset+r, and a column is the sum of the cell
// widths before it, counted as TextArea counts them.
type highlightedEditor struct {
	*tview.TextArea

	// The spans of the text they were computed for; a draw with the buffer
	// unchanged — moving the caret, a redraw for the status bar — reuses them.
	text  string
	spans []span

	// afterKey runs once the widget has handled a key or a paste, the first
	// point at which both the text and the caret reflect it: TextArea's
	// changed callback fires before the caret has moved.
	afterKey func()
}

func (e *highlightedEditor) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	handle := e.TextArea.InputHandler()
	return func(ev *tcell.EventKey, setFocus func(tview.Primitive)) {
		handle(ev, setFocus)
		e.keyHandled()
	}
}

// PasteHandler is wrapped too: a bracketed paste never passes through the
// key handler, so without this it would be the one edit afterKey missed.
func (e *highlightedEditor) PasteHandler() func(string, func(tview.Primitive)) {
	handle := e.TextArea.PasteHandler()
	return func(text string, setFocus func(tview.Primitive)) {
		handle(text, setFocus)
		e.keyHandled()
	}
}

func (e *highlightedEditor) keyHandled() {
	if e.afterKey != nil {
		e.afterKey()
	}
}

func (e *highlightedEditor) Draw(screen tcell.Screen) {
	e.TextArea.Draw(screen)

	text := e.GetText()
	if text == "" {
		return
	}
	if text != e.text || e.spans == nil {
		e.text, e.spans = text, highlights(text)
	}

	x, y, width, height := e.GetInnerRect()
	rowOffset, columnOffset := e.GetOffset()
	_, selFrom, selTo := e.GetSelection()

	spans := e.spans
	lineStart := 0
	for row := 0; row < rowOffset+height; row++ {
		end := strings.IndexByte(text[lineStart:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += lineStart
		}
		// Spans that ended before this line are done with.
		for len(spans) > 0 && spans[0].to <= lineStart {
			spans = spans[1:]
		}
		if row >= rowOffset {
			paintLine(screen, text[lineStart:end], lineStart, spans,
				x-columnOffset, y+row-rowOffset, x, x+width, selFrom, selTo)
		}
		if end == len(text) {
			return
		}
		lineStart = end + 1
	}
}

// paintLine recolours one buffer line drawn at row, whose column 0 lands on
// screen column origin. Only cells in [left, right) are on screen.
func paintLine(screen tcell.Screen, line string, base int, spans []span,
	origin, row, left, right, selFrom, selTo int) {
	column := 0
	rest := line
	state := -1
	at := base
	for rest != "" {
		var cluster string
		var boundaries int
		cluster, rest, boundaries, state = uniseg.StepString(rest, state)
		cells := boundaries >> uniseg.ShiftWidth
		if cluster == "\t" {
			cells = tview.TabSize
		}

		for len(spans) > 0 && spans[0].to <= at {
			spans = spans[1:]
		}
		inSpan := len(spans) > 0 && spans[0].from <= at
		selected := at >= selFrom && at < selTo
		if inSpan && !selected {
			for c := 0; c < cells; c++ {
				sx := origin + column + c
				if sx < left || sx >= right {
					continue
				}
				mainc, combc, style, _ := screen.GetContent(sx, row)
				screen.SetContent(sx, row, mainc, combc, styled(style, spans[0].role))
			}
		}

		column += cells
		at += len(cluster)
	}
}

func styled(style tcell.Style, r role) tcell.Style {
	switch r {
	case roleKeyword:
		return style.Foreground(colourKeyword).Bold(true)
	case roleLiteral:
		return style.Foreground(colourLiteral)
	case roleComment:
		return style.Foreground(colourComment).Italic(true)
	}
	return style
}
