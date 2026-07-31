// Package glab borrows credentials from the GitLab CLI.
//
// datavase stores no GitLab token of its own. Anyone reaching a self-managed
// instance already has glab logged in to it — often through SSO, with a token
// that rotates on its own — and asking for a second personal access token
// would mean a second credential to create, rotate and revoke for nothing.
//
// Only the credential is borrowed. The request itself is still made by
// internal/gitlab, which keeps that package a plain HTTP client whose tests
// need neither glab nor a network. Handing the whole request to `glab api`
// was the other option and is deliberately not taken: GitLab answers 404
// rather than 401 for a private project seen without permission, so the most
// common failure of all — an expired login — would arrive disguised as "no
// such project".
package glab

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// timeout bounds the lookup. No network is involved — glab reads its own
// configuration and the OS keyring — but a keyring can prompt, and a prompt
// nobody answers must not hang the session.
const timeout = 10 * time.Second

var (
	// ErrNotInstalled means glab is not on the PATH.
	ErrNotInstalled = errors.New("glab is not installed")
	// ErrNoToken means glab is installed but has no credential for the host.
	ErrNoToken = errors.New("glab has no token for this host")
)

// CLI runs the GitLab CLI.
type CLI struct {
	// run executes glab with the given arguments. A field so the tests can
	// answer from memory rather than needing glab installed and logged in —
	// the same reason cli.App carries ReadPassword.
	run func(ctx context.Context, args ...string) ([]byte, error)
}

// New returns a CLI that runs the real glab.
func New() *CLI {
	return &CLI{run: runGlab}
}

// Token returns the credential glab holds for a host.
//
// An empty result is the answer for a host glab has never been logged in to:
// it exits successfully and prints nothing. Reading the exit code alone would
// hand back an empty token and turn a missing login into a puzzling 401 much
// later, so emptiness is reported as ErrNoToken here.
func (c *CLI) Token(ctx context.Context, host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		// Left to itself, glab takes the host from the current directory's git
		// remote. datavase runs in whatever directory the user was already in,
		// which may be a checkout of something else entirely.
		return "", errors.New("no GitLab host to ask glab about")
	}

	out, err := c.run(ctx, "config", "get", "token", "--host", host)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrNotInstalled
		}
		return "", fmt.Errorf("asking glab for the %s token: %w", host, err)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("%s: %w", host, ErrNoToken)
	}
	return token, nil
}

func runGlab(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "glab", args...)
	// Nothing on stdin: the interface owns the terminal, and a child that
	// waited for input there would deadlock behind a screen it cannot draw on.
	cmd.Stdin = nil
	// Output captures stdout and, because Stderr is nil, files stderr on the
	// error — so a failure can explain itself without glab writing over the
	// interface.
	cmd.Stderr = nil
	return cmd.Output()
}
