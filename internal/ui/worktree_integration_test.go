//go:build integration

package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/worktree"
	"github.com/gdamore/tcell/v2"
)

// gitInRepo runs a git command, failing the test with its output.
func gitInRepo(t *testing.T, dir string, args ...string) {
	t.Helper()

	full := append([]string{
		"-C", dir,
		"-c", "user.email=test@example.invalid",
		"-c", "user.name=test",
		"-c", "commit.gpgsign=false",
	}, args...)

	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

const initialSQL = "SELECT 1;\n"

// newWorktree builds a repository with one committed-and-edited migration and
// one that was never added, which is what the marker column is there to tell
// apart.
func newWorktree(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	gitInRepo(t, root, "init", "-b", "feature/add-index")

	writeFile(t, filepath.Join(root, "migrations", "001_init.sql"), initialSQL)
	gitInRepo(t, root, "add", ".")
	gitInRepo(t, root, "commit", "-m", "initial")

	writeFile(t, filepath.Join(root, "migrations", "001_init.sql"), initialSQL+"-- edited\n")
	writeFile(t, filepath.Join(root, "scratch.sql"), "SELECT 2;\n")

	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// attachWorktree points the running interface at a directory, through the same
// call the attach dialog makes.
func (h *harness) attachWorktree(dir string) {
	h.t.Helper()

	h.app.app.QueueUpdateDraw(func() { h.app.attach(dir) })
	h.settle()
	h.waitFor("the worktree to be listed", func(a *App) bool {
		return a.wt != nil && len(a.wtSnap.Files) > 0
	})
}

// frontPage reports which page is on top, which is how a dialog's presence is
// asserted without reading the screen.
func (h *harness) frontPage() string {
	h.t.Helper()

	var name string
	h.inspect(func(a *App) bool {
		name, _ = a.pages.GetFrontPage()
		return true
	})
	return name
}

// openMigration loads the edited migration through the finder, which is the
// route a user takes to it.
func (h *harness) openMigration(t *testing.T) {
	t.Helper()

	h.do(keymap.ActionFindFile)
	h.waitFor("the file finder", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageFiles
	})

	h.typeInto("001_init")
	h.press(tcell.KeyEnter)
	h.waitFor("the file to load", func(a *App) bool {
		return a.openFile.rel == "migrations/001_init.sql"
	})
}

// The marker column is the whole reason git is consulted: it says which of
// these files is the one being worked on right now.
func TestTheFinderListsTheWorktreeWithGitsMarkers(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.attachWorktree(newWorktree(t))

	h.do(keymap.ActionFindFile)

	if !h.waitForScreen("migrations/001_init.sql") {
		t.Fatalf("the finder does not list the migration; screen:\n%s", h.text())
	}
	screen := h.text()
	if !strings.Contains(screen, "M migrations/001_init.sql") {
		t.Errorf("the edited migration carries no modified marker:\n%s", screen)
	}
	if !strings.Contains(screen, "? scratch.sql") {
		t.Errorf("the untracked file carries no untracked marker:\n%s", screen)
	}
	// The branch is what says which piece of work these files belong to.
	if !strings.Contains(screen, "feature/add-index") {
		t.Errorf("the finder does not name the branch:\n%s", screen)
	}
}

// Opening a file is the point of the feature; the editor has to end up
// holding exactly what is on disk.
func TestChoosingAFileLoadsItIntoTheEditor(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root := newWorktree(t)
	h.attachWorktree(root)

	h.openMigration(t)

	want := readFile(t, filepath.Join(root, "migrations", "001_init.sql"))
	if got := h.editorText(); got != want {
		t.Errorf("editor holds %q, want %q", got, want)
	}
	// A file just loaded is not an edited one.
	h.waitFor("the buffer to be clean", func(a *App) bool { return !a.fileDirty() })
}

// Saving is the other half. Without it the file list is a viewer.
func TestSavingWritesTheEditorBackToTheFile(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root := newWorktree(t)
	path := filepath.Join(root, "migrations", "001_init.sql")

	h.attachWorktree(root)
	h.openMigration(t)

	h.typeSQL("ALTER TABLE users ADD INDEX idx_email (email);\n")
	h.waitFor("the buffer to be marked unsaved", func(a *App) bool { return a.fileDirty() })

	h.do(keymap.ActionSaveFile)
	h.waitFor("the unsaved marker to clear", func(a *App) bool { return !a.fileDirty() })

	if got := readFile(t, path); got != "ALTER TABLE users ADD INDEX idx_email (email);\n" {
		t.Errorf("the file on disk holds %q", got)
	}
}

// A file that changed underneath the session belongs to someone else — a
// rebase, another editor. Overwriting it silently is the one way this feature
// could destroy work.
func TestAFileChangedOnDiskIsNotOverwrittenWithoutAsking(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root := newWorktree(t)
	path := filepath.Join(root, "migrations", "001_init.sql")

	h.attachWorktree(root)
	h.openMigration(t)

	const elsewhere = "-- written by something else\n"
	writeFile(t, path, elsewhere)

	h.typeSQL("SELECT 'mine';\n")
	h.do(keymap.ActionSaveFile)

	h.waitFor("the overwrite confirmation", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageConfirm
	})

	// Enter takes the focused button, which is Cancel.
	h.press(tcell.KeyEnter)
	h.waitFor("the dialog to close", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageMain
	})

	if got := readFile(t, path); got != elsewhere {
		t.Errorf("the file was overwritten despite cancelling: %q", got)
	}
}

// Confirming has to actually go through, or the dialog is a wall rather than
// a question.
func TestConfirmingTheOverwriteSaves(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root := newWorktree(t)
	path := filepath.Join(root, "migrations", "001_init.sql")

	h.attachWorktree(root)
	h.openMigration(t)

	writeFile(t, path, "-- written by something else\n")

	h.typeSQL("SELECT 'mine';\n")
	h.do(keymap.ActionSaveFile)
	h.waitFor("the overwrite confirmation", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageConfirm
	})

	// Tab moves off Cancel and onto Overwrite.
	h.press(tcell.KeyTab)
	h.press(tcell.KeyEnter)

	h.waitFor("the save to land", func(a *App) bool { return !a.fileDirty() })
	if got := readFile(t, path); got != "SELECT 'mine';\n" {
		t.Errorf("confirming did not save; the file holds %q", got)
	}
}

// The buffer is the only copy of unsaved work, and quitting is one keystroke.
func TestQuittingWithUnsavedChangesAsksFirst(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.attachWorktree(newWorktree(t))
	h.openMigration(t)

	h.typeSQL("SELECT 'unsaved';\n")
	h.waitFor("the buffer to be marked unsaved", func(a *App) bool { return a.fileDirty() })

	h.do(keymap.ActionQuit)
	h.waitFor("the quit confirmation", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageConfirm
	})

	// The interface is still running, which is the whole point.
	h.press(tcell.KeyEnter)
	if got := h.frontPage(); got != pageMain {
		t.Errorf("front page is %q after cancelling the quit", got)
	}
}

// A scratch buffer has never been anywhere else, and prompting about every
// session's leftovers is how a prompt stops being read.
func TestQuittingWithNoFileOpenDoesNotAsk(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1")

	if h.inspect(func(a *App) bool { return a.fileDirty() }) {
		t.Error("a buffer with no file behind it is reported as unsaved")
	}
}

// Pressing the finder key before attaching anything used to leave a line on
// the status bar naming a palette command — which is no help to anyone who
// has not found the palette. The one useful thing to do next is attach, so
// that is what opens.
func TestTheFinderOpensTheAttachDialogWhenNothingIsAttached(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.do(keymap.ActionFindFile)

	h.waitFor("the attach dialog", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageAttach
	})
	if !h.waitForScreen("attach directory") {
		t.Errorf("the dialog does not say what it is; screen:\n%s", h.text())
	}

	// Escaping out reveals the status bar, which is where the explanation of
	// what just happened lands — a dialog covers the whole screen while open.
	h.press(tcell.KeyEscape)
	if !h.waitForScreen("no worktree attached") {
		t.Errorf("nothing explains why the attach dialog appeared; screen:\n%s", h.text())
	}
}

// Attaching through the dialog is the route someone takes who never opened
// the palette, so it has to work end to end.
func TestTheAttachDialogAttachesTheTypedDirectory(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	root := newWorktree(t)

	h.do(keymap.ActionFindFile)
	h.waitFor("the attach dialog", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageAttach
	})

	h.typeInto(root)
	h.press(tcell.KeyEnter)

	// The root is compared resolved: Open follows symlinks on purpose, so that
	// every containment check compares like with like, and a temporary
	// directory on macOS is reached through one.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	h.waitFor("the worktree to be attached", func(a *App) bool {
		return a.wt != nil && a.wt.Root == resolved
	})
}

// The key reference is where anyone will look, and attaching is the entry
// point to the whole feature.
//
// The rendered text is asserted rather than the screen: the reference is
// longer than a terminal and scrolls, so where a line lands is tview's
// business — that it is in there at all is this package's.
func TestTheKeyReferenceIncludesThePaletteCommands(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	for _, want := range []string{"attach directory", "export csv", "keymap vim"} {
		if !h.inspect(func(a *App) bool { return strings.Contains(a.helpText(), want) }) {
			t.Errorf("the key reference does not mention %q", want)
		}
	}
}

// Detaching leaves the text alone: what is on screen is the user's work
// whether or not a file is still behind it.
func TestDetachingKeepsTheBufferButForgetsTheFile(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.attachWorktree(newWorktree(t))
	h.openMigration(t)

	text := h.editorText()
	h.app.app.QueueUpdateDraw(func() { h.app.detachWorktree() })
	h.settle()

	if got := h.editorText(); got != text {
		t.Errorf("detaching changed the buffer: %q", got)
	}
	h.waitFor("the file to be forgotten", func(a *App) bool {
		return a.wt == nil && !a.openFile.isOpen()
	})
}

// The guard does not care where a statement came from. A production DELETE
// loaded from a file is the same statement it would be if it had been typed.
func TestAStatementLoadedFromAFileStillMeetsTheGuard(t *testing.T) {
	h := newHarness(t, config.EnvProd)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "danger.sql"), "DELETE FROM users;\n")

	wt, err := worktree.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	h.app.app.QueueUpdateDraw(func() {
		h.app.wt = wt
		h.app.loadWorktreeFile(worktree.File{Rel: "danger.sql", Abs: filepath.Join(root, "danger.sql")})
	})
	h.settle()

	h.do(keymap.ActionRun)
	if !h.waitForScreen("Refused") {
		t.Fatalf("a production DELETE from a file was not refused; screen:\n%s", h.text())
	}
}
