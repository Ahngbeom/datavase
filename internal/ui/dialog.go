package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahngbeom/datavase/internal/guard"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"github.com/Ahngbeom/datavase/internal/vim"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// refusalText is the whole of what a refused statement says, and it is a
// plain function so a test can read it without a screen.
//
// The unlock is offered here rather than as a button. A button next to the
// refusal is the "run anyway" this dialog exists not to have; making the user
// leave, open the palette and name the command is the deliberateness that a
// production write is supposed to cost.
func refusalText(d guard.Decision, paletteKey string) string {
	text := fmt.Sprintf("Refused\n\n%s", d.Reason)
	if d.Unlockable {
		text += "\n\n" + unlockHint(paletteKey)
	}
	return text
}

// unlockHint names the route past the production write lock.
//
// guard deliberately does not compose this: it cannot know which preset is in
// force or which keys the terminal can deliver, and the reason it used to
// carry named ":write", a command no preset has ever had.
func unlockHint(paletteKey string) string {
	return fmt.Sprintf("Writes can be unlocked for this session: %s, then %q.",
		paletteKey, cmdEnableWrites)
}

// refuse tells the user why a statement will not run. There is no override
// here on purpose: a dialog offering "run anyway" is a dialog people learn
// to dismiss, which is exactly how the production accident happens.
func (a *App) refuse(d guard.Decision) {
	modal := newModal().
		SetText(refusalText(d, a.keyLabel(keymap.ActionCommandPalette))).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) { a.closeDialog() })

	modal.SetTextColor(colourDanger)
	a.openDialog(modal)
}

// confirm asks before running a statement that changes data.
//
// When the guard supplies a phrase, the user has to type it. Requiring the
// hands to spell out "DELETE" is what turns a reflex into a decision.
func (a *App) confirm(stmt sqlparse.Statement, d guard.Decision) {
	if d.TypeToConfirm == "" {
		a.confirmWithButtons(stmt, d)
		return
	}
	a.confirmByTyping(stmt, d)
}

func (a *App) confirmWithButtons(stmt sqlparse.Statement, d guard.Decision) {
	modal := newModal().
		SetText(fmt.Sprintf("%s\n\n%s\n\nRun it?", d.Reason, preview(stmt.SQL))).
		AddButtons([]string{"Cancel", "Run"}).
		SetDoneFunc(func(_ int, label string) {
			a.closeDialog()
			if label != "Run" {
				a.abandonBatch()
				return
			}
			a.start(stmt, d)
		})

	a.openDialog(modal)
}

func (a *App) confirmByTyping(stmt sqlparse.Statement, d guard.Decision) {
	a.typeToConfirm(
		fmt.Sprintf("%s\n\n%s", d.Reason, preview(stmt.SQL)),
		d.TypeToConfirm, "Run",
		func() { a.start(stmt, d) },
		a.abandonBatch)
}

// typeToConfirm asks for a word to be spelled out before doing something.
//
// Requiring the hands to type it is what turns a reflex into a decision, and
// it is the same demand wherever it appears — a production write, or stopping
// somebody else's statement.
func (a *App) typeToConfirm(message, phrase, verb string, confirm, cancel func()) {
	form := tview.NewForm()
	typed := ""

	form.AddTextView("", message, 60, 6, true, false).
		AddInputField(fmt.Sprintf("Type %s to proceed", phrase), "", 24,
			nil, func(text string) { typed = text }).
		AddButton("Cancel", func() {
			a.closeDialog()
			if cancel != nil {
				cancel()
			}
		}).
		AddButton(verb, func() {
			// Comparison is case-insensitive: the point is deliberate
			// effort, not exact keystrokes.
			if !strings.EqualFold(strings.TrimSpace(typed), phrase) {
				a.notice(fmt.Sprintf("type %s exactly to confirm", phrase))
				return
			}
			a.closeDialog()
			confirm()
		})

	form.SetBorder(true).SetTitle(" confirm ").SetTitleAlign(tview.AlignLeft)
	form.SetBackgroundColor(tcell.ColorBlack)

	a.openDialog(centred(form, 70, 15))
}

// preview shortens a statement for a dialog while keeping it recognisable.
func preview(sql string) string {
	const limit = 240

	flat := strings.Join(strings.Fields(sql), " ")
	return result.EscapeTags(result.Truncate(flat, limit))
}

// newModal is the one place a tview Modal is built, because it is the one
// place its background can be set completely.
//
// A Modal is three primitives — a Box, a Frame and a Form — and its own
// SetBackgroundColor reaches only the last two. The Box is what draws the
// border, so a Modal told to be black came out as a ring of the library's
// blue around black text. Every other dialog here draws its border on the
// background it was given; these four were the ones that did not, and one of
// them is the guard's refusal.
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
// opening: seven groups and forty commands answer "which key does X" and never
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
			keymap.ActionExplain, keymap.ActionAnalyze,
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
			keymap.ActionSaveFile,
		},
	},
	{
		title: "Finding things",
		actions: []keymap.Action{
			keymap.ActionFind, keymap.ActionFindNext, keymap.ActionFindPrev,
			keymap.ActionSearchHistory,
			keymap.ActionGoToTable, keymap.ActionFindFile,
			keymap.ActionInspect, keymap.ActionCommandPalette,
		},
	},
	{
		title: "Results",
		actions: []keymap.Action{
			keymap.ActionSortColumn, keymap.ActionSessions, keymap.ActionKillSession, keymap.ActionLocks,
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

	line := func(action keymap.Action) {
		labels := make([]string, 0, 3)
		for _, binding := range a.keys.DisplayBindings(action) {
			labels = append(labels, binding.Label(onMac))
		}
		fmt.Fprintf(&b, "  %s  %s\n",
			keymap.PadLabel(strings.Join(labels, "  "), helpKeyColumn),
			action.Describe())
	}

	b.WriteString(tag(colourAccent, "datavase") + "\n")

	b.WriteString("\n" + headingTag("Start here") + "\n")
	for _, action := range startHere {
		line(action)
	}
	b.WriteString(a.modalEscapeHatch())

	for _, group := range helpGroups {
		fmt.Fprintf(&b, "\n%s\n", headingTag(group.title))

		for _, action := range group.actions {
			line(action)
		}
	}

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

// commandHelpText lists the command palette's entries.
//
// These carry no key of their own, so without this the only way to find one is
// to already know it exists — which is how attaching a worktree, the entry
// point to a whole feature, became undiscoverable while the keys that need it
// were listed above.
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
