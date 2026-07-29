package ui

import "github.com/gdamore/tcell/v2"

// Modal navigation outside the editor.
//
// tview's tree, table and text views already answer to j, k, g and G — its
// list does not, and the tables pane is a list. Rather than reimplement
// movement, the keys are translated into the ones the widget already handles,
// so the two can never disagree about what "the end" means.

// vimListKey translates j/k/gg/G for a list, returning the key the widget
// should see, or nil when the key was consumed.
func (a *App) vimListKey(ev *tcell.EventKey) *tcell.EventKey {
	if !a.keys.Modal() || ev.Key() != tcell.KeyRune {
		a.listPending = 0
		return ev
	}

	r := ev.Rune()

	// "gg" is the only two-key sequence a list needs.
	if a.listPending == 'g' {
		a.listPending = 0
		if r == 'g' {
			return plainKey(tcell.KeyHome)
		}
		return nil
	}

	switch r {
	case 'j':
		return plainKey(tcell.KeyDown)
	case 'k':
		return plainKey(tcell.KeyUp)
	case 'G':
		return plainKey(tcell.KeyEnd)
	case 'g':
		a.listPending = 'g'
		return nil
	}
	return ev
}

func plainKey(key tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(key, 0, tcell.ModNone)
}
