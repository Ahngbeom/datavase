package ui

import (
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// welcomeText is what someone sees the first time dv opens on their machine.
//
// It says what the program is for and then hands over the same keys the
// reference opens with, rendered by the same line builder, so the welcome
// cannot advertise a key the help screen has since moved.
func welcomeText(km *keymap.Map) string {
	var b strings.Builder

	b.WriteString(tag(colourAccent, "Welcome to datavase") + "\n\n")
	b.WriteString("  Write SQL in the editor, run it, and read the result as it streams in.\n" +
		"  Browse the schema, search what you ran, copy a result out.\n")

	b.WriteString("\n" + headingTag("Start here") + "\n")
	for _, action := range startHere {
		description := action.Describe()
		if action == keymap.ActionHelp {
			description = "show every key"
		}
		b.WriteString(keyReferenceLineSaying(km, action, description))
	}

	b.WriteString("\n" + tag(colourMuted, "Enter or Esc to start. This shows once."))
	return b.String()
}

// showWelcome opens the first-run greeting over the editor.
func (a *App) showWelcome() {
	text := welcomeText(a.keys)
	view := tview.NewTextView().
		SetDynamicColors(true).
		SetText(text)

	view.SetBorder(true).SetTitle(" welcome ")
	view.SetBackgroundColor(tcell.ColorBlack)
	view.SetDoneFunc(func(tcell.Key) {
		a.pages.RemovePage(pageWelcome)
		a.app.SetFocus(a.editor)
	})

	a.pages.AddPage(pageWelcome, centredText(view, text, 84, 20), true, true)
	a.app.SetFocus(view)
}
