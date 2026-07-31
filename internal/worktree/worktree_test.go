package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireGit skips rather than fails: this package is deliberately usable on a
// machine without git, and so are its tests.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// git runs a command in dir, failing the test with the output when it errors.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	// Identity and signing are supplied here so the test does not depend on
	// whatever the developer's global git config happens to say.
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

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo builds a repository with one committed, one modified, one staged-new
// and one untracked SQL file, plus files that must not appear at all.
func newRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)

	root := t.TempDir()
	git(t, root, "init", "-b", "main")

	write(t, filepath.Join(root, "migrations", "001_init.sql"), "SELECT 1;\n")
	write(t, filepath.Join(root, "migrations", "002_users.sql"), "SELECT 2;\n")
	write(t, filepath.Join(root, "notes.txt"), "not sql\n")
	write(t, filepath.Join(root, ".gitignore"), "dumps/\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "initial")

	// Changed since the commit.
	write(t, filepath.Join(root, "migrations", "001_init.sql"), "SELECT 1; -- edited\n")
	// Staged as new.
	write(t, filepath.Join(root, "migrations", "003_index.sql"), "SELECT 3;\n")
	git(t, root, "add", "migrations/003_index.sql")
	// Never added. In a directory that is itself new, which git collapses into
	// a single entry unless asked for every file.
	write(t, filepath.Join(root, "scratch", "probe.sql"), "SELECT 4;\n")
	// Ignored, and therefore none of datavase's business.
	write(t, filepath.Join(root, "dumps", "backup.sql"), "-- huge\n")

	return root
}

func scan(t *testing.T, path string) Snapshot {
	t.Helper()

	wt, err := Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	snap, err := wt.Scan(context.Background())
	if err != nil {
		t.Fatalf("scanning %s: %v", path, err)
	}
	return snap
}

func statuses(snap Snapshot) map[string]Status {
	out := make(map[string]Status, len(snap.Files))
	for _, f := range snap.Files {
		out[f.Rel] = f.Status
	}
	return out
}

// The listing is the whole feature: a file the user is working on that does
// not appear cannot be opened at all.
func TestScanListsEverySQLFileWithWhatGitSaysAboutIt(t *testing.T) {
	root := newRepo(t)
	got := statuses(scan(t, root))

	want := map[string]Status{
		"migrations/001_init.sql":  StatusModified,
		"migrations/002_users.sql": StatusNone,
		"migrations/003_index.sql": StatusAdded,
		// Inside a directory git would otherwise report as one untracked entry.
		"scratch/probe.sql": StatusUntracked,
	}
	for path, status := range want {
		if got[path] != status {
			t.Errorf("%s is %q, want %q", path, got[path], status)
		}
	}

	// An ignored dump is not work in progress, and loading one would be the
	// worst thing the file list could offer.
	if _, ok := got["dumps/backup.sql"]; ok {
		t.Error("an ignored file was listed")
	}
	if _, ok := got["notes.txt"]; ok {
		t.Error("a file that is not SQL was listed")
	}
	if len(got) != len(want) {
		t.Errorf("listing has unexpected entries: %#v", got)
	}
}

// The branch is the only thing on screen saying which piece of work these
// files belong to.
func TestScanNamesTheBranch(t *testing.T) {
	root := newRepo(t)
	git(t, root, "checkout", "-q", "-b", "feature/add-index")

	snap := scan(t, root)
	if snap.Branch != "feature/add-index" {
		t.Errorf("branch is %q, want feature/add-index", snap.Branch)
	}
	if !snap.Dirty {
		t.Error("a tree with uncommitted changes is reported as clean")
	}
}

// A detached HEAD has no branch name, and showing the literal "HEAD" would
// read as a branch someone had created.
func TestADetachedHeadIsReportedAsACommit(t *testing.T) {
	root := newRepo(t)
	git(t, root, "checkout", "-q", "--detach")

	snap := scan(t, root)
	if !snap.Detached {
		t.Fatal("a detached HEAD is not reported as detached")
	}
	if snap.Branch == "" || snap.Branch == "HEAD" {
		t.Errorf("detached HEAD reports branch %q, want a commit id", snap.Branch)
	}
}

// Attaching a subdirectory means working on that part of the tree; files
// elsewhere in the repository are noise.
func TestOnlyFilesUnderTheAttachedDirectoryAreListed(t *testing.T) {
	root := newRepo(t)
	got := statuses(scan(t, filepath.Join(root, "migrations")))

	if _, ok := got["probe.sql"]; ok {
		t.Error("a file outside the attached directory was listed")
	}
	// Paths are shown relative to what was attached, not to the repository.
	if got["001_init.sql"] != StatusModified {
		t.Errorf("paths are not relative to the attached directory: %#v", got)
	}
}

// git is optional. A directory that is not a repository — or a machine with
// no git at all — should cost the branch and the markers, not the feature.
func TestADirectoryThatIsNotARepositoryStillListsItsSQL(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "queries", "report.sql"), "SELECT 1;\n")
	write(t, filepath.Join(root, "readme.md"), "no\n")

	snap := scan(t, root)
	if snap.Branch != "" {
		t.Errorf("a non-repository reported branch %q", snap.Branch)
	}
	if len(snap.Files) != 1 || snap.Files[0].Rel != "queries/report.sql" {
		t.Errorf("SQL was not listed without git: %#v", snap.Files)
	}
	if snap.Files[0].Status != StatusNone {
		t.Errorf("a file with no git to describe it is marked %q", snap.Files[0].Status)
	}
}

// The finder hands back whatever path it was given. A crafted or stale one
// must not reach a file the user never attached.
func TestPathsOutsideTheWorktreeAreRefused(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(root, "secret.sql")
	write(t, secret, "SELECT 'private';\n")

	inner := filepath.Join(root, "work")
	write(t, filepath.Join(inner, "own.sql"), "SELECT 1;\n")

	wt, err := Open(inner)
	if err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"../secret.sql", "/etc/hosts", "sub/../../secret.sql"} {
		if _, _, err := wt.Read(rel); !errors.Is(err, ErrOutsideWorktree) {
			t.Errorf("reading %q returned %v, want ErrOutsideWorktree", rel, err)
		}
		if _, err := wt.Write(rel, "overwritten"); !errors.Is(err, ErrOutsideWorktree) {
			t.Errorf("writing %q returned %v, want ErrOutsideWorktree", rel, err)
		}
	}

	// The refusal has to be real, not just a different error message.
	if b, err := os.ReadFile(secret); err != nil || string(b) != "SELECT 'private';\n" {
		t.Errorf("the file outside the worktree was modified: %q %v", b, err)
	}
}

// A mysqldump loaded into the editor freezes the interface. Refusing to open
// it is the only outcome that leaves the user with a working application.
func TestAFileTooLargeToEditIsRefused(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "dump.sql"), strings.Repeat("x", MaxFileSize+1))

	wt, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := wt.Read("dump.sql"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("reading an oversized file returned %v, want ErrTooLarge", err)
	}
}

// Saving must not be able to leave a migration half-written, and must not
// quietly widen its permissions either.
func TestWriteReplacesTheFileWholeAndKeepsItsMode(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "one.sql")
	write(t, path, "SELECT 1;\n")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	wt, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}

	stamp, err := wt.Write("one.sql", "SELECT 2;\n")
	if err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil || string(b) != "SELECT 2;\n" {
		t.Errorf("file holds %q (%v), want the new text", b, err)
	}
	if stamp.Size != int64(len("SELECT 2;\n")) {
		t.Errorf("the returned stamp says %d bytes, want %d", stamp.Size, len("SELECT 2;\n"))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode became %v, want 0600", info.Mode().Perm())
	}

	// The temporary file the replacement went through must not be left behind
	// for the finder to offer.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("writing left extra files: %v", entries)
	}
}

// The stamp is what tells the interface the file changed underneath it, so a
// save cannot silently discard someone else's edit.
func TestTheStampFollowsTheFileOnDisk(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "one.sql"), "SELECT 1;\n")

	wt, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}

	_, first, err := wt.Read("one.sql")
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(root, "one.sql"), "SELECT 1; -- changed elsewhere\n")

	second, err := wt.Stat("one.sql")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("a file rewritten outside datavase produced an identical stamp")
	}
}

// Attaching something that is not a usable directory has to fail at the point
// the user asked for it, not later as an empty file list.
func TestOpenRefusesWhatIsNotADirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "one.sql")
	write(t, file, "SELECT 1;\n")

	if _, err := Open(file); err == nil {
		t.Error("opening a file as a worktree succeeded")
	}
	if _, err := Open(filepath.Join(root, "nope")); err == nil {
		t.Error("opening a missing directory succeeded")
	}
}
