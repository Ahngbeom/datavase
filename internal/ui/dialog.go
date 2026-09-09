package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/vim"
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
// The palette is here because it is the one key that finds everything else,
// and quit because a beginner's first question about an unfamiliar full-screen
// program is how to get out of it.
var startHere = []keymap.Action{
	keymap.ActionRun,
	keymap.ActionToggleSidebar,
	keymap.ActionCommandPalette,
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
			keymap.ActionInspect, keymap.ActionCommandPalette,
		},
	},
	{
		title: "Results",
		actions: []keymap.Action{
			keymap.ActionSortColumn,
		},
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
// Generating it rather than writing it out is what keeps the help honest
// after a user rebinds something in configuration — a hardcoded list would
// start lying to precisely the people who need it.
func (a *App) helpText() string {
	var b strings.Builder

	b.WriteString(tag(colourAccent, "datavase") + "\n")

	b.WriteString("\n" + headingTag("Start here") + "\n")
	for _, action := range startHere {
		b.WriteString(keyReferenceLine(a.keys, action))
	}
	b.WriteString(a.modalEscapeHatch())

	b.WriteString(helpReference(a.keys))

	b.WriteString(commandHelpText(a.keyLabel(keymap.ActionCommandPalette)))
	b.WriteString(a.vimHelp())
	b.WriteString("\n  Enter in the schema tree expands it, or pastes a column name.\n")

	if advice := keymap.TerminalAdvice(os.Getenv("TERM"), a.keys); advice != "" {
		fmt.Fprintf(&b, "\n%s\n", tag(colourNotice, advice))
	}
	if onMac {
		b.WriteString("\n" + tag(colourMuted, "⌘ bindings need the terminal to forward them:\n"+
			"run `dv keys --ghostty` or `dv keys --iterm2` outside datavase.") + "\n")
	}

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

// helpReference renders the key groups and, below them, the keys a reader
// already knows.
//
// It takes the map rather than reading the App's so the reference can be
// checked without building an interface: what it renders depends only on
// that map and the package-level onMac, neither of which needs a terminal.
func helpReference(km *keymap.Map) string {
	var b strings.Builder
	var known []keymap.Action

	for _, group := range helpGroups {
		ours, groupKnown := keymap.SplitByFamiliarity(group.actions)
		known = append(known, groupKnown...)
		if len(ours) == 0 {
			continue
		}

		fmt.Fprintf(&b, "\n%s\n", headingTag(group.title))
		for _, action := range ours {
			b.WriteString(keyReferenceLine(km, action))
		}
	}

	fmt.Fprintf(&b, "\n%s\n", headingTag("Already what you expect"))
	for _, line := range keymap.PackFamiliar(km, known, onMac, familiarWidth) {
		fmt.Fprintf(&b, "  %s\n", line)
	}

	return b.String()
}

// commandHelpText lists the command palette's entries.
//
// These carry no key of their own, so without this the only way to find one
// is to already know it exists.
//
// It is generated from the same list the palette offers, so the two cannot
// drift apart, and it takes the palette's key label rather than the App so the
// section can be rendered in a test without a terminal.
func commandHelpText(paletteKey string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n%s\n", headingTag("Commands — "+result.EscapeTags(paletteKey)+", then type"))
	for _, c := range paletteCommands() {
		fmt.Fprintf(&b, "  %s  %s\n",
			keymap.PadLabel(result.EscapeTags(c.name), helpKeyColumn), result.EscapeTags(c.summary))
	}
	return b.String()
}

// vimHelp renders the modal commands, and the way out of them.
//
// The escape hatch is not an afterthought: someone who did not choose a modal
// editor and cannot type into it needs to be told how to leave in the same
// place they went looking for help.
func (a *App) vimHelp() string {
	if !a.keys.Modal() {
		return ""
	}

	var b strings.Builder
	for _, group := range vim.Reference() {
		fmt.Fprintf(&b, "\n%s\n", headingTag(group.Title))
		for _, entry := range group.Entries {
			fmt.Fprintf(&b, "  %s  %s\n",
				keymap.PadLabel(entry.Keys, helpKeyColumn), entry.Description)
		}
	}

	b.WriteString("\n" + tag(colourMuted, "To keep this keyboard, put `keymap: {preset: vim}` in\n"+
		"~/.config/datavase/config.yaml.") + "\n")
	return b.String()
}

// modalEscapeHatch is the way out of a modal editor nobody asked for.
//
// It sits under "Start here" rather than at the foot of the vim reference,
// which is where it used to be. Someone who cannot type into the editor opens
// the help and reads the top of it; putting the answer past forty keys and a
// full vim table meant scrolling through the thing they were trying to leave
// to find out that they could.
func (a *App) modalEscapeHatch() string {
	if !a.keys.Modal() {
		return ""
	}
	return "\n" + tag(colourMuted, fmt.Sprintf(
		"Typing does nothing? This editor is modal — press i first.\n"+
			"For an ordinary editor: %s, then \"keymap datagrip\".",
		a.keyLabel(keymap.ActionCommandPalette))) + "\n"
}

// helpKeyColumn is the width of the key column on the help screen.
const helpKeyColumn = 20

// familiarWidth keeps the packed block from wrapping on an eighty-column
// terminal, the narrowest this reference is meant to be read on. A wrapped
// line costs two rows, which undoes the packing it wrapped. At that size the
// dialog is 80-2*dialogMargin wide, its border takes two columns and the
// block is indented two.
const familiarWidth = 80 - 2*dialogMargin - 2 - 2

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
