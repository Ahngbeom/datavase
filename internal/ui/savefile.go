package ui

import (
	"fmt"

	"github.com/Ahngbeom/datavase/internal/keymap"
)

// saveFile writes the editor back to the file it came from.
//
// There is no "save as": a datasource client that can create files anywhere is
// a file manager, and the one thing this needs to do is put an edited
// migration back where it was found.
func (a *App) saveFile() {
	if !a.openFile.isOpen() {
		a.notice(fmt.Sprintf("no file open — %s opens one from the attached worktree",
			a.keyLabel(keymap.ActionFindFile)))
		return
	}
	if a.openFile.readOnly() {
		a.notice(fmt.Sprintf("%s came from %s — there is nowhere here to save it back to",
			a.openFile.rel, a.openFile.origin))
		return
	}
	if a.wt == nil {
		// Detaching leaves the text but takes the file with it, so there is
		// nowhere for this to go.
		a.notice("the worktree was detached; nothing to save to")
		return
	}

	// A file that changed since it was read is somebody else's work — a
	// rebase, an editor in the next window, a generator. Overwriting it
	// without asking is the one way this feature could destroy something.
	if stamp, err := a.wt.Stat(a.openFile.rel); err == nil && stamp != a.openFile.stamp {
		a.confirmDiscard(
			fmt.Sprintf("%s changed on disk since you opened it.\n\nOverwrite those changes?", a.openFile.rel),
			"Overwrite",
			a.writeFile)
		return
	}
	a.writeFile()
}

func (a *App) writeFile() {
	text := a.editor.GetText()

	stamp, err := a.wt.Write(a.openFile.rel, text)
	if err != nil {
		a.notice(fmt.Sprintf("cannot save %s: %v", a.openFile.rel, err))
		return
	}

	// The buffer is now what is on disk, which is what clears the unsaved
	// marker; the stamp moves with it so the next save compares against the
	// version this one wrote.
	a.openFile.loaded = text
	a.openFile.stamp = stamp

	// The listing carries a status per file, and saving usually changes one.
	a.rescan()

	a.notice(fmt.Sprintf("saved %s", a.openFile.rel))
}

// confirmDiscard asks before an action that throws work away.
//
// Unlike the guard's refusal there is a way through, because this is the
// user's own text and their own file — the point is that it takes a decision
// rather than a reflex.
func (a *App) confirmDiscard(text, proceedLabel string, proceed func()) {
	modal := newModal().
		SetText(text).
		AddButtons([]string{"Cancel", proceedLabel}).
		SetDoneFunc(func(_ int, label string) {
			a.closeDialog()
			if label == proceedLabel {
				proceed()
			}
		})

	modal.SetTextColor(colourNotice)
	a.openDialog(modal)
}
