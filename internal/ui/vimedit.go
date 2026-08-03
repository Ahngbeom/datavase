package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/Ahngbeom/datavase/internal/vim"
	"github.com/gdamore/tcell/v2"
)

// The modal editor.
//
// The state machine in internal/vim decides what a key means; everything here
// carries that decision out against the TextArea, reusing the same motion and
// edit helpers the other keyboards use. Nothing in this file interprets keys,
// and nothing in the vim package touches a widget.

// vimKey routes a key through the modal state machine.
//
// The return value is what the widget sees. In normal and visual modes that
// is always nil: a key that leaks through would be typed into the buffer,
// which is the failure that makes a modal editor feel broken.
func (a *App) vimKey(ev *tcell.EventKey) *tcell.EventKey {
	cmd, outcome := a.vim.Feed(ev)

	switch outcome {
	case vim.OutcomePass:
		return ev
	case vim.OutcomePending:
		return nil
	}
	return a.runVimCommand(cmd)
}

// runVimCommand carries out a completed command, returning a key for the
// widget to handle itself when that is the only way to reach the behaviour.
func (a *App) runVimCommand(cmd vim.Command) *tcell.EventKey {
	switch cmd.Kind {
	case vim.KindMove:
		a.moveCursor(vimMove(cmd), a.vimSelecting())

	case vim.KindVisual:
		a.startVisual()

	case vim.KindEscape:
		a.collapseSelection()

	case vim.KindInsert:
		a.vimInsertAt(cmd.At)

	case vim.KindDelete, vim.KindChange, vim.KindYank:
		a.vimOperate(cmd)

	case vim.KindPaste:
		a.vimPut(cmd.At)

	case vim.KindSearch:
		a.showTextSearch(cmd.Backward)
	case vim.KindSearchNext:
		a.searchAgain(false)
	case vim.KindSearchPrev:
		a.searchAgain(true)

	// TextArea keeps its own undo stack and exposes it only through its key
	// handling, so u and Ctrl+R are delivered as the keys it recognises.
	// Reimplementing undo here would mean a second, disagreeing history.
	case vim.KindUndo:
		return tcell.NewEventKey(tcell.KeyCtrlZ, 'z', tcell.ModCtrl)
	case vim.KindRedo:
		return tcell.NewEventKey(tcell.KeyCtrlY, 'y', tcell.ModCtrl)
	}
	return nil
}

// vimSelecting reports whether motions should extend a selection.
func (a *App) vimSelecting() bool {
	switch a.vim.Mode() {
	case vim.ModeVisual, vim.ModeVisualLine:
		return true
	default:
		return false
	}
}

// vimMove resolves a command's motion and applies its count.
//
// Repeating the one-step function is how a count works for every motion here,
// including the ones where repeating changes nothing — "3$" is the end of the
// line, the same as "$", which is what vim does too.
func vimMove(cmd vim.Command) func(text string, offset int) int {
	step := vimMotion(cmd.Motion)
	if target := cmd.Target; target != 0 {
		step = findMotion(cmd.Motion, target)
	}

	count := cmd.Count
	if count <= 1 {
		return step
	}
	return func(text string, offset int) int {
		for i := 0; i < count; i++ {
			next := step(text, offset)
			// A motion that has stopped moving has run out of text, and
			// repeating it further would just spin.
			if next == offset {
				break
			}
			offset = next
		}
		return offset
	}
}

// inclusiveMotion reports whether an operator's range takes in the character
// the motion landed on.
func inclusiveMotion(m vim.Motion) bool {
	return m == vim.MotionFindForward || m == vim.MotionTillForward
}

// findMotion moves to the next or previous occurrence of a character on the
// current line.
//
// It stays on the line on purpose: that is what f and t do in vim, and a
// column list is exactly the case they are reached for. Not finding the
// character leaves the caret alone rather than guessing.
func findMotion(m vim.Motion, target rune) func(text string, offset int) int {
	return func(text string, offset int) int {
		start, end := lineStartAt(text, offset), lineEndAt(text, offset)

		switch m {
		case vim.MotionFindForward, vim.MotionTillForward:
			at := strings.Index(text[min(offset+1, end):end], string(target))
			if at < 0 {
				return offset
			}
			at += min(offset+1, end)
			if m == vim.MotionTillForward {
				return at - 1
			}
			return at

		case vim.MotionFindBackward, vim.MotionTillBackward:
			at := strings.LastIndex(text[start:offset], string(target))
			if at < 0 {
				return offset
			}
			at += start
			if m == vim.MotionTillBackward {
				return at + 1
			}
			return at
		}
		return offset
	}
}

// vimMotion maps a motion onto the editor's movement functions.
//
// Most of these are the ones the other keyboards already use; only the ones
// where vim genuinely differs are its own.
func vimMotion(m vim.Motion) func(text string, offset int) int {
	switch m {
	case vim.MotionLeft:
		return charLeft
	case vim.MotionRight:
		return charRight
	case vim.MotionUp:
		return lineUp
	case vim.MotionDown:
		return lineDown
	case vim.MotionWordForward:
		return wordStartRight
	case vim.MotionWordBackward:
		return wordLeft
	case vim.MotionWordEnd:
		return wordRight
	case vim.MotionLineStart:
		return lineStartAt
	case vim.MotionFirstNonBlank:
		return firstNonBlankAt
	case vim.MotionLineEnd:
		return lineEndAt
	case vim.MotionFileStart:
		return func(string, int) int { return 0 }
	case vim.MotionFileEnd:
		return func(text string, _ int) int { return len(text) }
	default:
		return func(_ string, offset int) int { return offset }
	}
}

// startVisual anchors a selection at the caret.
func (a *App) startVisual() {
	at := a.caretOffset(a.editor.GetText())

	a.editor.Select(at, at)
	a.selectionAnchor = at
	a.selectionCaret = at
}

// collapseSelection drops any selection, leaving the caret where it is.
func (a *App) collapseSelection() {
	at := a.caretOffset(a.editor.GetText())

	a.editor.Select(at, at)
	a.clearAnchor()
}

// vimInsertAt moves the caret to where insert mode should begin, opening a
// line first when that is what was asked for.
func (a *App) vimInsertAt(at vim.Place) {
	text := a.editor.GetText()
	caret := a.caretOffset(text)

	switch at {
	case vim.PlaceAfter:
		caret = charRight(text, caret)
	case vim.PlaceLineStart:
		caret = firstNonBlankAt(text, caret)
	case vim.PlaceLineEnd:
		caret = lineEndAt(text, caret)

	// A line opened without the indentation of the one it follows is a line
	// the user immediately has to fix by hand.
	case vim.PlaceOpenBelow:
		end := lineEndAt(text, caret)
		indent := vimIndent(text, caret)
		a.editor.Replace(end, end, "\n"+indent)
		caret = end + 1 + len(indent)

	case vim.PlaceOpenAbove:
		start := lineStartAt(text, caret)
		indent := vimIndent(text, caret)
		a.editor.Replace(start, start, indent+"\n")
		caret = start + len(indent)
	}

	a.editor.Select(caret, caret)
	a.clearAnchor()
}

// vimOperate applies delete, change or yank to the range the command names.
//
// All three go through a single Replace so each is one step for undo — an
// editor whose undo cannot reverse its own commands is one people stop
// using the commands of.
func (a *App) vimOperate(cmd vim.Command) {
	text := a.editor.GetText()
	start, end, linewise := a.vimTarget(cmd, text)

	yanked := text[start:end]
	if linewise {
		yanked += "\n"
	}
	a.setRegister(yanked, linewise)

	switch cmd.Kind {
	case vim.KindYank:
		// vim leaves the caret at the start of what was yanked.
		a.editor.Select(start, start)
		a.notice("yanked")

	case vim.KindChange:
		if linewise {
			// A changed line keeps its indentation and its place; only the
			// text goes. Removing the line outright would put the caret on
			// the following one, which is not where the typing should land.
			indent := vimIndent(text, start)
			a.editor.Replace(start, end, indent)
			a.editor.Select(start+len(indent), start+len(indent))
			break
		}
		a.editor.Replace(start, end, "")
		a.editor.Select(start, start)

	case vim.KindDelete:
		if linewise {
			e := deleteLines(text, start, end)
			a.editor.Replace(e.start, e.end, e.text)
			a.editor.Select(e.start, e.start)
			break
		}
		a.editor.Replace(start, end, "")
		a.editor.Select(start, start)
	}

	a.clearAnchor()
}

// vimTarget resolves a command to a byte range of the buffer.
//
// Linewise ranges cover the line's text without its newline; taking the
// newline is left to whoever is deleting, because a yank and a delete want
// different things from it.
func (a *App) vimTarget(cmd vim.Command, text string) (start, end int, linewise bool) {
	caret := a.caretOffset(text)
	from, to := caret, caret

	switch {
	case cmd.Object != vim.ObjectNone:
		if s, e, ok := objectSpan(text, caret, cmd.Object, cmd.Around); ok {
			from, to = s, e
		}
	case cmd.Selection:
		if a.editor.HasSelection() {
			_, from, to = a.editor.GetSelection()
		}
	case cmd.Motion != vim.MotionNone:
		to = vimMove(cmd)(text, caret)
	}
	if from > to {
		from, to = to, from
	}

	// f and t are inclusive when an operator is using them: "df," takes the
	// comma with it, which is the difference between deleting a column from a
	// list and leaving its separator behind. Backward finds stay exclusive,
	// as vim's do — the swap above already puts the caret at the far end.
	if inclusiveMotion(cmd.Motion) && to < len(text) {
		_, size := utf8.DecodeRuneInString(text[to:])
		to += size
	}

	// Expanding to whole lines last means one rule serves all three ways a
	// linewise range arises: "dd", "V" and a line-spanning motion like "dG".
	// A count on a linewise operator means more lines — "3dd" is three of
	// them — and there is no motion to repeat, so the span is walked down
	// before it is expanded.
	if cmd.Linewise {
		if cmd.Motion == vim.MotionNone {
			for i := 1; i < cmd.Count; i++ {
				next := lineDown(text, to)
				if next == to {
					break
				}
				to = next
			}
		}
		from, to = lineSpan(text, from, to)
	}
	return from, to, cmd.Linewise
}

// setRegister records what was yanked or deleted.
//
// There is one unnamed register, and it is mirrored to the system clipboard:
// the most common reason to yank in a SQL editor is to paste somewhere else.
func (a *App) setRegister(text string, linewise bool) {
	if text == "" {
		return
	}

	a.register = text
	a.registerLinewise = linewise
	a.setClipboard(text)
}

// vimPut inserts the register at or after the caret.
func (a *App) vimPut(at vim.Place) {
	if a.register == "" {
		a.notice("nothing to put")
		return
	}

	text := a.editor.GetText()
	caret := a.caretOffset(text)

	if a.registerLinewise {
		a.putLines(text, caret, at)
		return
	}

	insertAt := caret
	if at == vim.PlaceAfter {
		insertAt = charRight(text, caret)
	}
	a.editor.Replace(insertAt, insertAt, a.register)
	a.editor.Select(insertAt+len(a.register), insertAt+len(a.register))
	a.clearAnchor()
}

// putLines puts a linewise register above or below the caret's line, which is
// what makes "yyp" duplicate a line rather than split it.
func (a *App) putLines(text string, caret int, at vim.Place) {
	block := trimTrailingNewline(a.register)

	if at == vim.PlaceAfter {
		end := lineEndAt(text, caret)
		a.editor.Replace(end, end, "\n"+block)
		a.editor.Select(end+1, end+1)
		a.clearAnchor()
		return
	}

	start := lineStartAt(text, caret)
	a.editor.Replace(start, start, block+"\n")
	a.editor.Select(start, start)
	a.clearAnchor()
}
