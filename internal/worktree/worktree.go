// Package worktree lists the SQL files of a working directory and reports
// what git says about them.
//
// It knows nothing about tview and nothing about databases: it answers "which
// files are here" and "which of them changed", and the caller decides what to
// do with the answers.
//
// git is optional. A directory that is not a repository, or a machine with no
// git installed, still yields its .sql files — only the branch and the
// per-file markers go missing. That is the same bargain the schema cache and
// the query history make: a missing convenience must not cost the session.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxFileSize is the largest file that will be loaded into the editor.
//
// A mysqldump is routinely hundreds of megabytes, and handing one to a text
// widget does not produce a slow interface — it produces one that never draws
// again. Refusing is the only outcome that leaves the user something working.
const MaxFileSize = 4 << 20

// maxWalkFiles caps the listing when git is not there to do the filtering.
// Without an index to consult, a walk can wander into a node_modules nobody
// meant to attach.
const maxWalkFiles = 2000

// gitTimeout bounds every git invocation. git is a convenience here, so one
// that hangs — a stale index lock, a network-backed filesystem — must not
// take the file list with it.
const gitTimeout = 2 * time.Second

var (
	// ErrOutsideWorktree refuses a path that resolves beyond the attached
	// directory. The interface hands back whatever path it was given, and a
	// stale or crafted one must not reach a file the user never attached.
	ErrOutsideWorktree = errors.New("path is outside the worktree")

	// ErrTooLarge refuses a file that would wedge the editor.
	ErrTooLarge = errors.New("file is too large to open")
)

// Status of a file, and the file itself, live in status.go.

// File is one SQL file of the worktree.
type File struct {
	// Rel is relative to the attached directory, which is what the user
	// chose and therefore what they should be shown.
	Rel string
	Abs string

	Status Status
}

// Snapshot is one reading of the worktree: what is in it, and where git says
// the work stands.
//
// It is a value rather than state on Worktree so that a rescan cannot leave
// the file list and the branch describing different moments.
type Snapshot struct {
	// Branch is the checked-out branch, or the short commit id when HEAD is
	// detached. Empty when git had nothing to say.
	Branch   string
	Detached bool
	Dirty    bool

	// Remote is the origin URL, empty when there is no origin. It is read here
	// rather than asked for because the project a session wants to browse is
	// nearly always the one the checkout came from.
	Remote string

	Files []File

	// Truncated reports that the listing hit its cap, so the user is told
	// rather than left believing they saw everything.
	Truncated bool
}

// Worktree is an attached directory.
type Worktree struct {
	// Root is absolute, with symlinks resolved, so containment checks compare
	// like with like.
	Root string

	// repoRoot is the enclosing repository, empty when there is none. git is
	// always run from here rather than from Root: `git status --porcelain`
	// reports paths relative to the current directory in some configurations
	// and to the repository root in others, and running at the root makes the
	// two the same answer.
	repoRoot string
}

// Open attaches a directory, resolving and checking it before anything else
// depends on it.
//
// git is not consulted here — that happens on Scan, so attaching stays
// instant even on a repository large enough that a status takes a moment.
func Open(path string) (*Worktree, error) {
	expanded, err := expandHome(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(expanded)
	if err != nil {
		return nil, err
	}

	// Symlinks are resolved once, here, so that every later containment check
	// compares resolved paths against a resolved root.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", abs)
	}

	return &Worktree{Root: resolved, repoRoot: repositoryRoot(resolved)}, nil
}

// Name is the worktree's last path element, which is what identifies it on a
// status bar too narrow for the whole path.
func (w *Worktree) Name() string { return filepath.Base(w.Root) }

// Scan lists the SQL files and reads the branch.
func (w *Worktree) Scan(ctx context.Context) (Snapshot, error) {
	if w.repoRoot == "" {
		files, truncated, err := w.walk()
		return Snapshot{Files: files, Truncated: truncated}, err
	}

	snap := w.gitBranch(ctx)
	snap.Remote = w.gitRemote(ctx)
	statuses := w.gitStatus(ctx)
	snap.Dirty = len(statuses) > 0

	files, err := w.gitFiles(ctx, statuses)
	if err != nil {
		// git answered about the branch but not about the files, which would
		// leave an empty list looking like an empty directory. The walk is
		// the honest fallback.
		files, snap.Truncated, err = w.walk()
		if err != nil {
			return snap, err
		}
	}
	snap.Files = files
	return snap, nil
}

// Read loads a file's text along with the stamp identifying the version read.
func (w *Worktree) Read(rel string) (string, Stamp, error) {
	abs, err := w.resolve(rel)
	if err != nil {
		return "", Stamp{}, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", Stamp{}, err
	}
	if info.Size() > MaxFileSize {
		return "", Stamp{}, fmt.Errorf("%s is %s: %w", rel, humanSize(info.Size()), ErrTooLarge)
	}

	b, err := os.ReadFile(abs)
	if err != nil {
		return "", Stamp{}, err
	}
	return string(b), stampOf(info), nil
}

// Write replaces a file's contents and reports the new stamp.
//
// The replacement goes through a temporary file in the same directory and a
// rename, so a crash partway through leaves the previous version intact
// rather than a truncated migration.
func (w *Worktree) Write(rel, text string) (Stamp, error) {
	abs, err := w.resolve(rel)
	if err != nil {
		return Stamp{}, err
	}

	// A file being edited already exists; its mode is the user's decision and
	// replacing it must not quietly widen it.
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(abs); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(abs), "."+filepath.Base(abs)+".*")
	if err != nil {
		return Stamp{}, err
	}
	tmpName := tmp.Name()

	// Every failure from here on has to take the temporary file with it, or
	// the next listing offers a half-written stray.
	cleanup := func(err error) (Stamp, error) {
		tmp.Close()
		os.Remove(tmpName)
		return Stamp{}, err
	}

	if _, err := tmp.WriteString(text); err != nil {
		return cleanup(err)
	}
	// Renaming an unsynced file can leave an empty one behind a power loss,
	// which is the failure this whole dance exists to prevent.
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return Stamp{}, err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return Stamp{}, err
	}
	if err := os.Rename(tmpName, abs); err != nil {
		os.Remove(tmpName)
		return Stamp{}, err
	}

	return w.Stat(rel)
}

// Stat reports the stamp of a file without reading it, which is how a save
// notices the file changed since it was opened.
func (w *Worktree) Stat(rel string) (Stamp, error) {
	abs, err := w.resolve(rel)
	if err != nil {
		return Stamp{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Stamp{}, err
	}
	return stampOf(info), nil
}

// Stamp identifies the version of a file that was read.
//
// It is compared, never interpreted, so a comparable struct is the whole of
// what it needs to be.
type Stamp struct {
	ModTime time.Time
	Size    int64
}

func stampOf(info fs.FileInfo) Stamp {
	return Stamp{ModTime: info.ModTime(), Size: info.Size()}
}

// resolve turns a worktree-relative path into an absolute one, refusing
// anything that escapes the root.
//
// The check is on the cleaned path rather than on the presence of "..":
// "sub/../../secret.sql" contains no leading "..", and a textual check would
// pass it straight through.
func (w *Worktree) resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%s: %w", rel, ErrOutsideWorktree)
	}

	abs := filepath.Join(w.Root, rel)
	inside, err := filepath.Rel(w.Root, abs)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: %w", rel, ErrOutsideWorktree)
	}
	return abs, nil
}

// isSQL reports whether a path is one datavase would open. The comparison is
// case-insensitive because Windows and macOS filesystems are.
func isSQL(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".sql")
}

// walk lists SQL files without git, which is what a plain directory gets.
func (w *Worktree) walk() ([]File, bool, error) {
	var (
		files     []File
		truncated bool
	)

	err := filepath.WalkDir(w.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// One unreadable directory is not a reason to abandon the listing.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) && path != w.Root {
				return fs.SkipDir
			}
			return nil
		}
		if !isSQL(d.Name()) {
			return nil
		}
		if len(files) >= maxWalkFiles {
			truncated = true
			return filepath.SkipAll
		}

		rel, err := filepath.Rel(w.Root, path)
		if err != nil {
			return nil
		}
		files = append(files, File{Rel: filepath.ToSlash(rel), Abs: path})
		return nil
	})
	if err != nil {
		return nil, truncated, err
	}

	sortFiles(files)
	return files, truncated, nil
}

// skipDir names directories that never hold work worth listing. Descending
// into node_modules is how a listing becomes useless rather than merely long.
func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".idea", ".vscode":
		return true
	}
	return false
}

// sortFiles puts changed files first and then orders by path, so the work in
// progress is at the top where it is being looked for.
func sortFiles(files []File) {
	sort.SliceStable(files, func(i, j int) bool {
		ci, cj := files[i].Status != StatusNone, files[j].Status != StatusNone
		if ci != cj {
			return ci
		}
		return files[i].Rel < files[j].Rel
	})
}

// repositoryRoot finds the enclosing repository, or "" when there is none or
// git is not installed.
func repositoryRoot(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	out, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}

	root := strings.TrimSpace(string(out))
	if root == "" {
		return ""
	}
	// Resolved for the same reason Root is: the two are compared.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return resolved
	}
	return root
}

// gitBranch reads the checked-out branch.
//
// symbolic-ref is asked first because it answers in a repository with no
// commits yet, where rev-parse has no HEAD to resolve. It fails on a detached
// HEAD, which is exactly when the commit id is the useful answer — reporting
// the literal "HEAD" would read as a branch someone had created.
func (w *Worktree) gitBranch(ctx context.Context) Snapshot {
	if out, err := runGit(ctx, w.repoRoot, "symbolic-ref", "--short", "HEAD"); err == nil {
		return Snapshot{Branch: strings.TrimSpace(string(out))}
	}

	out, err := runGit(ctx, w.repoRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return Snapshot{}
	}
	return Snapshot{Branch: strings.TrimSpace(string(out)), Detached: true}
}

// gitRemote reads the origin URL, if there is one.
//
// A repository with no origin is ordinary — a scratch checkout, or one whose
// remote is called something else — so this reports nothing rather than
// failing the scan it is part of.
func (w *Worktree) gitRemote(ctx context.Context) string {
	out, err := runGit(ctx, w.repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitStatus maps repository-relative paths to their status.
//
// -uall is not optional: without it a newly created directory is reported as
// one "?? scratch/" entry, and every SQL file inside the migration folder
// someone just made would be invisible.
func (w *Worktree) gitStatus(ctx context.Context) map[string]Status {
	out, err := runGit(ctx, w.repoRoot, "status", "--porcelain", "-z", "-uall")
	if err != nil {
		return nil
	}
	return parsePorcelain(out)
}

// gitFiles lists the SQL files git knows about, tracked or not.
//
// The union of ls-files and the untracked entries of status is what makes the
// listing respect .gitignore without this package ever parsing one: a build
// artefact or a dump directory is simply never named by either.
func (w *Worktree) gitFiles(ctx context.Context, statuses map[string]Status) ([]File, error) {
	out, err := runGit(ctx, w.repoRoot, "ls-files", "-z", "--", "*.sql")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var files []File

	add := func(repoRel string) {
		if seen[repoRel] || !isSQL(repoRel) {
			return
		}

		abs := filepath.Join(w.repoRoot, filepath.FromSlash(repoRel))
		rel, err := filepath.Rel(w.Root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			// Elsewhere in the repository, which is not what was attached.
			return
		}

		seen[repoRel] = true
		files = append(files, File{
			Rel:    filepath.ToSlash(rel),
			Abs:    abs,
			Status: statuses[repoRel],
		})
	}

	for _, name := range parseNulList(out) {
		add(name)
	}
	// Untracked files are in status and nowhere else; a file created five
	// minutes ago is precisely the one being looked for.
	for path, status := range statuses {
		if status == StatusUntracked {
			add(path)
		}
	}

	// A deleted file is still listed by ls-files, and offering to open one
	// produces a confusing "no such file" a moment later.
	kept := files[:0]
	for _, f := range files {
		if f.Status == StatusDeleted {
			continue
		}
		kept = append(kept, f)
	}

	sortFiles(kept)
	return kept, nil
}

// runGit invokes git in dir with a bounded lifetime.
func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if dir == "" {
		return nil, errors.New("no repository")
	}

	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	// GIT_OPTIONAL_LOCKS=0 keeps a status from taking the index lock, so
	// browsing files here cannot make the user's own git commands fail with
	// "unable to create index.lock" in the terminal next door.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	cmd.Stderr = nil

	return cmd.Output()
}

// expandHome resolves a leading ~, which is how anyone types a path to a
// worktree.
func expandHome(path string) (string, error) {
	if path == "" {
		return "", errors.New("no directory given")
	}
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

// humanSize keeps a refusal readable; the number is why the file was refused.
func humanSize(n int64) string {
	const unit = 1 << 20
	if n < unit {
		return fmt.Sprintf("%d bytes", n)
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/unit)
}
