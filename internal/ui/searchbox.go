package ui

import (
	"sort"

	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// searchItem is one row of a search dialog.
type searchItem struct {
	primary   string
	secondary string
	// complete is what Tab puts back in the field, for a row that can be
	// refined further rather than only chosen. Directory rows set it, which is
	// what lets a path be built one segment at a time instead of typed out in
	// full. An empty complete leaves Tab doing what it has always done.
	complete string
	// accept runs when the row is chosen. A nil accept makes the row a
	// message rather than a choice.
	accept func()
}

// message builds a non-selectable row, for "nothing found" and errors.
func message(text, detail string) searchItem {
	return searchItem{primary: text, secondary: detail}
}

// ranked pairs a row with what decides where it sorts.
//
// Ordering is not presentation here: Enter takes the first row, so a worse
// match sorting above a better one runs the wrong command.
type ranked struct {
	item searchItem
	// tier separates kinds of match that no score should be able to cross —
	// a name match always beats a summary match, however well the summary
	// scored.
	tier  int
	score int
}

// sortRanked orders the rows and drops the scores.
//
// The sort is stable so that an empty term, which scores everything the same,
// leaves the caller's own order alone.
func sortRanked(rows []ranked) []searchItem {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].tier != rows[j].tier {
			return rows[i].tier > rows[j].tier
		}
		return rows[i].score > rows[j].score
	})

	items := make([]searchItem, len(rows))
	for i, r := range rows {
		items[i] = r.item
	}
	return items
}

// truncatedNotice admits that a listing stopped early.
//
// A list that was cut looks exactly like a short one, and the difference
// matters when the thing being looked for is what got cut. Shared so that
// every dialog with a limit says so — the directory list used to cut silently
// at forty.
func truncatedNotice(detail string) searchItem {
	return message("…the listing was cut short", detail)
}

// newSearchBox builds the "type to filter, arrow down to choose" pairing used
// by the history and go-to-table dialogs.
//
// tview's InputField reports Enter, Tab and Escape through SetDoneFunc but not
// the arrow keys, so moving from the search field into the results — which is
// what everyone reaches for first — has to be wired explicitly.
func (a *App) newSearchBox(label, title, page string, search func(term string) []searchItem) tview.Primitive {
	input := tview.NewInputField().SetLabel(label)
	list := tview.NewList().ShowSecondaryText(true).SetHighlightFullLine(true)

	var items []searchItem

	reload := func(term string) {
		items = search(term)

		// A list where nothing has a second line must not reserve one: every
		// row would cost two, halving how many choices fit on screen. That is
		// what pushed the last command of the palette off the bottom.
		secondLine := false
		for _, it := range items {
			if it.secondary != "" {
				secondLine = true
				break
			}
		}
		list.ShowSecondaryText(secondLine)

		list.Clear()
		for _, it := range items {
			item := it
			list.AddItem(result.EscapeTags(item.primary), result.EscapeTags(item.secondary), 0, func() {
				if item.accept != nil {
					item.accept()
				}
			})
		}
	}
	reload("")

	input.SetChangedFunc(reload)

	// complete puts a row's refinement back in the field. It reports whether
	// there was one, so the caller can fall back to whatever the key did
	// before.
	complete := func(index int) bool {
		if index < 0 || index >= len(items) || items[index].complete == "" {
			return false
		}
		input.SetText(items[index].complete)
		a.app.SetFocus(input)
		return true
	}

	input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			// Nothing is highlighted while the field has focus, so Tab takes
			// the first row — the same one Enter would.
			if complete(0) {
				return nil
			}
			fallthrough

		case tcell.KeyDown:
			if list.GetItemCount() > 0 {
				a.app.SetFocus(list)
			}
			return nil

		case tcell.KeyEnter:
			// Enter from the field takes the first result: the list is
			// ordered by relevance, so that is the obvious choice.
			if len(items) > 0 && items[0].accept != nil {
				items[0].accept()
			}
			return nil

		case tcell.KeyEscape:
			a.closeSearchBox(page)
			return nil
		}
		return ev
	})

	list.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyEscape:
			a.closeSearchBox(page)
			return nil
		case tcell.KeyTab:
			// Completing from the list hands typing back, because the point of
			// completing a path is to carry on typing the rest of it.
			if complete(list.GetCurrentItem()) {
				return nil
			}
		case tcell.KeyUp:
			// Stepping above the first entry returns to typing rather than
			// wrapping to the bottom, which is disorienting in a search.
			if list.GetCurrentItem() == 0 {
				a.app.SetFocus(input)
				return nil
			}
		}
		return ev
	})

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(input, 1, 0, true).
		AddItem(list, 0, 1, false)
	layout.SetBorder(true).SetTitle(title)
	layout.SetBackgroundColor(tcell.ColorBlack)

	// Typing has to work the moment the dialog opens.
	a.app.SetFocus(input)
	return layout
}

func (a *App) closeSearchBox(page string) {
	a.pages.RemovePage(page)
	a.app.SetFocus(a.editor)
}
