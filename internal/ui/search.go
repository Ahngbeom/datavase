package ui

import (
	"fmt"

	"github.com/Ahngbeom/datavase/internal/match"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Finding text on screen.
//
// The prompt lives here rather than in the vim package, which only reports
// that a search was asked for. Collecting the pattern there would put every
// keystroke of it through the modal state machine, where an arrow key resolves
// as a motion and takes any waiting operator with it — deleting a character in
// the middle of typing a search term.
//
// One prompt serves both the editor and the results. Which of them it searches
// is decided when it opens, from whatever has focus, and remembered so that n
// and N keep meaning the same thing afterwards.

// searchWhere names what a search walks.
type searchWhere int

const (
	searchEditorText searchWhere = iota
	searchResultCells
)

// searchState is the last search, kept so n and N have something to repeat.
type searchState struct {
	pattern string
	// backward is the direction the pattern was first searched in. n follows
	// it and N goes the other way, which is what makes N after "?" go
	// forwards rather than merely backwards again.
	backward bool
	where    searchWhere
}

// searchOrigin is where a search started, so that Escape can put everything
// back and each keystroke can match from the same place.
type searchOrigin struct {
	offset      int
	row, column int
}

// showTextSearch opens the prompt.
func (a *App) showTextSearch(backward bool) {
	where := searchEditorText
	if a.app.GetFocus() == a.grid {
		where = searchResultCells
	}

	origin := searchOrigin{offset: a.caretOffset(a.editor.GetText())}
	origin.row, origin.column = a.grid.GetSelection()

	label := "/"
	if backward {
		label = "?"
	}

	input := tview.NewInputField().SetLabel(label)
	input.SetFieldBackgroundColor(tcell.ColorBlack)

	// Nothing typed yet is not a failed search.
	matched := true

	// Matching as the pattern grows, always from where the search began rather
	// than from the last match: without that, deleting a character leaves the
	// caret wherever the longer pattern had taken it.
	input.SetChangedFunc(func(pattern string) {
		if pattern == "" {
			a.returnTo(where, origin)
			matched = true
			return
		}

		matched = a.jumpToMatch(pattern, where, origin, backward)
		if !matched {
			// Back to the start, not left wherever the last pattern that did
			// match had reached. Otherwise typing one character too many
			// leaves the caret on a match for a prefix of what is on screen,
			// which reads as a match for the whole of it.
			a.returnTo(where, origin)
		}
	})

	dismiss := func() {
		a.pages.RemovePage(pageSearch)
		if where == searchResultCells {
			a.app.SetFocus(a.grid)
			return
		}
		a.app.SetFocus(a.editor)
	}

	input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyEnter:
			pattern := input.GetText()
			dismiss()
			if pattern == "" {
				return nil
			}
			// Remembered even when it found nothing, so that repeating after
			// more rows arrive looks for the same thing.
			a.search = searchState{pattern: pattern, backward: backward, where: where}
			if !matched {
				// Only now: the prompt sits over the status bar, so anything
				// said while typing would have been said to a covered line.
				a.notice(a.noMatch(pattern))
			}
			return nil

		case tcell.KeyEscape:
			a.returnTo(where, origin)
			dismiss()
			return nil
		}
		return ev
	})

	a.pages.AddPage(pageSearch, newDocked(input, 1), true, true)
	a.app.SetFocus(input)
}

// returnTo puts the caret or the selected cell back where the search started.
func (a *App) returnTo(where searchWhere, origin searchOrigin) {
	if where == searchResultCells {
		a.grid.Select(origin.row, origin.column)
		return
	}
	a.moveCursor(func(string, int) int { return origin.offset }, false)
}

// jumpToMatch moves to the first match at or after the origin, reporting
// whether there was one.
func (a *App) jumpToMatch(pattern string, where searchWhere, origin searchOrigin, backward bool) bool {
	if where == searchResultCells {
		row, column, ok := a.findInResults(pattern, origin.row, origin.column, backward, true)
		if ok {
			a.grid.Select(row, column)
		}
		return ok
	}

	offset, ok := findInText(a.editor.GetText(), pattern, origin.offset, backward)
	if !ok {
		return false
	}
	// Extending rather than jumping while a selection is being made, so that
	// searching in visual mode reaches the match the way every other motion
	// there does.
	a.moveCursor(func(string, int) int { return offset }, a.vimSelecting())
	return true
}

// gridKey gives the results the search keys.
//
// Plainly, in every keyboard preset: a grid is not a text field, so there is
// nothing for an unmodified letter to collide with. tview's own table already
// answers to hjkl, g and G, so movement is left to it rather than reimplemented
// here where the two could disagree.
func (a *App) gridKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() != tcell.KeyRune {
		return ev
	}

	switch ev.Rune() {
	case '/':
		a.showTextSearch(false)
	case '?':
		a.showTextSearch(true)
	case 'n':
		a.searchAgain(false)
	case 'N':
		a.searchAgain(true)
	default:
		return ev
	}
	return nil
}

// searchAgain repeats the last search. reverse is N rather than n.
func (a *App) searchAgain(reverse bool) {
	if a.search.pattern == "" {
		a.notice("nothing has been searched for yet")
		return
	}

	backward := a.search.backward != reverse

	if a.search.where == searchResultCells {
		row, column := a.grid.GetSelection()
		found, foundColumn, ok := a.findInResults(a.search.pattern, row, column, backward, false)
		if !ok {
			a.notice(a.noMatch(a.search.pattern))
			return
		}
		a.grid.Select(found, foundColumn)
		return
	}

	text := a.editor.GetText()
	from := a.caretOffset(text)
	// Past the match already under the caret, or n would never leave it.
	if !backward {
		from++
	}

	offset, ok := findInText(text, a.search.pattern, from, backward)
	if !ok {
		a.notice(a.noMatch(a.search.pattern))
		return
	}
	a.moveCursor(func(string, int) int { return offset }, a.vimSelecting())
}

// findInText searches forwards or backwards, starting over at the far end
// rather than stopping.
//
// Wrapping is what vim does and what makes n usable: a search that dies at the
// end of the buffer has to be retyped from the top.
func findInText(text, pattern string, from int, backward bool) (int, bool) {
	if backward {
		if offset, ok := match.Prev(text, pattern, from); ok {
			return offset, true
		}
		return match.Prev(text, pattern, len(text))
	}
	if offset, ok := match.Next(text, pattern, from); ok {
		return offset, true
	}
	return match.Next(text, pattern, 0)
}

// findInResults walks the buffer cell by cell, in reading order.
//
// The raw value is formatted afresh rather than read through Buffer.Cell,
// which returns what the grid displays: that is truncated to a couple of
// hundred runes and has its brackets doubled for the markup parser, so a
// search for "[" would never match one in the data and anything past the
// truncation could not be found at all.
//
// Rows are pulled whole. Buffer takes its read lock per call, and asking cell
// by cell would take it a hundred thousand times over a large result while the
// goroutine still filling the buffer waits behind it.
func (a *App) findInResults(pattern string, row, column int, backward, inclusive bool) (int, int, bool) {
	rows := a.buf.RowCount()
	columns := a.buf.ColumnCount()
	if rows == 0 || columns == 0 {
		return 0, 0, false
	}

	total := rows * columns
	// The grid carries a header row that the buffer does not.
	from := clamp((row-1)*columns+column, 0, total-1)

	step := 1
	if backward {
		step = -1
	}
	// A pattern being typed may already match the cell that is selected, and
	// jumping off it would read as the search skipping one. Repeating with n
	// has to move.
	first := 1
	if inclusive {
		first = 0
	}

	for i := first; i < first+total; i++ {
		at := ((from+step*i)%total + total) % total

		r, c := at/columns, at%columns
		if match.Contains(result.Format(a.buf.Raw(r, c)), pattern) {
			return r + 1, c, true
		}
	}
	return 0, 0, false
}

// noMatch says nothing was found, admitting that a result still arriving may
// simply not hold the match yet — "no match" and "no match so far" are
// different answers, and only one of them is worth waiting on.
func (a *App) noMatch(pattern string) string {
	if a.running != nil {
		return fmt.Sprintf("no match for %q in the rows so far", pattern)
	}
	return fmt.Sprintf("no match for %q", pattern)
}

// docked pins a primitive to the bottom rows of the screen and draws nothing
// else.
//
// Unlike the floating dialogs it deliberately does not fill its rect: a search
// prompt that blanked the screen would hide the text being searched, which is
// the one thing that has to stay visible while the pattern is typed.
type docked struct {
	*tview.Box
	content tview.Primitive
	rows    int
}

func newDocked(content tview.Primitive, rows int) *docked {
	return &docked{Box: tview.NewBox(), content: content, rows: rows}
}

func (d *docked) Draw(screen tcell.Screen) {
	x, y, width, height := d.GetRect()
	if height < d.rows || width < 1 {
		return
	}
	d.content.SetRect(x, y+height-d.rows, width, d.rows)
	d.content.Draw(screen)
}

func (d *docked) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return d.content.InputHandler()
}

func (d *docked) Focus(delegate func(p tview.Primitive)) { delegate(d.content) }

func (d *docked) HasFocus() bool { return d.content.HasFocus() }

func (d *docked) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return d.content.MouseHandler()
}
