package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/complete"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// completionTimeout bounds a lookup. It is generous relative to a cache read
// (tens of microseconds) and exists only so a locked database cannot wedge
// the keystroke that triggered it.
const completionTimeout = 2 * time.Second

// popupRows is how many candidates are visible before scrolling.
const popupRows = 10

// showCompletion offers candidates for the caret position.
//
// The lookup runs inline rather than in a goroutine: it reads the local cache
// and returns in microseconds, and a popup that appears a moment after the
// keystroke feels broken in a way a synchronous one does not.
func (a *App) showCompletion() {
	if a.completion == nil {
		a.notice("completion needs the schema cache, which is still loading")
		return
	}

	text := a.editor.GetText()
	row, column, _, _ := a.editor.GetCursor()
	offset := offsetAt(text, row, column)

	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()

	candidates, err := a.completion.Suggest(ctx, text, offset)
	if err != nil {
		a.notice(fmt.Sprintf("completion failed: %v", err))
		return
	}
	if len(candidates) == 0 {
		a.notice("nothing to complete here")
		return
	}

	// A single candidate needs no menu; inserting it directly is what makes
	// completion feel quick rather than ceremonial.
	if len(candidates) == 1 {
		a.acceptCandidate(candidates[0])
		return
	}

	a.openCompletionPopup(candidates)
}

func (a *App) openCompletionPopup(candidates []complete.Candidate) {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetWrapAround(false)

	for _, c := range candidates {
		candidate := c
		label := fmt.Sprintf("%-28s %s", result.EscapeTags(c.Text), c.Kind)
		if c.Detail != "" {
			label = fmt.Sprintf("%-28s %-8s %s",
				result.EscapeTags(c.Text), c.Kind, result.EscapeTags(c.Detail))
		}
		list.AddItem(label, "", 0, func() {
			a.closeCompletion()
			a.acceptCandidate(candidate)
		})
	}

	list.SetBorder(true).SetTitle(fmt.Sprintf(" complete (%d) ", len(candidates)))
	list.SetBackgroundColor(tcell.ColorBlack)

	// Escape dismisses; anything else the list does not handle also returns
	// control to the editor, so the popup never traps the keyboard.
	list.SetDoneFunc(func() { a.closeCompletion() })

	height := len(candidates) + 2
	if height > popupRows+2 {
		height = popupRows + 2
	}

	a.pages.AddPage(pageComplete, centred(list, 60, height), true, true)
	a.app.SetFocus(list)
}

func (a *App) closeCompletion() {
	a.pages.RemovePage(pageComplete)
	a.app.SetFocus(a.editor)
}

// acceptCandidate writes the chosen text over the range it replaces.
func (a *App) acceptCandidate(c complete.Candidate) {
	a.editor.Replace(c.ReplaceFrom, c.ReplaceTo, c.Text)

	at := c.ReplaceFrom + len(c.Text)
	a.editor.Select(at, at)
	a.app.SetFocus(a.editor)
}
