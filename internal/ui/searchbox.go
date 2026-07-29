package ui

import (
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// searchItem is one row of a search dialog.
type searchItem struct {
	primary   string
	secondary string
	// accept runs when the row is chosen. A nil accept makes the row a
	// message rather than a choice.
	accept func()
}

// message builds a non-selectable row, for "nothing found" and errors.
func message(text, detail string) searchItem {
	return searchItem{primary: text, secondary: detail}
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

	input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyDown, tcell.KeyTab:
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
