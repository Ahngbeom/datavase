package ui

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/intro"
	"github.com/Ahngbeom/datavase/internal/keymap"
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

	fmt.Fprintf(&b, "%s — connected to %s\n\n",
		tag(colourAccent, "datavase"), tag(colourAccent, a.conn.DataSource().Name))

	for _, action := range startHere {
		fmt.Fprintf(&b, "  %s  %s\n",
			keymap.PadLabel(a.keyLabel(action), introKeyColumn), action.Describe())
	}

	// What the guard will do is the one thing about this session that cannot be
	// worked out by pressing keys, and it is the reason the rest of this exists.
	fmt.Fprintf(&b, "\n%s\n", tag(colourNotice, a.introGuardLine()))

	if a.keys.Modal() {
		b.WriteString("\n" + tag(colourMuted, "The editor is modal: press i before you type, Esc to leave.") + "\n")
	}

	b.WriteString("\n" + tag(colourMuted, fmt.Sprintf("Enter to start · %s then %q to read this again.",
		a.keyLabel(keymap.ActionCommandPalette), cmdGettingStarted)))
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
	view.SetInputCapture(a.introKey)

	a.pages.AddPage(pageIntro, centred(view, 76, 18), true, true)
	// This is what hands the card the keyboard when it is reopened from the
	// palette, over an interface that is already running. On the first run it
	// is redundant rather than load-bearing: Run's SetRoot takes the focus
	// afterwards and Pages gives it to the front page, which is this one.
	a.app.SetFocus(view)
}

// introKey puts the card away on any key, and then does what that key names.
//
// A dialog in front suspends the global bindings, which is right for every
// other dialog and exactly wrong for one whose contents are a list of keys.
// Someone reading "F1 show this help", pressing F1 and watching nothing happen
// has met the failure the empty-editor placeholder and the opening notice both
// exist to prevent — from the screen that was put there to prevent it.
func (a *App) introKey(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	// Moving within the card is not a decision to leave it. On a terminal too
	// small for the whole thing, closing on the very key that would have shown
	// the rest is how the card becomes unreadable.
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn:
		return ev
	}

	a.closeIntro()

	// A key bound to nothing — Enter and Escape on every preset, which is why
	// the card offers them as the way out — dispatches to nothing and has
	// already done its job by getting here.
	a.dispatch(a.keys.Lookup(ev))
	return nil
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
