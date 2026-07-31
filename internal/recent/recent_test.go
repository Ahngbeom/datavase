package recent

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Attaching the same directory twice is the ordinary case — it is the one you
// keep coming back to — and it must not push the others out of the list.
func TestReattachingADirectoryMovesItToTheFrontRatherThanRepeatingIt(t *testing.T) {
	l := &List{}
	l.Add("/a")
	l.Add("/b")
	l.Add("/a")

	if got, want := l.Entries(), []string{"/a", "/b"}; !slices.Equal(got, want) {
		t.Errorf("Entries() = %v, want %v", got, want)
	}
}

func TestTheListStopsGrowing(t *testing.T) {
	l := &List{}
	for i := 0; i < Limit*3; i++ {
		l.Add(filepath.Join("/dir", string(rune('a'+i%26)), string(rune('0'+i/26))))
	}

	if got := len(l.Entries()); got != Limit {
		t.Errorf("kept %d entries, want the cap of %d", got, Limit)
	}
}

// Entries the caller mutates must not reach back into the list.
func TestEntriesHandsBackACopy(t *testing.T) {
	l := &List{}
	l.Add("/a")
	l.Add("/b")

	got := l.Entries()
	got[0] = "/tampered"

	if l.Entries()[0] != "/b" {
		t.Errorf("mutating the returned slice changed the list: %v", l.Entries())
	}
}

func TestSavingAndReopeningKeepsTheOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "recent-dirs.json")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	l.Add("/first")
	l.Add("/second")
	if err := l.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, want := reopened.Entries(), []string{"/second", "/first"}; !slices.Equal(got, want) {
		t.Errorf("Entries() = %v, want %v", got, want)
	}
}

// This is a convenience, so nothing about it may cost the session. A file that
// is not there yet is the ordinary first run.
func TestAMissingFileOpensEmptyRatherThanFailing(t *testing.T) {
	l, err := Open(filepath.Join(t.TempDir(), "never-written.json"))
	if err != nil {
		t.Fatalf("Open() on a missing file returned %v, want no error", err)
	}
	if got := len(l.Entries()); got != 0 {
		t.Errorf("a missing file yielded %d entries", got)
	}
}

// A state file nobody can be expected to repair by hand must not lock the
// feature out for good. Starting over loses a list of paths; refusing to start
// loses the feature.
func TestACorruptFileStartsOverInsteadOfFailingForever(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent-dirs.json")
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open() on a corrupt file returned %v, want no error", err)
	}
	if got := len(l.Entries()); got != 0 {
		t.Errorf("a corrupt file yielded %d entries", got)
	}

	l.Add("/fresh")
	if err := l.Save(); err != nil {
		t.Fatalf("Save() over a corrupt file returned %v", err)
	}
	reopened, err := Open(path)
	if err != nil || len(reopened.Entries()) != 1 {
		t.Errorf("the corrupt file was never replaced: %v, %v", reopened.Entries(), err)
	}
}

// The list holds paths, and paths are not public knowledge.
func TestTheFileIsNotReadableByOtherUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "recent-dirs.json")

	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	l.Add("/somewhere/private")
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("file mode is %v, want nothing for group or other", perm)
	}
}

func TestBlankPathsAreNotRecorded(t *testing.T) {
	l := &List{}
	l.Add("")
	l.Add("   ")

	if got := len(l.Entries()); got != 0 {
		t.Errorf("recorded %d blank entries", got)
	}
}
