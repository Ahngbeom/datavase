package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/gitlab"
	"github.com/Ahngbeom/datavase/internal/glab"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/match"
)

// Reading SQL out of GitLab.
//
// A merge request is a second place SQL lives, alongside the attached
// worktree, and the interesting one is usually the migration that has not been
// merged yet. What comes back is read-only: there is no branch here to commit
// to, and a client that could push would be a worse git than git.
//
// Every call is made off the interface's goroutine and every failure is a
// notice. This is a convenience bolted onto a database client, and a slow
// proxy must not be able to freeze the session someone came here to use.

// gitlabTimeout bounds a whole listing, which may be several requests.
const gitlabTimeout = 20 * time.Second

// GitLabSource is the part of the GitLab client the interface uses.
//
// An interface rather than the concrete client so that the tests can answer
// from memory: the unit and integration suites must not need a GitLab, and a
// test that reaches one is a test that fails on an aeroplane.
type GitLabSource interface {
	Project(ctx context.Context, path string) (gitlab.Project, error)
	MergeRequests(ctx context.Context, projectID int) ([]gitlab.MergeRequest, error)
	SQLFiles(ctx context.Context, projectID, iid int) ([]string, error)
	File(ctx context.Context, projectID int, ref, path string) (string, error)
	Snippets(ctx context.Context, projectID int) ([]gitlab.Snippet, error)
	SnippetContent(ctx context.Context, s gitlab.Snippet) (string, error)
}

// gitlabFile is one openable file, carrying everything needed to fetch it.
//
// Self-contained on purpose: the fetch happens on a background goroutine, and
// anything it had to read back off App would be a data race with the drawing
// one.
type gitlabFile struct {
	// path is what to show and, for a repository file, where to read it.
	path      string
	ref       string
	projectID int
	// snippet is set instead when the file is a snippet, which is fetched
	// whole rather than by path.
	snippet *gitlab.Snippet
	// origin says where this came from, for the refusal to save it.
	origin string
	// source is the reader that fetched this file's listing, carried along so
	// that opening it needs nothing from App.
	source GitLabSource
}

// showGitLab lists what can be opened: the open merge requests, then the
// snippets.
func (a *App) showGitLab() {
	if a.openGitLab == nil {
		a.notice("GitLab reading is not wired up in this session")
		return
	}

	host, project, ok := a.gitlabProject()
	if !ok {
		a.notice("no GitLab project — attach a checkout of one, or set gitlab.project")
		return
	}

	a.notice(fmt.Sprintf("asking GitLab about %s…", project))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitlabTimeout)
		defer cancel()

		// Resolved here rather than at startup: borrowing the credential costs
		// half a second of keyring access, and a session that never opens this
		// dialog should not pay it. Off the interface's goroutine, so the wait
		// is not a frozen screen.
		source, err := a.openGitLab(ctx, host)
		if err != nil {
			a.app.QueueUpdateDraw(func() { a.notice(gitlabAdvice(host, err)) })
			return
		}

		sources, err := fetchGitLabSources(ctx, source, project)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(gitlabAdvice(host, err))
				return
			}
			if len(sources) == 0 {
				a.notice(fmt.Sprintf("%s has no open merge requests or snippets", project))
				return
			}
			a.showGitLabSources(host, project, sources)
		})
	}()
}

// gitlabAdvice turns a failure to reach GitLab into the command that fixes it.
//
// The two failures need different sentences, and neither is helped by the
// underlying error: someone told only "no token" goes looking for a personal
// access token to create, which is exactly the second credential this avoids.
func gitlabAdvice(host string, err error) string {
	switch {
	case errors.Is(err, glab.ErrNotInstalled):
		return "glab is not installed — datavase reads GitLab through it"

	// Both of these are fixed by the same command. A token glab holds but
	// GitLab rejects has expired, and renewing it is glab's job, not a
	// personal access token to go and mint.
	case errors.Is(err, glab.ErrNoToken):
		// The host appears once and the command comes early: the status bar
		// truncates from the right, and the half worth keeping is the one that
		// can be typed.
		return fmt.Sprintf("not logged in — run: glab auth login --hostname %s", host)
	case errors.Is(err, gitlab.ErrUnauthorized):
		return fmt.Sprintf("token rejected — run: glab auth login --hostname %s", host)

	default:
		return fmt.Sprintf("GitLab: %v", err)
	}
}

// gitlabSourceRow is one merge request or snippet in the first dialog.
type gitlabSourceRow struct {
	title  string
	detail string
	// files is what choosing this row offers. Snippets are one file each and
	// are known already; a merge request's files need another request, so its
	// list arrives empty and fetch fills it.
	files []gitlabFile
	fetch func(ctx context.Context) ([]gitlabFile, error)
}

// fetchGitLabSources gathers the merge requests and snippets.
//
// Snippets failing does not sink the merge requests: an instance where the
// snippets feature is off, or a token without the scope for it, should still
// list the thing most people came for.
//
// A free function taking its reader as an argument, not a method: it runs on a
// background goroutine, and anything it reached for on App would be a race
// with the one drawing.
func fetchGitLabSources(ctx context.Context, source GitLabSource, path string) ([]gitlabSourceRow, error) {
	project, err := source.Project(ctx, path)
	if err != nil {
		return nil, err
	}

	requests, err := source.MergeRequests(ctx, project.ID)
	if err != nil {
		return nil, err
	}

	var rows []gitlabSourceRow
	for _, r := range requests {
		mr := r
		rows = append(rows, gitlabSourceRow{
			title:  fmt.Sprintf("!%d %s", mr.IID, mr.Title),
			detail: "merge request · " + mr.SourceBranch,
			fetch: func(ctx context.Context) ([]gitlabFile, error) {
				paths, err := source.SQLFiles(ctx, project.ID, mr.IID)
				if err != nil {
					return nil, err
				}
				files := make([]gitlabFile, 0, len(paths))
				for _, p := range paths {
					files = append(files, gitlabFile{
						path:      p,
						ref:       mr.SHA,
						projectID: project.ID,
						origin:    fmt.Sprintf("%s!%d", project.Path, mr.IID),
						source:    source,
					})
				}
				return files, nil
			},
		})
	}

	if snippets, err := source.Snippets(ctx, project.ID); err == nil {
		for _, s := range snippets {
			snippet := s
			name := snippet.FileName
			if name == "" {
				name = snippet.Title
			}
			rows = append(rows, gitlabSourceRow{
				title:  snippet.Title,
				detail: "snippet · " + name,
				files: []gitlabFile{{
					path:    name,
					snippet: &snippet,
					origin:  "a GitLab snippet",
					source:  source,
				}},
			})
		}
	}
	return rows, nil
}

// showGitLabSources offers the merge requests and snippets.
//
// The host travels with them so that a failure on the second request — the one
// that lists a merge request's files — gives the same advice as a failure on
// the first.
func (a *App) showGitLabSources(host, project string, rows []gitlabSourceRow) {
	box := a.newSearchBox("gitlab: ", " "+project+" ", pageGitLab, func(term string) []searchItem {
		scored := make([]ranked, 0, len(rows))
		for _, r := range rows {
			row := r
			score, ok := match.Fuzzy(term, row.title+" "+row.detail)
			if !ok {
				continue
			}
			scored = append(scored, ranked{
				item: searchItem{
					primary:   row.title,
					secondary: row.detail,
					accept: func() {
						a.closeSearchBox(pageGitLab)
						a.openGitLabSource(host, row)
					},
				},
				score: score,
			})
		}
		if len(scored) == 0 {
			return []searchItem{message("nothing matching", "press Escape to close")}
		}
		return sortRanked(scored)
	})

	a.pages.AddPage(pageGitLab, centred(box, 84, 24), true, true)
}

// openGitLabSource opens a row's only file, or offers the choice.
func (a *App) openGitLabSource(host string, row gitlabSourceRow) {
	if row.fetch == nil {
		a.chooseGitLabFile(row.title, row.files)
		return
	}

	a.notice(fmt.Sprintf("looking in %s…", row.title))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitlabTimeout)
		defer cancel()

		files, err := row.fetch(ctx)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(gitlabAdvice(host, err))
				return
			}
			if len(files) == 0 {
				a.notice(fmt.Sprintf("%s touches no .sql files", row.title))
				return
			}
			a.chooseGitLabFile(row.title, files)
		})
	}()
}

// chooseGitLabFile offers the files, skipping the dialog when there is one.
func (a *App) chooseGitLabFile(title string, files []gitlabFile) {
	if len(files) == 1 {
		a.openGitLabFile(files[0])
		return
	}

	box := a.newSearchBox("file: ", " "+title+" ", pageGitLabFiles, func(term string) []searchItem {
		scored := make([]ranked, 0, len(files))
		for _, f := range files {
			file := f
			score, ok := match.Fuzzy(term, file.path)
			if !ok {
				continue
			}
			scored = append(scored, ranked{
				item: searchItem{
					primary: file.path,
					accept: func() {
						a.closeSearchBox(pageGitLabFiles)
						a.openGitLabFile(file)
					},
				},
				score: score,
			})
		}
		if len(scored) == 0 {
			return []searchItem{message("no matching file", "type part of a path")}
		}
		return sortRanked(scored)
	})

	a.pages.AddPage(pageGitLabFiles, centred(box, 84, 24), true, true)
}

// openGitLabFile loads a file, confirming away unsaved work first.
func (a *App) openGitLabFile(file gitlabFile) {
	if a.fileDirty() {
		a.confirmDiscard(
			fmt.Sprintf("%s has unsaved changes.\n\nOpen %s and lose them?", a.openFile.rel, file.path),
			"Open",
			func() { a.loadGitLabFile(file) })
		return
	}
	a.loadGitLabFile(file)
}

func (a *App) loadGitLabFile(file gitlabFile) {
	a.notice(fmt.Sprintf("fetching %s…", file.path))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitlabTimeout)
		defer cancel()

		text, err := fetchGitLabText(ctx, file)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.notice(fmt.Sprintf("cannot open %s: %v", file.path, err))
				return
			}

			a.editor.SetText(text, false)
			// No stamp: there is nothing on disk for one to describe, and the
			// origin is what makes saving refuse rather than ask.
			a.openFile = openFile{rel: file.path, loaded: text, origin: file.origin}
			a.app.SetFocus(a.editor)

			a.notice(fmt.Sprintf("%s from %s — read-only, %s runs the statement under the cursor",
				file.path, file.origin, a.keyLabel(keymap.ActionRun)))
		})
	}()
}

func fetchGitLabText(ctx context.Context, file gitlabFile) (string, error) {
	if file.snippet != nil {
		return file.source.SnippetContent(ctx, *file.snippet)
	}
	return file.source.File(ctx, file.projectID, file.ref, file.path)
}

// gitlabProject decides which instance and which project to browse.
//
// The attached checkout's origin comes first: someone who has attached a
// worktree is working on that project, and making them configure its name as
// well would be asking for something already on disk.
//
// With no host configured the origin's own host is used, so a checkout is all
// the configuration there is. Naming a host in configuration narrows it back
// down — a checkout of something on another host is then not this instance's
// project, and saying so beats asking GitLab about a repository it has never
// heard of.
func (a *App) gitlabProject() (host, path string, ok bool) {
	if a.wtSnap.Remote != "" {
		if remoteHost, remotePath, parsed := gitlab.ProjectFromRemote(a.wtSnap.Remote); parsed {
			if a.gitlabHost == "" {
				return remoteHost, remotePath, true
			}
			if remoteHost == a.gitlabHost {
				return remoteHost, remotePath, true
			}
		}
	}
	if a.cfg.GitLab.Project != "" {
		return a.cfg.GitLab.Resolved(), a.cfg.GitLab.Project, true
	}
	return "", "", false
}
