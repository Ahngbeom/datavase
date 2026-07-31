//go:build integration

package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/gitlab"
	"github.com/Ahngbeom/datavase/internal/glab"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// fakeGitLab answers from memory.
//
// The suite must never need a GitLab, or a token, or a network: those tests
// fail on an aeroplane and pass for the wrong reasons everywhere else. The
// client's own behaviour is pinned against an httptest server in its package.
type fakeGitLab struct {
	project  gitlab.Project
	requests []gitlab.MergeRequest
	files    map[int][]string
	contents map[string]string
	snippets []gitlab.Snippet

	projectErr  error
	snippetsErr error
}

func (f *fakeGitLab) Project(context.Context, string) (gitlab.Project, error) {
	return f.project, f.projectErr
}

func (f *fakeGitLab) MergeRequests(context.Context, int) ([]gitlab.MergeRequest, error) {
	return f.requests, nil
}

func (f *fakeGitLab) SQLFiles(_ context.Context, _, iid int) ([]string, error) {
	return f.files[iid], nil
}

func (f *fakeGitLab) File(_ context.Context, _ int, ref, path string) (string, error) {
	text, ok := f.contents[ref+":"+path]
	if !ok {
		return "", errors.New("no such file")
	}
	return text, nil
}

func (f *fakeGitLab) Snippets(context.Context, int) ([]gitlab.Snippet, error) {
	return f.snippets, f.snippetsErr
}

func (f *fakeGitLab) SnippetContent(_ context.Context, s gitlab.Snippet) (string, error) {
	text, ok := f.contents[fmt.Sprintf("snippet:%d", s.ID)]
	if !ok {
		return "", errors.New("no such snippet")
	}
	return text, nil
}

const migrationSQL = "ALTER TABLE users ADD INDEX idx_email (email);\n"

func stubGitLab() *fakeGitLab {
	return &fakeGitLab{
		project:  gitlab.Project{ID: 7, Path: "group/project"},
		requests: []gitlab.MergeRequest{{IID: 42, Title: "Add the email index", SHA: "abc123", SourceBranch: "add-index"}},
		files:    map[int][]string{42: {"db/003_index.sql"}},
		contents: map[string]string{
			"abc123:db/003_index.sql": migrationSQL,
			"snippet:5":               "SELECT 1;\n",
		},
		snippets: []gitlab.Snippet{{ID: 5, Title: "row counts", FileName: "counts.sql"}},
	}
}

// useGitLab points the running interface at a fake, naming the project in
// configuration rather than through a checkout.
func (h *harness) useGitLab(source GitLabSource) {
	h.t.Helper()

	h.inspect(func(a *App) bool {
		a.openGitLab = opening(source, nil)
		a.gitlabHost = "gitlab.example.com"
		a.cfg.GitLab = config.GitLab{Host: "gitlab.example.com", Project: "group/project"}
		return true
	})
}

// opening builds the factory the interface calls when the GitLab command is
// used, recording the host it was asked for.
func opening(source GitLabSource, host *string) func(context.Context, string) (GitLabSource, error) {
	return func(_ context.Context, asked string) (GitLabSource, error) {
		if host != nil {
			*host = asked
		}
		return source, nil
	}
}

// failing builds a factory that cannot produce a reader, which is what a
// missing glab or a missing login looks like from here.
func failing(err error) func(context.Context, string) (GitLabSource, error) {
	return func(context.Context, string) (GitLabSource, error) { return nil, err }
}

func (h *harness) openFromGitLab() {
	h.t.Helper()

	h.inspect(func(a *App) bool { a.showGitLab(); return true })
	h.waitFor("the gitlab dialog", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageGitLab
	})
}

// With no checkout attached and nothing configured there is no project to ask
// about, and the command has to say so rather than fail silently.
func TestOpeningFromGitLabWithNoProjectSaysSo(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.inspect(func(a *App) bool {
		a.openGitLab = opening(stubGitLab(), nil)
		a.gitlabHost = "gitlab.example.com"
		return true
	})

	h.inspect(func(a *App) bool { a.showGitLab(); return true })

	if !h.waitForScreen("no GitLab project") {
		t.Errorf("nothing said the project was unknown; screen:\n%s", h.text())
	}
}

// The two ways of having no credential need different sentences. Told only
// "no token", someone goes off to create a personal access token — which is
// precisely the second credential borrowing from glab exists to avoid.
func TestTheMissingPieceIsNamedRatherThanTheFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		// Each wanted string is one the underlying error does not itself
		// contain, so that falling through to a bare "GitLab: <err>" fails
		// here rather than passing on the sentinel's own wording.
		{"glab absent", glab.ErrNotInstalled, "datavase reads GitLab through it"},
		{"glab logged out", glab.ErrNoToken, "glab auth login --hostname gitlab.example.com"},
		{"token rejected", gitlab.ErrUnauthorized, "token rejected — run: glab auth login"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, config.EnvDev)
			h.inspect(func(a *App) bool {
				a.openGitLab = failing(tc.err)
				a.gitlabHost = "gitlab.example.com"
				a.cfg.GitLab = config.GitLab{Host: "gitlab.example.com", Project: "group/project"}
				return true
			})

			h.inspect(func(a *App) bool { a.showGitLab(); return true })

			if !h.waitForScreen(tc.want) {
				t.Errorf("the advice does not mention %q; screen:\n%s", tc.want, h.text())
			}
		})
	}
}

func TestTheOpenMergeRequestsAndSnippetsAreListed(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.useGitLab(stubGitLab())

	h.openFromGitLab()

	screen := h.text()
	if !strings.Contains(screen, "Add the email index") {
		t.Errorf("the merge request is not listed:\n%s", screen)
	}
	if !strings.Contains(screen, "row counts") {
		t.Errorf("the snippet is not listed:\n%s", screen)
	}
}

// An instance with snippets switched off, or a token without the scope, must
// still list the merge requests — which is what people came for.
func TestSnippetsFailingDoesNotHideTheMergeRequests(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	source := stubGitLab()
	source.snippetsErr = errors.New("403 Forbidden")
	h.useGitLab(source)

	h.openFromGitLab()

	if !strings.Contains(h.text(), "Add the email index") {
		t.Errorf("a snippets failure took the merge requests with it:\n%s", h.text())
	}
}

// The point of the whole feature: the migration in an unmerged branch ends up
// in the editor without the branch being checked out.
func TestChoosingAMergeRequestLoadsItsMigration(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.useGitLab(stubGitLab())

	h.openFromGitLab()
	h.typeInto("email index")
	h.press(tcell.KeyEnter)

	h.waitFor("the migration to load", func(a *App) bool {
		return a.editor.GetText() == migrationSQL
	})
	h.waitFor("the origin to be recorded", func(a *App) bool {
		return a.openFile.rel == "db/003_index.sql" && a.openFile.origin == "group/project!42"
	})
}

// There is no branch here to commit to. Refusing has to name the reason, or
// the reader goes off to attach a worktree that would not help.
func TestSavingAFileFromGitLabIsRefusedWithItsReason(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.useGitLab(stubGitLab())

	h.openFromGitLab()
	h.typeInto("email index")
	h.press(tcell.KeyEnter)
	h.waitFor("the migration to load", func(a *App) bool {
		return a.openFile.origin != ""
	})

	h.do(keymap.ActionSaveFile)

	if !h.waitForScreen("nowhere here to save it") {
		t.Errorf("saving did not explain itself; screen:\n%s", h.text())
	}
	// And the buffer is untouched.
	if got := h.editorText(); got != migrationSQL {
		t.Errorf("the refused save changed the buffer: %q", got)
	}
}

func TestChoosingASnippetLoadsIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.useGitLab(stubGitLab())

	h.openFromGitLab()
	h.typeInto("row counts")
	h.press(tcell.KeyEnter)

	h.waitFor("the snippet to load", func(a *App) bool {
		return a.editor.GetText() == "SELECT 1;\n"
	})
}

// A statement read out of a merge request is still a statement. Where it came
// from buys it nothing with the guard.
func TestAMigrationFromGitLabStillMeetsTheGuard(t *testing.T) {
	h := newHarness(t, config.EnvProd)
	source := stubGitLab()
	source.contents["abc123:db/003_index.sql"] = "DELETE FROM users;\n"
	h.useGitLab(source)

	h.openFromGitLab()
	h.typeInto("email index")
	h.press(tcell.KeyEnter)
	h.waitFor("the file to load", func(a *App) bool { return a.openFile.origin != "" })

	h.do(keymap.ActionRun)
	if !h.waitForScreen("Refused") {
		t.Fatalf("a production DELETE from GitLab was not refused; screen:\n%s", h.text())
	}
}

// The project is nearly always the one the attached checkout came from, and
// making that be configured as well is asking for something already on disk.
func TestTheProjectIsTakenFromTheAttachedCheckoutsOrigin(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	root := newWorktree(t)
	gitInRepo(t, root, "remote", "add", "origin", "git@gitlab.example.com:group/project.git")
	h.attachWorktree(root)

	h.inspect(func(a *App) bool {
		a.openGitLab = opening(stubGitLab(), nil)
		a.gitlabHost = "gitlab.example.com"
		return true
	})
	h.waitFor("the remote to be read", func(a *App) bool { return a.wtSnap.Remote != "" })

	h.openFromGitLab()
	if !strings.Contains(h.text(), "group/project") {
		t.Errorf("the dialog does not name the project from the origin:\n%s", h.text())
	}
}

// With no host configured, a checkout is all the configuration there is — the
// instance is whichever one it was cloned from.
func TestWithNoHostConfiguredTheCheckoutDecidesTheInstance(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	root := newWorktree(t)
	gitInRepo(t, root, "remote", "add", "origin", "git@gitlab.internal.example:team/db.git")
	h.attachWorktree(root)

	var asked string
	h.inspect(func(a *App) bool {
		a.openGitLab = opening(stubGitLab(), &asked)
		a.gitlabHost = "" // nothing configured
		return true
	})
	h.waitFor("the remote to be read", func(a *App) bool { return a.wtSnap.Remote != "" })

	h.openFromGitLab()
	if asked != "gitlab.internal.example" {
		t.Errorf("asked glab for %q, want the host the checkout came from", asked)
	}
}

// A checkout of something else entirely must not be mistaken for the
// configured instance's project.
func TestAnOriginOnAnotherHostIsNotUsed(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	root := newWorktree(t)
	gitInRepo(t, root, "remote", "add", "origin", "git@github.com:someone/else.git")
	h.attachWorktree(root)

	h.inspect(func(a *App) bool {
		a.openGitLab = opening(stubGitLab(), nil)
		a.gitlabHost = "gitlab.example.com"
		return true
	})
	h.waitFor("the remote to be read", func(a *App) bool { return a.wtSnap.Remote != "" })

	h.inspect(func(a *App) bool { a.showGitLab(); return true })
	if !h.waitForScreen("no GitLab project") {
		t.Errorf("a foreign origin was taken for the configured instance; screen:\n%s", h.text())
	}
}
