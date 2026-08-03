package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// cmdIntent is what a ":" line turned out to mean.
type cmdIntent int

const (
	// cmdNothing is an empty line, which closes the prompt and says nothing.
	cmdNothing cmdIntent = iota
	// cmdUnknown named nothing at all; cmdAmbiguous named more than one.
	cmdUnknown
	cmdAmbiguous
	cmdSave
	cmdQuit
	cmdForceQuit
	cmdSaveQuit
	cmdEdit
	cmdPalette
)

// cmdResolution is a ":" line worked out, before anything has been done about
// it.
//
// Resolution is separated from execution because it is where the whole risk
// of a command line lives: everything below decides which of two opposite
// things a half-typed word meant. That decision is worth being able to test
// as a table, without a terminal and without anything having run.
type cmdResolution struct {
	intent cmdIntent
	// arg is what followed the verb, for ":e path".
	arg string
	// name is the palette command to run, for cmdPalette.
	name string
	// among lists what an ambiguous line could have meant, so the refusal can
	// say rather than merely refuse.
	among []string
}

// vimFileCommands are the ones a vim user types without deciding to.
//
// They are matched whole and ahead of the palette, and they are spelled out
// rather than abbreviated at resolution time: a reflex must not depend on
// what else happens to be in the command list.
var vimFileCommands = map[string]cmdIntent{
	"w": cmdSave, "write": cmdSave,
	"q": cmdQuit, "quit": cmdQuit,
	"q!": cmdForceQuit, "quit!": cmdForceQuit,
	"wq": cmdSaveQuit, "wq!": cmdSaveQuit, "x": cmdSaveQuit, "xit": cmdSaveQuit,
	"e": cmdEdit, "edit": cmdEdit,
}

// resolveCommandLine works out what was typed at the ":" prompt.
//
// An abbreviation runs only while it names exactly one command. The palette
// can afford to guess, because it shows the row it picked and Enter is a
// choice from a visible list; a command line runs the moment Enter is pressed,
// and "c" alone stands in front of both "cancel" and "commit". Refusing is
// the same instinct the guard already has about a statement it cannot
// classify.
func resolveCommandLine(line string, cmds []command) cmdResolution {
	line = strings.TrimSpace(line)
	if line == "" {
		return cmdResolution{intent: cmdNothing}
	}

	verb, rest, _ := strings.Cut(line, " ")
	if intent, ok := vimFileCommands[verb]; ok {
		return cmdResolution{intent: intent, arg: strings.TrimSpace(rest)}
	}

	// An exact command still counts as a candidate here even though it can
	// never be the answer. Leaving it out would not merely hide it: it would
	// make everything sharing its first letters easier to reach than before,
	// so ":u" would stop being the ambiguity it is and quietly become "use
	// schema". Hiding a dangerous command must not change what the safe ones
	// mean.
	var prefixed []command
	for _, c := range cmds {
		if c.name == line {
			return cmdResolution{intent: cmdPalette, name: c.name}
		}
		if strings.HasPrefix(c.name, line) {
			prefixed = append(prefixed, c)
		}
	}

	switch {
	case len(prefixed) == 0 || (len(prefixed) == 1 && prefixed[0].exact):
		return cmdResolution{intent: cmdUnknown}
	case len(prefixed) == 1:
		return cmdResolution{intent: cmdPalette, name: prefixed[0].name}
	default:
		among := make([]string, len(prefixed))
		for i, c := range prefixed {
			among[i] = c.name
		}
		sort.Strings(among)
		return cmdResolution{intent: cmdAmbiguous, among: among}
	}
}

// showCommandLine opens the ":" prompt on the bottom row.
//
// It is docked rather than centred for the same reason the search prompt is:
// a command line that covers the text it is about to act on is one you cannot
// check yourself against.
func (a *App) showCommandLine() {
	input := tview.NewInputField().SetLabel(":")
	input.SetFieldBackgroundColor(tcell.ColorBlack)

	dismiss := func() {
		a.pages.RemovePage(pageCommand)
		a.app.SetFocus(a.editor)
	}

	input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			// Completing does not run: the point of finishing a name for
			// someone is to let them read it before they commit to it, which
			// matters most for the command this refuses to abbreviate.
			if whole, ok := completeCommand(input.GetText(), paletteCommands()); ok {
				input.SetText(whole)
			}
			return nil

		case tcell.KeyEnter:
			line := input.GetText()
			// Dismissed first: everything below can put a dialog or a notice
			// on the screen, and the prompt sits over the status bar.
			dismiss()
			a.runCommandLine(line)
			return nil

		case tcell.KeyEscape:
			dismiss()
			return nil
		}
		return ev
	})

	a.pages.AddPage(pageCommand, newDocked(input, 1), true, true)
	a.app.SetFocus(input)
}

// completeCommand finishes a name while only one command answers to it.
func completeCommand(typed string, cmds []command) (string, bool) {
	typed = strings.TrimSpace(typed)
	if typed == "" {
		return "", false
	}

	var found string
	for _, c := range cmds {
		if !strings.HasPrefix(c.name, typed) {
			continue
		}
		if found != "" {
			return "", false
		}
		found = c.name
	}
	return found, found != ""
}

// runCommandLine carries out what the prompt resolved to.
func (a *App) runCommandLine(line string) {
	r := resolveCommandLine(line, paletteCommands())

	switch r.intent {
	case cmdNothing:

	case cmdSave:
		a.saveFile()

	case cmdQuit:
		a.quit()

	case cmdForceQuit:
		a.forceQuit()

	case cmdSaveQuit:
		// Saving can put a dialog up — a file changed underneath, or nothing
		// open to save to — and quitting through it would answer that dialog
		// for the user. So the quit only happens once the buffer is genuinely
		// on disk.
		a.saveThenQuit()

	case cmdEdit:
		a.editCommand(r.arg)

	case cmdPalette:
		for _, c := range paletteCommands() {
			if c.name == r.name {
				c.run(a)
				return
			}
		}

	case cmdAmbiguous:
		a.notice(fmt.Sprintf("%q could be %s — type enough to tell them apart",
			strings.TrimSpace(line), strings.Join(r.among, ", ")))

	case cmdUnknown:
		a.notice(fmt.Sprintf("no command %q — %s lists them",
			strings.TrimSpace(line), a.keyLabel(keymap.ActionCommandPalette)))
	}
}

// saveThenQuit is ":wq".
//
// The quit is decided after the save rather than queued behind it, because
// saving can stop to ask: the file may have changed underneath, or there may
// be nowhere to save to at all. Quitting through that question would answer
// it on the user's behalf and take the work with it, so anything still
// unsaved keeps the session open and leaves the dialog to be answered.
func (a *App) saveThenQuit() {
	a.saveFile()
	if !a.openFile.isOpen() || a.fileDirty() {
		return
	}
	a.quit()
}

// editCommand is ":e". Bare, it opens the finder; with a path, that file.
//
// The path has to name a file in the worktree exactly. Loading a near miss
// would replace the buffer, and the buffer is the only copy of whatever has
// been typed into it — the finder is right there for anyone who wants to
// choose from a list.
func (a *App) editCommand(rel string) {
	if rel == "" {
		a.showFindFile()
		return
	}
	if a.wt == nil {
		a.notice(fmt.Sprintf("no worktree attached — nothing to open %s from", rel))
		return
	}

	a.rescan()
	for _, f := range a.wtSnap.Files {
		if f.Rel == rel {
			a.openWorktreeFile(f)
			return
		}
	}
	a.notice(fmt.Sprintf("no %s in %s", rel, a.worktreeLabel()))
}
