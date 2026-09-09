package ui

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newModal is the one place a tview Modal is built, because it is the one
// place its background can be set completely.
//
// A Modal is three primitives — a Box, a Frame and a Form — and its own
// SetBackgroundColor reaches only the last two. The Box is what draws the
// border, so a Modal told to be black came out as a ring of the library's
// blue around black text. Every other dialog here draws its border on the
// background it was given; these were the ones that did not.
func newModal() *tview.Modal {
	modal := tview.NewModal()
	modal.SetBackgroundColor(tcell.ColorBlack)
	// The embedded Box, which SetBackgroundColor above does not reach.
	modal.Box.SetBackgroundColor(tcell.ColorBlack)
	return modal
}

func (a *App) openDialog(p tview.Primitive) {
	a.pages.AddPage(pageConfirm, p, true, true)
	a.app.SetFocus(p)
}

func (a *App) closeDialog() {
	a.pages.RemovePage(pageConfirm)
	a.app.SetFocus(a.editor)
}

// centred floats a primitive in the middle of the screen at its preferred
// size, shrinking it when the terminal is smaller.
//
// The sizes passed in are maxima, not requirements: a dialog that insists on
// 80×34 is simply clipped on a smaller terminal, which is how the key
// reference came to be cut off.
func centred(p tview.Primitive, width, height int) tview.Primitive {
	return newFitted(p, width, height)
}

// centredText is centred for a dialog whose whole contents are known, so the
// box can be as tall as they are rather than as tall as it was allowed to be.
func centredText(p tview.Primitive, text string, width, height int) tview.Primitive {
	return newFitted(p, width, height).
		sizedTo(func(w int) int { return dialogHeight(text, w) })
}

// startHere is the whole of what someone needs on the first screenful.
//
// The reference below is complete, which is what makes it useless as an
// opening: six groups and thirty commands answer "which key does X" and never
// answer "what do I do now". These five do, and each appears again in its own
// group further down — the repetition is the point, not an oversight.
//
// Switching datasource is here because it is the one action with no other
// way to discover it exists, and quit because a beginner's first question
// about an unfamiliar full-screen program is how to get out of it.
var startHere = []keymap.Action{
	keymap.ActionRun,
	keymap.ActionToggleSidebar,
	keymap.ActionSwitchDataSource,
	keymap.ActionHelp,
	keymap.ActionQuit,
}

// helpGroups organise the key reference. Actions appear in this order.
var helpGroups = []struct {
	title   string
	actions []keymap.Action
}{
	{
		title: "Running",
		actions: []keymap.Action{
			keymap.ActionRun, keymap.ActionRunAll, keymap.ActionCancel,
		},
	},
	{
		title: "Cursor",
		actions: []keymap.Action{
			keymap.ActionWordLeft, keymap.ActionWordRight,
			keymap.ActionSelectWordLeft, keymap.ActionSelectWordRight,
			keymap.ActionLineStart, keymap.ActionLineEnd,
			keymap.ActionSelectLineStart, keymap.ActionSelectLineEnd,
		},
	},
	{
		title: "Editing",
		actions: []keymap.Action{
			keymap.ActionComplete,
			keymap.ActionCopyOrCancel, keymap.ActionCut, keymap.ActionPaste,
			keymap.ActionSelectAll, keymap.ActionToggleComment,
			keymap.ActionDuplicateLine, keymap.ActionDeleteLine,
			keymap.ActionDeleteWordLeft, keymap.ActionDeleteToLineStart,
		},
	},
	{
		title: "Finding things",
		actions: []keymap.Action{
			keymap.ActionFind, keymap.ActionFindNext, keymap.ActionFindPrev,
			keymap.ActionSearchHistory,
			keymap.ActionInspect,
		},
	},
	{
		title:   "Results",
		actions: []keymap.Action{keymap.ActionSortColumn, keymap.ActionCopyResult},
	},
	{
		title: "Moving around",
		actions: []keymap.Action{
			keymap.ActionNextPane, keymap.ActionPrevPane, keymap.ActionCycleTab,
			keymap.ActionToggleSidebar, keymap.ActionUseSchema,
			keymap.ActionSwitchDataSource, keymap.ActionRefreshSchema,
		},
	},
	{
		title:   "Other",
		actions: []keymap.Action{keymap.ActionHelp, keymap.ActionQuit},
	},
}

// helpText renders the key reference from the live key map.
//
// Generating it from the map rather than writing it out is what keeps the
// help honest if a binding in baseMap ever changes — a hardcoded list would
// start lying to precisely the people who need it.
func (a *App) helpText() string {
	var b strings.Builder

	b.WriteString(tag(colourAccent, "datavase") + "\n")

	b.WriteString("\n" + headingTag("Start here") + "\n")
	for _, action := range startHere {
		b.WriteString(keyReferenceLine(a.keys, action))
	}

	b.WriteString(helpReference(a.keys))

	b.WriteString("\n  Enter in the schema tree expands it, or pastes a column name.\n" +
		"  Double-click a table there, or Enter on one in the tables tab, to see its first 100 rows.\n")

	b.WriteString("\n" + tag(colourMuted, "If ⌘↩ or Ctrl+↩ does nothing here, F5 runs: some terminals keep modified keys.") + "\n")

	b.WriteString("\n" + tag(colourMuted, "Press Escape to close."))
	return b.String()
}

// keyReferenceLine is shared by helpText's opening list and helpReference's
// groups, so the two render one action's row identically instead of each
// carrying its own closure that could drift from the other.
func keyReferenceLine(km *keymap.Map, action keymap.Action) string {
	labels := make([]string, 0, 3)
	for _, binding := range km.DisplayBindings(action) {
		labels = append(labels, binding.Label(onMac))
	}
	return fmt.Sprintf("  %s  %s\n",
		keymap.PadLabel(strings.Join(labels, "  "), helpKeyColumn),
		action.Describe())
}

// helpReference renders every key group in full.
//
// It takes the map rather than reading the App's so the reference can be
// checked without building an interface: what it renders depends only on
// that map and the package-level onMac, neither of which needs a terminal.
func helpReference(km *keymap.Map) string {
	var b strings.Builder
	for _, group := range helpGroups {
		fmt.Fprintf(&b, "\n%s\n", headingTag(group.title))
		for _, action := range group.actions {
			b.WriteString(keyReferenceLine(km, action))
		}
	}
	return b.String()
}

// helpKeyColumn is the width of the key column on the help screen.
const helpKeyColumn = 20

func (a *App) showHelp() {
	text := a.helpText()
	view := tview.NewTextView().
		SetDynamicColors(true).
		SetText(text)

	// The reference no longer fits a modest terminal, and a pane that scrolls
	// without saying so reads as one that is simply cut off — which is what
	// this dialog was doing before.
	view.SetBorder(true).SetTitle(" keys — ↑↓ scroll · Esc close ")
	view.SetBackgroundColor(tcell.ColorBlack)
	view.SetDoneFunc(func(tcell.Key) {
		a.pages.RemovePage(pageHelp)
		a.app.SetFocus(a.editor)
	})

	a.pages.AddPage(pageHelp, centredText(view, text, 84, 40), true, true)
	a.app.SetFocus(view)
}
