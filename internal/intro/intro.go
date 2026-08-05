// Package intro remembers whether the introduction has been shown.
//
// The whole state is one bit, so a file's existence is the whole store: a
// database for a boolean would be more machinery than the thing it holds, and
// the catalog cache and the query history are already the two places where a
// third would not belong.
//
// It strikes the same bargain they do — every failure is swallowed, because a
// read-only home directory should cost a note about which keys to press, not
// the session. It leans one way on purpose: a marker that could not be written
// leaves the introduction to be shown again, which is a moment's annoyance,
// where guessing the other way would hide it from the person it is for.
package intro

import (
	"fmt"
	"os"
	"path/filepath"
)

// Seen reports whether the introduction has already been shown.
//
// An empty path is a session that never found a state directory. That is the
// absence of an answer rather than a yes, so it counts as unseen.
func Seen(path string) bool {
	if path == "" {
		return false
	}

	_, err := os.Stat(path)
	return err == nil
}

// MarkSeen records that the introduction has been shown.
//
// Writing it twice is what two sessions opened at once do, and is not a
// failure: the file's existence is the state, and it already exists.
func MarkSeen(path string) error {
	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	// Empty: the name carries the whole meaning, and anything written inside
	// would be a second thing to keep true.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return fmt.Errorf("recording that the introduction was shown: %w", err)
	}
	return nil
}

// DefaultPath returns the on-disk location, alongside the catalog cache, the
// query history and the directory list.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "datavase", "intro-seen"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "datavase", "intro-seen"), nil
}
