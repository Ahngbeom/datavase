package intro

import (
	"os"
	"path/filepath"
	"testing"
)

// The ordinary first run: nothing has been recorded, so the introduction is
// due.
func TestNothingRecordedMeansItHasNotBeenSeen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datavase", "intro-seen")

	if Seen(path) {
		t.Error("Seen() = true before anything was recorded")
	}
}

// Shown once and never again is the whole of what this decides.
func TestRecordingItMeansItHasBeenSeen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datavase", "intro-seen")

	if err := MarkSeen(path); err != nil {
		t.Fatalf("MarkSeen() error = %v", err)
	}
	if !Seen(path) {
		t.Error("Seen() = false after MarkSeen()")
	}
}

// The state directory does not exist on a machine that has never run
// datavase, which is precisely the machine this is for.
func TestRecordingCreatesTheStateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datavase", "intro-seen")

	if err := MarkSeen(path); err != nil {
		t.Fatalf("MarkSeen() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Stat() error = %v, want the marker to exist", err)
	}
}

// Recording twice is what happens when two sessions are open at once, and it
// is not a failure.
func TestRecordingTwiceIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "intro-seen")

	if err := MarkSeen(path); err != nil {
		t.Fatalf("first MarkSeen() error = %v", err)
	}
	if err := MarkSeen(path); err != nil {
		t.Errorf("second MarkSeen() error = %v", err)
	}
}

// A session with nowhere to write is the fallback a read-only home directory
// gets, and it must not decide that the introduction has been seen — an empty
// path is the absence of an answer, not a yes.
func TestAnUnknownPathIsNotAnAnswer(t *testing.T) {
	if Seen("") {
		t.Error(`Seen("") = true, want an unknown location to count as unseen`)
	}
	if err := MarkSeen(""); err != nil {
		t.Errorf(`MarkSeen("") error = %v, want nowhere to write to be no error`, err)
	}
}

// Alongside the schema cache, the query history and the directory list, and
// honouring the same variable they do.
func TestTheDefaultPathIsUnderTheStateDirectory(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if want := filepath.Join(state, "datavase", "intro-seen"); path != want {
		t.Errorf("DefaultPath() = %q, want %q", path, want)
	}
}
