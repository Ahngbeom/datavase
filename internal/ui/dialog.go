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
	modal := tview.NewModal().
		SetText(refusalText(d, a.keyLabel(keymap.ActionCommandPalette))).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) { a.closeDialog() })

	modal.SetBackgroundColor(tcell.ColorBlack)
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
	modal := tview.NewModal().
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

	modal.SetBackgroundColor(tcell.ColorBlack)
	a.openDialog(modal)
}

func (a *App) confirmByTyping(stmt sqlparse.Statement, d guard.Decision) {
	form := tview.NewForm()
	typed := ""

	form.AddTextView("", fmt.Sprintf("%s\n\n%s", d.Reason, preview(stmt.SQL)), 60, 6, true, false).
		AddInputField(fmt.Sprintf("Type %s to proceed", d.TypeToConfirm), "", 24,
			nil, func(text string) { typed = text }).
		AddButton("Cancel", func() {
			a.closeDialog()
			a.abandonBatch()
		}).
		AddButton("Run", func() {
			// Comparison is case-insensitive: the point is deliberate
			// effort, not exact keystrokes.
			if !strings.EqualFold(strings.TrimSpace(typed), d.TypeToConfirm) {
				a.notice(fmt.Sprintf("type %s exactly to confirm", d.TypeToConfirm))
				return
			}
			a.closeDialog()
			a.start(stmt, d)
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

// helpGroups organise the key reference. Actions appear in this order.
var helpGroups = []struct {
	title   string
	actions []keymap.Action
}{
	{
		title: "Running",
		actions: []keymap.Action{
			keymap.ActionRun, keymap.ActionRunAll, keymap.ActionCancel,
			keymap.ActionExplain,
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

	b.WriteString("[aqua]datavase[-]\n")

	for _, group := range helpGroups {
		fmt.Fprintf(&b, "\n[yellow]%s[-]\n", group.title)

		for _, action := range group.actions {
			labels := make([]string, 0, 3)
			for _, binding := range a.keys.DisplayBindings(action) {
				labels = append(labels, binding.Label(onMac))
			}
			fmt.Fprintf(&b, "  %s  %s\n",
				keymap.PadLabel(strings.Join(labels, "  "), helpKeyColumn),
				action.Describe())
		}
	}

	b.WriteString(commandHelpText(a.keyLabel(keymap.ActionCommandPalette)))
	b.WriteString(a.vimHelp())
	b.WriteString("\n  Enter in the schema tree expands it, or pastes a column name.\n")

	if advice := keymap.TerminalAdvice(os.Getenv("TERM"), a.keys); advice != "" {
		fmt.Fprintf(&b, "\n[yellow]%s[-]\n", result.EscapeTags(advice))
	}
	if onMac {
		b.WriteString("\n[gray]⌘ bindings need the terminal to forward them:\n" +
			"run `dv keys --ghostty` or `dv keys --iterm2` outside datavase.[-]\n")
	}

	b.WriteString("\n[gray]Press Escape to close.[-]")
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

	fmt.Fprintf(&b, "\n[yellow]Commands — %s, then type[-]\n", result.EscapeTags(paletteKey))
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
		fmt.Fprintf(&b, "\n[yellow]%s[-]\n", group.Title)
		for _, entry := range group.Entries {
			fmt.Fprintf(&b, "  %s  %s\n",
				keymap.PadLabel(entry.Keys, helpKeyColumn), entry.Description)
		}
	}

	b.WriteString("\n[gray]Not what you wanted? Put `keymap: {preset: datagrip}` in\n" +
		"~/.config/datavase/config.yaml, or run `keymap datagrip` from the\n" +
		"command palette to switch for this session.[-]\n")
	return b.String()
}

// helpKeyColumn is the width of the key column on the help screen.
const helpKeyColumn = 20

func (a *App) showHelp() {
	view := tview.NewTextView().
		SetDynamicColors(true).
		SetText(a.helpText())

	// The reference no longer fits a modest terminal, and a pane that scrolls
	// without saying so reads as one that is simply cut off — which is what
	// this dialog was doing before.
	view.SetBorder(true).SetTitle(" keys — ↑↓ scroll · Esc close ")
	view.SetBackgroundColor(tcell.ColorBlack)
	view.SetDoneFunc(func(tcell.Key) {
		a.pages.RemovePage(pageHelp)
		a.app.SetFocus(a.editor)
	})

	a.pages.AddPage(pageHelp, centred(view, 84, 40), true, true)
	a.app.SetFocus(view)
}
