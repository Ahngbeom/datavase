package visits

import (
	"os"
	"path/filepath"
	"testing"
)

// The welcome is for someone who has never opened dv here, and only them: a
// second session that still reports none before it would greet a returning
// user as a stranger every time they start.
func TestASecondSessionKnowsTheFirstHappened(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datavase", "visits")

	first, err := Record(path)
	if err != nil {
		t.Fatalf("first Record() error = %v", err)
	}
	second, err := Record(path)
	if err != nil {
		t.Fatalf("second Record() error = %v", err)
	}

	if first != 0 {
		t.Errorf("a machine that has never run dv reports %d sessions before this one", first)
	}
	if second != 1 {
		t.Errorf("the second session reports %d before it, want 1", second)
	}
}

// Somewhere that cannot be written costs the memory of having been here, and
// nothing more: the caller still gets an answer it can act on.
func TestAnUnwritableStateDirectoryStillAnswers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	seen, err := Record(filepath.Join(dir, "datavase", "visits"))
	if err == nil {
		t.Error("a write that cannot have happened was reported as done")
	}
	if seen != 0 {
		t.Errorf("an unwritable directory reports %d sessions before this one", seen)
	}
}

// A file someone has edited by hand still says dv was here. Treating it as
// "never" would bring the welcome back for a user who has long since seen it.
func TestAnUnreadableCountStillMeansSomeoneWasHere(t *testing.T) {
	path := filepath.Join(t.TempDir(), "visits")
	if err := os.WriteFile(path, []byte("not a number\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	seen, _ := Record(path)
	if seen == 0 {
		t.Error("a garbled count was read as a first visit")
	}
}
