package worktree

import "testing"

// A rename occupies two NUL-terminated tokens, and the file that now exists
// is the first of them. Treating the pair as two ordinary entries marked the
// vanished path as changed and left the real one unmarked.
func TestRenameMarksThePathThatStillExists(t *testing.T) {
	out := []byte("R  migrations/002_add_index.sql\x00migrations/002_index.sql\x00" +
		" M migrations/001_init.sql\x00")

	got := parsePorcelain(out)

	if got["migrations/002_add_index.sql"] != StatusRenamed {
		t.Errorf("the renamed file is not marked as renamed: %q", got["migrations/002_add_index.sql"])
	}
	if got["migrations/001_init.sql"] != StatusModified {
		t.Errorf("the entry after a rename was lost: %q", got["migrations/001_init.sql"])
	}
	// The count is asserted rather than the absence of the old path: read as
	// an entry of its own, that token yields a mangled name rather than the
	// path itself, which an absence check would happily let through.
	if len(got) != 2 {
		t.Errorf("the path the rename came from leaked into the listing: %#v", got)
	}
}

// A file staged as new and then edited again is still a new file; reporting
// it as merely modified hides that it does not exist on the branch yet.
func TestStagedStatusWinsOverTheWorkingTreeOne(t *testing.T) {
	cases := map[string]Status{
		"AM new.sql\x00":      StatusAdded,
		" M edited.sql\x00":   StatusModified,
		"MM both.sql\x00":     StatusModified,
		"?? scratch.sql\x00":  StatusUntracked,
		"UU conflict.sql\x00": StatusConflict,
		" D gone.sql\x00":     StatusDeleted,
	}

	for out, want := range cases {
		got := parsePorcelain([]byte(out))
		if len(got) != 1 {
			t.Fatalf("%q produced %d entries, want 1", out, len(got))
		}
		for path, status := range got {
			if status != want {
				t.Errorf("%q marked %s as %q, want %q", out, path, status, want)
			}
		}
	}
}

// -z output is never quoted, so a space in a path is part of the path. A
// parser that split on whitespace truncated the name at the first space.
func TestPathsKeepTheirSpaces(t *testing.T) {
	got := parsePorcelain([]byte("?? migrations/add index.sql\x00"))

	if got["migrations/add index.sql"] != StatusUntracked {
		t.Errorf("a path containing a space was not preserved: %#v", got)
	}
}

// git terminates every entry, but a truncated read must not take the last
// file down with it.
func TestAFinalEntryWithoutItsTerminatorIsStillReported(t *testing.T) {
	got := parsePorcelain([]byte(" M migrations/001_init.sql"))

	if got["migrations/001_init.sql"] != StatusModified {
		t.Errorf("an unterminated final entry was dropped: %#v", got)
	}
}

// A clean tree produces no output at all, which must not be an error or a
// phantom entry.
func TestACleanTreeYieldsNothing(t *testing.T) {
	if got := parsePorcelain(nil); len(got) != 0 {
		t.Errorf("empty output produced %#v", got)
	}
	if got := parsePorcelain([]byte("\x00")); len(got) != 0 {
		t.Errorf("a lone terminator produced %#v", got)
	}
}

// An entry too short to carry a status is malformed; skipping it is better
// than indexing past the end of it.
func TestAMalformedEntryIsSkippedRatherThanPanicking(t *testing.T) {
	got := parsePorcelain([]byte("M\x00 M real.sql\x00"))

	if len(got) != 1 || got["real.sql"] != StatusModified {
		t.Errorf("a malformed entry disturbed the rest: %#v", got)
	}
}

// ls-files terminates every name, so a naive split leaves an empty final
// element that would become a file with no name.
func TestTheTrailingTerminatorDoesNotBecomeAnEmptyName(t *testing.T) {
	got := parseNulList([]byte("a.sql\x00b.sql\x00"))

	want := []string{"a.sql", "b.sql"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
}

// The marker is one character on screen; the description is what the finder
// shows beside it, so every status the parser can produce needs one.
func TestEveryStatusDescribesItself(t *testing.T) {
	for _, s := range []Status{
		StatusNone, StatusModified, StatusAdded, StatusDeleted,
		StatusRenamed, StatusUntracked, StatusConflict,
	} {
		if s.Describe() == "" {
			t.Errorf("status %q has no description", s)
		}
	}
}
