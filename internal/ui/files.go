package ui

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/match"
	"github.com/Ahngbeom/datavase/internal/worktree"
)

// openFile is the file the editor was loaded from.
//
// loaded holds the text as it was read or last written, which is the only way
// to tell an edited buffer from an untouched one — tview's TextArea reports
// no such thing.
type openFile struct {
	rel    string
	loaded string
	stamp  worktree.Stamp
	// origin names where the text came from when it did not come from the
	// attached worktree — a merge request, a snippet. Empty means the
	// worktree, which is the only place anything can be saved back to.
	origin string
}

func (f openFile) isOpen() bool { return f.rel != "" }

// readOnly reports that there is nowhere to save this buffer back to.
//
// Refusing at save time with a reason beats refusing with "no worktree
// attached", which is true but sends the reader to attach one and try again.
func (f openFile) readOnly() bool { return f.origin != "" }

// fileDirty reports whether the buffer has diverged from the file behind it.
//
// It is computed rather than tracked. The status bar reads it at draw time and
// the quit path reads it once, so there is no keystroke handler that could
// forget to set a flag — the same reason the vim mode is read this way.
func (a *App) fileDirty() bool {
	return a.openFile.isOpen() && a.editor.GetText() != a.openFile.loaded
}

// showFindFile offers the attached worktree's SQL files.
//
// The listing is refreshed on open rather than per keystroke: git answers in
// milliseconds but not in microseconds, and running it on every letter typed
// would make the filter stutter.
func (a *App) showFindFile() {
	if a.wt == nil {
		// There is nothing to list, and attaching is the only useful thing to
		// do next — so do that rather than naming a palette command the reader
		// would then have to go and find.
		//
		// The notice is not visible behind the dialog, which covers the screen.
		// It is there for the reader who escapes out, which is exactly when an
		// explanation of what just appeared is wanted.
		//
		// Kept short on purpose: the status bar drops whole fields to fit, and
		// a longer sentence here is one an 80-column terminal shows none of.
		a.notice("no worktree attached — choose a directory")
		a.showAttachDirectory()
		return
	}
	a.rescan()

	title := fmt.Sprintf(" files · %s ", a.worktreeLabel())
	if a.wtSnap.Dirty {
		title = fmt.Sprintf(" files · %s * ", a.worktreeLabel())
	}

	box := a.newSearchBox("file: ", title, pageFiles, func(term string) []searchItem {
		return a.fileChoices(term)
	})

	a.pages.AddPage(pageFiles, centred(box, 84, 24), true, true)
}

// fileChoices filters the snapshot in memory.
func (a *App) fileChoices(term string) []searchItem {
	term = strings.TrimSpace(term)

	rows := make([]ranked, 0, len(a.wtSnap.Files))
	for _, f := range a.wtSnap.Files {
		file := f
		score, ok := match.Fuzzy(term, file.Rel)
		if !ok {
			continue
		}
		rows = append(rows, ranked{
			item: searchItem{
				// The marker leads so the changed files line up down one column.
				primary:   file.Status.Marker() + " " + file.Rel,
				secondary: file.Status.Describe(),
				accept: func() {
					a.closeSearchBox(pageFiles)
					a.openWorktreeFile(file)
				},
			},
			score: score,
		})
	}
	items := sortRanked(rows)

	if len(items) == 0 {
		if len(a.wtSnap.Files) == 0 {
			return []searchItem{message("no SQL files here",
				"nothing matching *.sql under "+a.wt.Root)}
		}
		if term == "" {
			return []searchItem{nothingHere("no SQL files here yet",
				"save one to the attached directory and it will show up")}
		}
		return []searchItem{noMatch("file", term)}
	}

	if a.wtSnap.Truncated {
		items = append(items, truncatedNotice("too many files to list them all"))
	}
	return items
}

// openWorktreeFile loads a file into the editor.
//
// Unsaved work is confirmed away rather than replaced: the buffer is the only
// copy of whatever has been typed into it, and there is no undo across a
// wholesale SetText.
func (a *App) openWorktreeFile(f worktree.File) {
	if a.fileDirty() {
		a.confirmDiscard(
			fmt.Sprintf("%s has unsaved changes.\n\nOpen %s and lose them?", a.openFile.rel, f.Rel),
			"Open",
			func() { a.loadWorktreeFile(f) })
		return
	}
	a.loadWorktreeFile(f)
}

func (a *App) loadWorktreeFile(f worktree.File) {
	text, stamp, err := a.wt.Read(f.Rel)
	if err != nil {
		a.notice(fmt.Sprintf("cannot open %s: %v", f.Rel, err))
		return
	}

	a.editor.SetText(text, false)
	a.openFile = openFile{rel: f.Rel, loaded: text, stamp: stamp}
	a.app.SetFocus(a.editor)

	a.notice(fmt.Sprintf("%s — %s runs the statement under the cursor, %s saves",
		f.Rel, a.keyLabel(keymap.ActionRun), a.keyLabel(keymap.ActionSaveFile)))
}
