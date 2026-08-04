package ui

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/intro"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showIntroOnce puts the first-run card up, if this is the first run.
//
// The card answers the question the interface itself cannot: an empty editor,
// an empty result pane and a status bar of abbreviations say nothing about
// what to press. The key reference answers "which key does X" and has to be
// found first, which is the same problem one step further back.
func (a *App) showIntroOnce() {
	if a.introPath == "" || intro.Seen(a.introPath) {
		return
	}
	a.showIntro()
}

// introKeyColumn is the width of the key column on the card. Narrower than the
// key reference's, because this list is five rows rather than forty and does
// not have to line up with anything else.
const introKeyColumn = 12

// introText is the card, generated from the live key map.
//
// Written out it would say F5 to someone who had rebound it — and this is the
// first thing they read, so it is the worst place to be wrong. Same reason the
// key reference is generated.
func (a *App) introText() string {
	var b strings.Builder

	fmt.Fprintf(&b, "[aqua]datavase[-] — connected to [%s]%s[-]\n\n",
		colourAccent, result.EscapeTags(a.conn.DataSource().Name))

	for _, action := range startHere {
		fmt.Fprintf(&b, "  %s  %s\n",
			keymap.PadLabel(a.keyLabel(action), introKeyColumn), action.Describe())
	}

	// What the guard will do is the one thing about this session that cannot be
	// worked out by pressing keys, and it is the reason the rest of this exists.
	fmt.Fprintf(&b, "\n[%s]%s[-]\n", colourNotice, a.introGuardLine())

	if a.keys.Modal() {
		b.WriteString("\n[gray]The editor is modal: press i before you type, Esc to leave.[-]\n")
	}

	fmt.Fprintf(&b, "\n[gray]Enter to start · %s then \"%s\" to read this again.[-]",
		result.EscapeTags(a.keyLabel(keymap.ActionCommandPalette)), cmdGettingStarted)
	return b.String()
}

// introGuardLine says what will happen to a statement that changes data.
//
// It is the guard's own rule rather than a paraphrase: guard.Evaluate keys off
// EnvProd alone, so stage and dev get the same sentence, and promising a stage
// database something the guard does not do would be worse than saying nothing.
func (a *App) introGuardLine() string {
	if a.conn.DataSource().Env == config.EnvProd {
		return "This is a production database: statements that change data are refused " +
			"until you unlock writes for the session."
	}
	return "Statements that change data ask before they run."
}

func (a *App) showIntro() {
	view := tview.NewTextView().
		SetDynamicColors(true).
		SetText(a.introText())

	view.SetBorder(true).SetTitle(" welcome ")
	view.SetBackgroundColor(tcell.ColorBlack)
	view.SetDoneFunc(func(tcell.Key) { a.closeIntro() })

	a.pages.AddPage(pageIntro, centred(view, 76, 18), true, true)
	a.app.SetFocus(view)
}

// closeIntro puts the card away and records that it was shown.
//
// A marker that could not be written is not worth a message: the cost is that
// the card appears once more, and the alternative — interrupting someone who
// has just asked to start work, to tell them about a file they did not know
// existed — is worse than the thing it reports.
func (a *App) closeIntro() {
	a.pages.RemovePage(pageIntro)
	a.app.SetFocus(a.editor)

	// Swallowed rather than reported: a read-only state directory costs the
	// once-only-ness of this card and nothing else, and there is nowhere to say
	// so that would not be another interruption in its place.
	_ = intro.MarkSeen(a.introPath)
}
