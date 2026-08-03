package ui

import "github.com/Ahngbeom/datavase/internal/vim"

// Dot-repeat.
//
// The vim package decides that "." was pressed and which commands are worth
// repeating; it cannot decide what to replay. A change that entered insert
// mode includes the text that was typed, and insert-mode keys go straight to
// the widget without the state machine seeing them — so the recording lives
// here, where both halves are visible.

// change is the last thing that altered the buffer, ready to be done again.
type change struct {
	cmd vim.Command
	// typed is the text entered before Escape, for a change that opened
	// insert mode. Empty for one that did not.
	typed string
	// insertFrom is where that text began, held while insert mode is open so
	// the text can be read out of the buffer rather than accumulated
	// keystroke by keystroke — which would miss a backspace or a paste.
	insertFrom int
	// insertOpen says the recording is still waiting for Escape.
	insertOpen bool
	ok         bool
}

// recordChange notes a command that altered the buffer.
//
// It is called after the command has run, so the caret is already where the
// typing will start.
func (a *App) recordChange(cmd vim.Command) {
	if !cmd.Changes() {
		return
	}

	a.lastChange = change{cmd: cmd, ok: true}
	if a.vim.Mode() == vim.ModeInsert {
		a.lastChange.insertFrom = a.caretOffset(a.editor.GetText())
		a.lastChange.insertOpen = true
	}
}

// closeInsertRecording reads back what was typed, once Escape has ended the
// insert the change opened.
//
// The text is taken from the buffer rather than accumulated as it was typed,
// so a backspace or a paste is recorded as what it left behind. A caret moved
// backwards out of the inserted run gives up rather than recording a span
// that means nothing.
func (a *App) closeInsertRecording() {
	if !a.lastChange.insertOpen {
		return
	}
	a.lastChange.insertOpen = false

	text := a.editor.GetText()
	from, to := a.lastChange.insertFrom, a.caretOffset(text)
	if from < 0 || to < from || to > len(text) {
		return
	}
	a.lastChange.typed = text[from:to]
}

// repeatChange carries out the recorded change again.
//
// A count on the "." replaces the one the change was recorded with, which is
// what vim does: "3." after a "dw" deletes three words rather than repeating
// a one-word delete three times.
func (a *App) repeatChange(count int) {
	if !a.lastChange.ok {
		a.notice("nothing to repeat")
		return
	}

	// Copied before running: replaying a change records it again, and the
	// recording would otherwise be rewritten underneath this.
	repeat := a.lastChange
	if count > 1 {
		repeat.cmd.Count = count
	}

	a.runVimCommand(repeat.cmd)

	if repeat.typed != "" {
		at := a.caretOffset(a.editor.GetText())
		a.editor.Replace(at, at, repeat.typed)
		a.editor.Select(at+len(repeat.typed), at+len(repeat.typed))
	}

	// The replay ends in normal mode however the change ended, because "."
	// finishes a change rather than opening one to carry on typing into. A
	// fresh state is how the interface already returns to normal elsewhere,
	// rather than growing the package an exported reset for one caller.
	if a.vim.Mode() == vim.ModeInsert {
		a.vim = vim.New()
	}
	a.lastChange = repeat
}
