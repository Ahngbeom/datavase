// Package recent remembers the directories that have been attached, most
// recent first.
//
// It is a convenience and nothing more, so every failure in it is swallowed:
// a read-only home directory should cost the list, not the session. That is
// the same bargain the catalog cache and the query history strike.
//
// A JSON file rather than a table. The catalog database is the schema cache
// and the history database is the query log; a list of twenty paths belongs in
// neither, and giving it a third database would be more machinery than the
// thing it stores.
package recent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Limit is how many paths are kept. Long enough to cover the checkouts someone
// moves between, short enough that the list is still a shortcut.
const Limit = 20

// List is an ordered set of paths, most recent first.
//
// The zero value is an empty list that remembers nothing across runs, which is
// what a session gets when the state directory could not be read.
type List struct {
	// path is where Save writes. Carried on the list rather than passed to
	// Save so that a caller cannot hold one and write it to the other.
	path    string
	entries []string
}

// Open reads a list.
//
// A missing file is the ordinary first run, and an unreadable or malformed one
// is treated the same way: starting over loses a list of paths, whereas
// refusing to start loses the feature for good, and nobody is going to hand-
// repair a state file to get their directory history back.
func Open(path string) (*List, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &List{path: path}, nil
	}

	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		return &List{path: path}, nil
	}

	l := &List{path: path}
	// Rebuilt through Add, backwards, so that a file edited by hand cannot put
	// duplicates or more than Limit entries into a live list.
	for i := len(entries) - 1; i >= 0; i-- {
		l.Add(entries[i])
	}
	return l, nil
}

// Add records a path, moving it to the front if it is already known.
func (l *List) Add(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}

	kept := make([]string, 0, len(l.entries)+1)
	kept = append(kept, path)
	for _, e := range l.entries {
		if e == path {
			continue
		}
		kept = append(kept, e)
		if len(kept) == Limit {
			break
		}
	}
	l.entries = kept
}

// Entries returns the paths, most recent first.
func (l *List) Entries() []string {
	// A copy: the caller ranks and filters these, and a sort in place would
	// silently reorder the history it was reading.
	out := make([]string, len(l.entries))
	copy(out, l.entries)
	return out
}

// Save writes the list, creating the directory if it is not there.
//
// A list with nowhere to write is not an error: it is the in-memory fallback a
// session gets when the state directory was unusable, and it should keep
// working for the rest of the session.
func (l *List) Save() error {
	if l.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.Marshal(l.entries)
	if err != nil {
		return fmt.Errorf("encoding recent directories: %w", err)
	}
	// Owner only: which directories someone works in is not public knowledge.
	if err := os.WriteFile(l.path, data, 0o600); err != nil {
		return fmt.Errorf("writing recent directories: %w", err)
	}
	return nil
}

// DefaultPath returns the on-disk location, alongside the catalog cache and
// the query history.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "datavase", "recent-dirs.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "datavase", "recent-dirs.json"), nil
}
