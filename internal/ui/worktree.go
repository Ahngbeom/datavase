package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/match"
	"github.com/Ahngbeom/datavase/internal/worktree"
)

// scanTimeout bounds a listing. git is already bounded inside the worktree
// package; this also covers the plain filesystem walk a non-repository gets,
// which is the case that can wander into somewhere enormous.
const scanTimeout = 3 * time.Second

// attach points the session at a directory of SQL work.
//
// Attaching is deliberately explicit — there is no scan of the working
// directory at startup. A client that decides on its own which directory it is
// "in" is one that opens the wrong branch's migrations without saying so.
func (a *App) attach(path string) {
	wt, err := worktree.Open(path)
	if err != nil {
		a.notice(fmt.Sprintf("cannot attach: %v", err))
		return
	}

	a.wt = wt
	a.wtSnap = worktree.Snapshot{}
	a.rescan()
	a.remember(wt.Root)

	a.notice(fmt.Sprintf("attached %s — %s for its files",
		a.worktreeLabel(), a.keyLabel(keymap.ActionFindFile)))
}

// remember records a directory so the next session offers it without the whole
// path being typed again.
//
// Failures are ignored, as they are for the query history: a read-only state
// directory should cost the shortcut, not the attach that just succeeded.
func (a *App) remember(path string) {
	if a.recentDirs == nil {
		return
	}
	a.recentDirs.Add(path)
	_ = a.recentDirs.Save()
}

// detach forgets the worktree. The buffer stays: the text on screen is the
// user's work whether or not it still has a file behind it.
func (a *App) detach() {
	if a.wt == nil {
		a.notice("no worktree is attached")
		return
	}

	name := a.worktreeLabel()
	a.wt = nil
	a.wtSnap = worktree.Snapshot{}
	a.openFile = openFile{}
	a.notice(fmt.Sprintf("detached %s", name))
}

// rescan refreshes the listing and the branch.
//
// A failed scan keeps the previous snapshot rather than emptying the list: an
// empty file list and a directory with no SQL in it look identical, and only
// one of them is a problem the user can act on.
func (a *App) rescan() {
	if a.wt == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()

	snap, err := a.wt.Scan(ctx)
	if err != nil {
		a.notice(fmt.Sprintf("listing %s: %v", a.wt.Name(), err))
		return
	}
	a.wtSnap = snap
}

// worktreeLabel names the attached worktree by its branch, falling back to the
// directory. The branch is what identifies the work; the directory is only
// where it happens to sit.
func (a *App) worktreeLabel() string {
	if a.wt == nil {
		return ""
	}
	if a.wtSnap.Branch != "" {
		return a.wtSnap.Branch
	}
	return a.wt.Name()
}

// showAttachDirectory asks for a directory to attach.
//
// The search box is reused as a path prompt: the list under the field shows
// the directories that could be attached from whatever has been typed, so a
// mistyped path is visible before Enter rather than after it.
func (a *App) showAttachDirectory() {
	box := a.newSearchBox("directory: ", " attach directory ", pageAttach, func(term string) []searchItem {
		return a.directoryChoices(term)
	})

	a.pages.AddPage(pageAttach, centred(box, 78, 20), true, true)
}

// directoryChoices offers the typed path first, then its subdirectories.
//
// Both attach rather than descend. One key doing two different things
// depending on which row it is on is the kind of dialog people learn to
// distrust; typing another path segment is the way down.
func (a *App) directoryChoices(term string) []searchItem {
	typed := strings.TrimSpace(term)

	// Before anything is typed, the directories attached before lead: the one
	// being looked for is nearly always one of them, and the alternative is
	// retyping a path that was already typed in full once.
	var items []searchItem
	if typed == "" {
		items = append(items, a.recentChoices()...)
		if cwd, err := os.Getwd(); err == nil {
			typed = cwd
		}
	}
	// The working directory is often one of the recent ones, and offering it
	// twice makes the list look like it is listing something else.
	listed := make(map[string]bool, len(items))
	for _, it := range items {
		listed[it.primary] = true
	}

	if dir, ok := existingDir(typed); ok && !listed[dir] {
		row := searchItem{
			primary:   dir,
			secondary: "attach this directory · " + directoryKind(dir),
			accept: func() {
				a.closeSearchBox(pageAttach)
				a.attach(dir)
			},
		}
		// Only when it would change something: Tab that leaves the field
		// exactly as it was reads as a key that has stopped working. It earns
		// its place expanding a leading ~.
		if dir != typed {
			row.complete = dir
		}
		items = append(items, row)
	}

	subs, cut := subdirectories(typed)
	for _, sub := range subs {
		if listed[sub] {
			continue
		}
		path := sub
		items = append(items, searchItem{
			primary:   path,
			secondary: directoryKind(path),
			complete:  path,
			accept: func() {
				a.closeSearchBox(pageAttach)
				a.attach(path)
			},
		})
	}

	if len(items) == 0 {
		return []searchItem{nothingHere("no such directory", "type an absolute path, or ~/…")}
	}
	if cut {
		items = append(items, truncatedNotice("too many directories — type more of the path"))
	}
	return items
}

// recentChoices offers the directories attached in earlier sessions.
//
// One that has since been deleted or renamed is dropped rather than offered
// and refused: a list of paths that no longer work is worse than no list.
func (a *App) recentChoices() []searchItem {
	if a.recentDirs == nil {
		return nil
	}

	var items []searchItem
	for _, entry := range a.recentDirs.Entries() {
		dir, ok := existingDir(entry)
		if !ok {
			continue
		}
		path := dir
		items = append(items, searchItem{
			primary:   path,
			secondary: "recent · " + directoryKind(path),
			complete:  path,
			accept: func() {
				a.closeSearchBox(pageAttach)
				a.attach(path)
			},
		})
	}
	return items
}

// directoryKind says what a candidate is before it is attached.
//
// A stat, never an exec: git answers in milliseconds, and forty rows rebuilt
// on every keystroke cannot afford forty of those. Which branch it is on has
// to wait until something is attached.
//
// Stat rather than IsDir because a linked worktree's .git is a file.
func directoryKind(path string) string {
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return "git repository"
	}
	return "plain directory"
}

// existingDir resolves a typed path and reports whether it is a directory.
func existingDir(path string) (string, bool) {
	resolved, err := expandUser(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return resolved, true
}

// subdirectories lists the children of the typed path, or the siblings
// matching it when the path is half-typed, and reports whether the list was
// cut short.
//
// The half-typed case is what makes the prompt usable: "~/work/data" should
// offer "~/work/datavase" rather than nothing at all.
func subdirectories(path string) (dirs []string, truncated bool) {
	resolved, err := expandUser(path)
	if err != nil {
		return nil, false
	}

	dir, prefix := resolved, ""
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		dir, prefix = filepath.Split(resolved)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}

	// A dot-directory is hidden until it is asked for by name, which is the
	// only way ~/.config is reachable without spelling the whole path: showing
	// every one of them by default buries the checkouts people are looking for.
	wantHidden := strings.HasPrefix(prefix, ".")

	// What was typed still wins as a prefix. Loose matching is there so that
	// "vase" can find "datavase", but it must never decide what Tab completes
	// to — a completion that skips over the thing you spelled out is one you
	// stop trusting.
	type candidate struct {
		path  string
		exact bool
		score int
	}

	var found []candidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && !wantHidden {
			continue
		}

		switch {
		case strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)):
			found = append(found, candidate{path: filepath.Join(dir, name), exact: true})
		default:
			score, ok := match.Fuzzy(prefix, name)
			if !ok {
				continue
			}
			found = append(found, candidate{path: filepath.Join(dir, name), score: score})
		}
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].exact != found[j].exact {
			return found[i].exact
		}
		if found[i].exact {
			return found[i].path < found[j].path
		}
		return found[i].score > found[j].score
	})

	if len(found) > maxDirectoryChoices {
		found, truncated = found[:maxDirectoryChoices], true
	}

	out := make([]string, len(found))
	for i, c := range found {
		out[i] = c.path
	}
	return out, truncated
}

// maxDirectoryChoices keeps a home directory full of checkouts from filling
// the dialog with rows nobody will scroll through.
const maxDirectoryChoices = 40

// expandUser resolves a leading ~ so a typed path behaves like a shell one.
func expandUser(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
