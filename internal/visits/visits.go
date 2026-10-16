// Package visits counts the sessions opened on this machine.
//
// The count is all the interface needs to greet someone once and to vary
// what it suggests from one session to the next. This package knows nothing
// about either: what a count of zero means is the interface's business.
package visits

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Record reports how many sessions were counted before this one, and counts
// this one.
//
// The count is returned even when it could not be written back. Like the
// schema cache and the history, this file is optional, and a home directory
// that refuses writes should cost the memory of having been here rather than
// the session.
func Record(path string) (int, error) {
	seen, err := read(path)
	if err != nil {
		return seen, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return seen, fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(seen+1)+"\n"), 0o600); err != nil {
		return seen, fmt.Errorf("writing %s: %w", path, err)
	}
	return seen, nil
}

// read is the count so far. A file that exists but does not hold a number
// still says someone was here, so it reads as one rather than none.
func read(path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 1, fmt.Errorf("reading %s: %w", path, err)
	}

	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 1 {
		return 1, nil
	}
	return n, nil
}
