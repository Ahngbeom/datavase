package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/complete"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// completionTimeout bounds a lookup. It is generous relative to a cache read
// (tens of microseconds) and exists only so a locked database cannot wedge
// the keystroke that triggered it.
const completionTimeout = 2 * time.Second

// popupRows is how many candidates are visible before scrolling.
const popupRows = 10

// menuWidth caps the menu so a long detail cannot cover the whole editor.
const menuWidth = 60

// showCompletion offers candidates for the caret position.
//
// The lookup runs inline rather than in a goroutine: it reads the local cache
// and returns in microseconds, and a menu that appears a moment after the
// keystroke feels broken in a way a synchronous one does not.
func (a *App) showCompletion() {
	if a.completion == nil {
		a.notice("completion needs the schema cache, which is still loading")
		return
	}

	candidates, err := a.suggestAtCaret()
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

	a.menu = completionMenu{anchor: candidates[0].ReplaceFrom}
	a.menu.set(candidates)
}

// bindCompletion hooks the menu into the editor and the frame. The menu
// closes when the editor loses focus, which covers every dialog, every pane
// switch and every click elsewhere at once.
func (a *App) bindCompletion() {
	a.editor.afterKey = a.narrowCompletion
	a.editor.SetBlurFunc(a.closeCompletion)
	a.app.SetAfterDrawFunc(a.drawCompletion)
}

func (a *App) suggestAtCaret() ([]complete.Candidate, error) {
	text := a.editor.GetText()

	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()
	return a.completion.Suggest(ctx, text, a.caretOffset(text))
}

// completionMenu is the candidate list drawn under the word being completed.
//
// It is state and drawing only, never a primitive that takes focus: the
// editor keeps the keyboard, so typing narrows the list instead of being
// swallowed by it.
type completionMenu struct {
	candidates    []complete.Candidate
	selected, top int
	// anchor is where the word being completed starts. A lookup that answers
	// for a different start means the caret has left that word.
	anchor int
	// box is where the last frame drew the menu, which is what a click has
	// to be judged against; zero when it was not drawn.
	box struct{ x, y, width, height int }
}

func (m *completionMenu) isOpen() bool { return len(m.candidates) > 0 }

func (m *completionMenu) current() complete.Candidate { return m.candidates[m.selected] }

// set replaces the candidates, keeping the highlight on the same name when
// it survived the narrowing, so typing does not move it out from under the
// user.
func (m *completionMenu) set(candidates []complete.Candidate) {
	keep := ""
	if m.isOpen() {
		keep = m.current().Text
	}
	m.candidates, m.selected, m.top = candidates, 0, 0
	for i, c := range candidates {
		if c.Text == keep {
			m.move(i)
			break
		}
	}
}

func (m *completionMenu) move(step int) {
	m.selected = max(0, min(len(m.candidates)-1, m.selected+step))
	if m.selected < m.top {
		m.top = m.selected
	}
	if m.selected >= m.top+popupRows {
		m.top = m.selected - popupRows + 1
	}
}

func (a *App) closeCompletion() { a.menu = completionMenu{} }

// acceptCandidate writes the chosen text over the range it replaces.
func (a *App) acceptCandidate(c complete.Candidate) {
	a.closeCompletion()
	a.editor.Replace(c.ReplaceFrom, c.ReplaceTo, c.Text)

	at := c.ReplaceFrom + len(c.Text)
	a.editor.Select(at, at)
	a.app.SetFocus(a.editor)
}

// completionKey handles a key while the menu is open, and reports whether
// the key was consumed.
//
// It runs ahead of the key map because Tab and the arrows mean something
// else there. Typing falls through to the editor and narrows the menu
// afterwards; any other key closes it first and then does what it always
// does, so the menu can never trap the keyboard.
func (a *App) completionKey(ev *tcell.EventKey) bool {
	plain := ev.Modifiers() == tcell.ModNone
	ctrl := func(r rune) bool {
		return ev.Key() == tcell.KeyRune && ev.Rune() == r && ev.Modifiers() == tcell.ModCtrl
	}

	switch {
	case plain && ev.Key() == tcell.KeyUp, ev.Key() == tcell.KeyCtrlP, ctrl('p'):
		a.menu.move(-1)
	case plain && ev.Key() == tcell.KeyDown, ev.Key() == tcell.KeyCtrlN, ctrl('n'):
		a.menu.move(1)
	case plain && (ev.Key() == tcell.KeyEnter || ev.Key() == tcell.KeyTab):
		a.acceptCandidate(a.menu.current())
	case plain && ev.Key() == tcell.KeyEscape:
		a.closeCompletion()
	case typing(ev):
		return false
	default:
		a.closeCompletion()
		return false
	}
	return true
}

// typing reports whether a key only edits the word at the caret.
func typing(ev *tcell.EventKey) bool {
	if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) != 0 {
		return false
	}
	switch ev.Key() {
	case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2:
		return true
	}
	return false
}

// narrowCompletion re-runs the lookup after the editor has taken a key.
func (a *App) narrowCompletion() {
	if !a.menu.isOpen() {
		return
	}
	candidates, err := a.suggestAtCaret()
	if err != nil || len(candidates) == 0 || candidates[0].ReplaceFrom != a.menu.anchor {
		a.closeCompletion()
		return
	}
	a.menu.set(candidates)
}

// completionMouse claims a mouse event that lands on the menu, and closes
// the menu on a press anywhere else before handing that press back.
func (a *App) completionMouse(ev *tcell.EventMouse, action tview.MouseAction) bool {
	x, y := ev.Position()
	b := a.menu.box
	inside := x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height

	if !inside || !a.mouseEnabled {
		if ev.Buttons() != tcell.ButtonNone {
			a.closeCompletion()
		}
		return false
	}

	switch action {
	case tview.MouseLeftClick:
		if at := a.menu.top + y - b.y; at < len(a.menu.candidates) {
			a.acceptCandidate(a.menu.candidates[at])
		}
	case tview.MouseScrollUp:
		a.menu.move(-1)
	case tview.MouseScrollDown:
		a.menu.move(1)
	case tview.MouseLeftUp:
		// tview decides a click only from a release nobody swallowed.
		return false
	}
	// Every other action on the menu is swallowed: a press reaching the
	// editor underneath would move the caret off the word.
	return true
}

// drawCompletion paints the menu over whatever the frame drew under the
// word, after everything else so nothing covers it.
func (a *App) drawCompletion(screen tcell.Screen) {
	a.menu.box.width, a.menu.box.height = 0, 0
	// HasFocus rather than GetFocus: the frame is drawn under the
	// application's lock, which GetFocus would wait on forever.
	if !a.menu.isOpen() || a.dialogOpen() || !a.editor.HasFocus() {
		return
	}

	x, caretY, ok := a.wordOnScreen()
	if !ok {
		return
	}

	m := &a.menu
	layout := menuLayout(m.candidates)
	labelW, width := layout.nameWidth, layout.width

	ex, _, ew, _ := a.editor.GetInnerRect()
	width = min(min(width, menuWidth), ew)
	// Cutting the detail short keeps the names lined up under the word;
	// the menu moves left only when the names themselves would not fit.
	if room := ex + ew - x; width > room {
		width = max(room, min(width, labelW+1))
	}
	x = max(ex, min(x, ex+ew-width))

	_, py, _, ph := a.rightPane.GetRect()
	rows := min(len(m.candidates), popupRows)
	below, above := py+ph-(caretY+1), caretY-py
	y := caretY + 1
	switch {
	case rows <= below:
	case rows <= above:
		y = caretY - rows
	case below >= above:
		rows = below
	default:
		rows, y = above, caretY-above
	}
	if rows <= 0 {
		return
	}
	// A menu squeezed shorter than popupRows still has to show the highlight.
	if m.selected >= m.top+rows {
		m.top = m.selected - rows + 1
	}

	// The spine's pair rather than the editor's black: with no border, rows
	// drawn in the editor's own colours read as more of the query. spineText
	// is the muted tone already chosen to be legible on spineColour.
	plainStyle := tcell.StyleDefault.Background(spineColour).Foreground(tcell.ColorWhite)
	for i := 0; i < rows; i++ {
		at := m.top + i
		if at >= len(m.candidates) {
			break
		}
		style, detail := plainStyle, plainStyle.Foreground(spineText)
		if at == m.selected {
			style = tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack)
			detail = style
		}
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, y+i, ' ', nil, style)
		}
		put(screen, layout.names[at], x, y+i, width, style)
		if labelW < width {
			put(screen, layout.trailers[at], x+labelW, y+i, width-labelW, detail)
		}
	}
	m.box.x, m.box.y, m.box.width, m.box.height = x, y, width, rows
}

// menuText is the menu's layout, measured in cells.
type menuText struct {
	// names are padded to nameWidth; trailers are each name's kind and
	// detail, which follow it in muted text.
	names, trailers []string
	nameWidth       int
	// width fits the widest row with a cell to spare.
	width int
}

func menuLayout(candidates []complete.Candidate) menuText {
	t := menuText{names: make([]string, len(candidates)), trailers: make([]string, len(candidates))}
	kindW := 0
	for _, c := range candidates {
		t.nameWidth = max(t.nameWidth, uniseg.StringWidth(oneLine(c.Text)))
		kindW = max(kindW, len(c.Kind.String()))
	}
	for i, c := range candidates {
		t.names[i] = padCells(oneLine(c.Text), t.nameWidth)
		t.trailers[i] = " " + padCells(c.Kind.String(), kindW)
		if c.Detail != "" {
			t.trailers[i] += " " + oneLine(c.Detail)
		}
		t.width = max(t.width, t.nameWidth+uniseg.StringWidth(t.trailers[i])+1)
	}
	return t
}

// padCells right-fills s to width terminal cells, which a wide character
// counts twice in.
func padCells(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-uniseg.StringWidth(s)))
}

// wordOnScreen is the screen cell where the word being completed starts,
// or false when its line has scrolled out of the editor.
//
// It counts cells as highlightedEditor does, which relies on wrapping being
// off: screen row r is buffer line rowOffset+r.
func (a *App) wordOnScreen() (x, y int, ok bool) {
	text := a.editor.GetText()
	anchor := min(a.menu.anchor, len(text))
	lineStart := strings.LastIndexByte(text[:anchor], '\n') + 1
	row := strings.Count(text[:anchor], "\n")

	column := 0
	for _, r := range text[lineStart:anchor] {
		if r == '\t' {
			column += tview.TabSize
		} else {
			column += uniseg.StringWidth(string(r))
		}
	}

	ex, ey, _, eh := a.editor.GetInnerRect()
	rowOffset, columnOffset := a.editor.GetOffset()
	x, y = ex+column-columnOffset, ey+row-rowOffset
	// Only the row decides. A caret placed without a key leaves TextArea's
	// horizontal scroll where it was, and the menu is clamped into the
	// editor afterwards anyway.
	return x, y, y >= ey && y < ey+eh
}

// put writes text from x, cell by cell, stopping at width. Drawing it
// directly rather than through tview.Print keeps a name holding "[" from
// being read as a style tag.
func put(screen tcell.Screen, text string, x, y, width int, style tcell.Style) {
	used := 0
	for _, r := range text {
		w := uniseg.StringWidth(string(r))
		if used+w > width {
			return
		}
		screen.SetContent(x+used, y, r, nil, style)
		used += w
	}
}
