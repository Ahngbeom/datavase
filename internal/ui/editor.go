package ui

import (
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// bindEditor puts the keymap in front of TextArea's own handling.
//
// tview's TextArea ships emacs-style bindings — copy is Ctrl+Q, Ctrl+A goes
// to the start of the line, Ctrl+D deletes a character. Those collide with
// what a DataGrip or VS Code user expects, so the keys datavase claims are
// consumed here and never reach the widget.
func (a *App) bindEditor() {
	a.editor.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if a.editorAction(a.keys.Lookup(ev)) {
			return nil
		}
		// The bound keys are all chords, so they are resolved first and the
		// modal machine never sees them: ⌘Y still deletes a line on the vim
		// keyboard, and a plain letter still reaches vim.
		if a.keys.Modal() {
			return a.vimKey(ev)
		}
		return ev
	})
}

// editorAction handles the actions that operate on editor text, and reports
// whether the key was consumed.
func (a *App) editorAction(action keymap.Action) bool {
	switch action {
	case keymap.ActionWordLeft:
		a.moveCursor(wordLeft, false)
	case keymap.ActionWordRight:
		a.moveCursor(wordRight, false)
	case keymap.ActionSelectWordLeft:
		a.moveCursor(wordLeft, true)
	case keymap.ActionSelectWordRight:
		a.moveCursor(wordRight, true)
	case keymap.ActionLineStart:
		a.moveCursor(lineStartAt, false)
	case keymap.ActionLineEnd:
		a.moveCursor(lineEndAt, false)
	case keymap.ActionSelectLineStart:
		a.moveCursor(lineStartAt, true)
	case keymap.ActionSelectLineEnd:
		a.moveCursor(lineEndAt, true)
	case keymap.ActionDeleteWordLeft:
		a.applyCaretEdit(deleteWordLeft)
	case keymap.ActionDeleteToLineStart:
		a.applyCaretEdit(deleteToLineStart)

	case keymap.ActionSelectAll:
		a.selectAll()
	case keymap.ActionCopy:
		a.copySelection()
	case keymap.ActionCut:
		a.cutSelection()
	case keymap.ActionPaste:
		a.paste()
	case keymap.ActionToggleComment:
		a.applyEdit(toggleComment)
	case keymap.ActionDuplicateLine:
		a.applyEdit(duplicateLines)
	case keymap.ActionDeleteLine:
		a.applyEdit(deleteLines)
	default:
		return false
	}
	return true
}

// editorRange returns the byte range the next edit should act on: the
// selection if there is one, otherwise the caret.
func (a *App) editorRange() (text string, from, to int) {
	text = a.editor.GetText()

	if a.editor.HasSelection() {
		_, start, end := a.editor.GetSelection()
		return text, start, end
	}

	at := a.caretOffset(text)
	return text, at, at
}

// caretOffset converts the editor's reported caret into a byte offset.
func (a *App) caretOffset(text string) int {
	row, column, _, _ := a.editor.GetCursor()
	return offsetAt(text, row, column)
}

// noAnchor marks the absence of a selection anchor.
const noAnchor = -1

// moveCursor moves the caret with one of the motion functions, extending the
// selection when asked.
//
// Both ends of a growing selection have to be remembered rather than read
// back. tview's Select swaps its arguments when they are out of order and
// leaves the caret at the larger one, and GetSelection reports the same
// normalised pair — so nothing the widget exposes says which end the user is
// dragging. Without that, extending right and then back left would flip the
// selection instead of shrinking it.
func (a *App) moveCursor(motion func(text string, offset int) int, extend bool) {
	text := a.editor.GetText()
	from := a.caretOffset(text)

	if extend {
		// The widget is the authority on whether a selection still exists;
		// anything typed since would have cleared it, leaving our anchor
		// stale.
		if a.selectionAnchor != noAnchor && a.editor.HasSelection() {
			from = a.selectionCaret
		} else {
			a.selectionAnchor = noAnchor
		}
	}

	target := motion(text, from)

	if !extend {
		a.clearAnchor()
		a.editor.Select(target, target)
		return
	}

	if a.selectionAnchor == noAnchor {
		a.selectionAnchor = from
	}
	a.selectionCaret = target
	a.editor.Select(a.selectionAnchor, target)
}

// clearAnchor forgets the selection direction, which any edit or unmodified
// movement invalidates.
func (a *App) clearAnchor() {
	a.selectionAnchor = noAnchor
	a.selectionCaret = 0
}

// applyEdit runs one of the edit helpers against the current range.
//
// Replace is used rather than SetText so the change is a single undo step —
// an editor whose Ctrl+Z cannot reverse its own commands is worse than one
// without the commands.
func (a *App) applyEdit(fn func(text string, from, to int) edit) {
	text, from, to := a.editorRange()
	e := fn(text, from, to)

	a.editor.Replace(e.start, e.end, e.text)
	// Place the caret at the end of what was written, matching where typing
	// would have left it.
	a.editor.Select(e.start+len(e.text), e.start+len(e.text))
	a.clearAnchor()
}

// applyCaretEdit runs an edit that works from the caret alone.
//
// A selection takes precedence: deleting a word while text is selected should
// remove the selection, which is what every editor does and what avoids
// destroying a range the user deliberately marked.
func (a *App) applyCaretEdit(fn func(text string, offset int) edit) {
	if a.editor.HasSelection() {
		_, start, end := a.editor.GetSelection()
		a.editor.Replace(start, end, "")
		a.editor.Select(start, start)
		a.clearAnchor()
		return
	}

	text := a.editor.GetText()
	e := fn(text, a.caretOffset(text))

	a.editor.Replace(e.start, e.end, e.text)
	a.editor.Select(e.start+len(e.text), e.start+len(e.text))
	a.clearAnchor()
}

func (a *App) selectAll() {
	a.editor.Select(0, a.editor.GetTextLength())
	// Extending from here should start at the beginning, which is the end
	// the caret is not on.
	a.selectionAnchor = 0
	a.selectionCaret = a.editor.GetTextLength()
}

// copySelection puts the selected text on the system clipboard.
func (a *App) copySelection() {
	if !a.editor.HasSelection() {
		a.notice("nothing selected")
		return
	}

	text, _, _ := a.editor.GetSelection()
	a.setClipboard(text)
	a.notice("copied")
}

func (a *App) cutSelection() {
	if !a.editor.HasSelection() {
		a.notice("nothing selected")
		return
	}

	text, start, end := a.editor.GetSelection()
	a.setClipboard(text)
	a.editor.Replace(start, end, "")
	a.editor.Select(start, start)
}

func (a *App) paste() {
	if text := a.editor.GetClipboardText(); text != "" {
		_, from, to := a.editorRange()
		a.editor.Replace(from, to, text)
		a.editor.Select(from+len(text), from+len(text))
	}
}

// setClipboard writes to the system clipboard and to the session-local copy.
//
// The terminal is reached with OSC 52, which works across SSH where a helper
// such as pbcopy would only ever reach the remote machine.
//
// Reading back is deliberately not attempted: OSC 52 answers asynchronously
// and most terminals refuse clipboard reads outright, since that is how a
// program would steal whatever the user last copied. Pasting therefore uses
// the session-local copy, and the terminal's own paste — already enabled
// through bracketed paste — covers text from other applications.
func (a *App) setClipboard(text string) {
	a.clipboard = text

	if a.screen != nil {
		a.screen.SetClipboard([]byte(text))
	}
}

func (a *App) readClipboard() string { return a.clipboard }
