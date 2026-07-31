package worktree

import "bytes"

// Status is what git says about one file, reduced to a single character.
//
// git reports two: one for the index and one for the working tree. The pane
// showing these has room for one, so the index status wins — a file staged as
// new and then edited again is still a file that does not exist on the branch
// yet, which is the more useful thing to know.
type Status rune

const (
	// StatusNone means git had nothing to say: the file is committed and
	// unchanged. It is the zero value so a file map needs no initialisation.
	StatusNone      Status = 0
	StatusModified  Status = 'M'
	StatusAdded     Status = 'A'
	StatusDeleted   Status = 'D'
	StatusRenamed   Status = 'R'
	StatusUntracked Status = '?'
	StatusConflict  Status = 'U'
)

// Marker is the character shown beside the file name. Clean files get a space
// so the names stay in one column.
func (s Status) Marker() string {
	if s == StatusNone {
		return " "
	}
	return string(rune(s))
}

// Describe names the status in words, for the line under the file name.
func (s Status) Describe() string {
	switch s {
	case StatusModified:
		return "modified"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusUntracked:
		return "untracked"
	case StatusConflict:
		return "conflicted"
	default:
		return "tracked"
	}
}

// parsePorcelain reads `git status --porcelain -z` into a status per path.
//
// The -z form is used rather than the line-based one because git quotes and
// escapes any path containing a space or a non-ASCII byte in the latter, and
// unquoting that correctly is a job nobody should be doing twice. Under -z
// paths are literal and NUL-separated.
//
// A rename occupies two entries — the path that now exists, then the one it
// came from — so the stream is walked token by token rather than split.
func parsePorcelain(out []byte) map[string]Status {
	statuses := make(map[string]Status)

	rest := out
	for len(rest) > 0 {
		var entry []byte
		entry, rest = nextToken(rest)

		// "XY " and at least one character of path.
		if len(entry) < 4 {
			continue
		}
		index, worktree := entry[0], entry[1]
		path := string(entry[3:])

		status := Status(index)
		if index == ' ' {
			status = Status(worktree)
		}
		statuses[path] = status

		// The path a rename or copy came from follows as its own token, and
		// naming it would report a file that is no longer there.
		if index == 'R' || index == 'C' || worktree == 'R' || worktree == 'C' {
			_, rest = nextToken(rest)
		}
	}
	return statuses
}

// parseNulList splits NUL-terminated output, which `git ls-files -z` produces.
//
// The terminator is on every name including the last, so splitting alone
// would leave an empty final element — a file with no name.
func parseNulList(out []byte) []string {
	var names []string

	rest := out
	for len(rest) > 0 {
		var name []byte
		name, rest = nextToken(rest)
		if len(name) > 0 {
			names = append(names, string(name))
		}
	}
	return names
}

// nextToken splits off everything up to the next NUL, returning the token and
// what follows it. A final token without its terminator is still returned:
// output truncated mid-write should not silently lose its last entry.
func nextToken(b []byte) (token, rest []byte) {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return b[:i], b[i+1:]
	}
	return b, nil
}
