package glab

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"testing"
)

// withRun builds a CLI whose glab is answered from memory, so the tests need
// no glab installation and no network.
func withRun(run func(ctx context.Context, args ...string) ([]byte, error)) *CLI {
	return &CLI{run: run}
}

// answering returns out for any call, recording the arguments it was given.
func answering(out string, got *[]string) func(context.Context, ...string) ([]byte, error) {
	return func(_ context.Context, args ...string) ([]byte, error) {
		*got = args
		return []byte(out), nil
	}
}

// The whole point: the token datavase uses is the one glab already holds.
func TestTheTokenComesBackWithoutItsSurroundingWhitespace(t *testing.T) {
	var args []string
	c := withRun(answering("glpat-abc123\n", &args))

	token, err := c.Token(context.Background(), "gitlab.example.com")
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token != "glpat-abc123" {
		t.Errorf("Token() = %q, want the token with the newline stripped", token)
	}
}

// This is the contract everything else rests on, and it is not the obvious
// one: glab exits 0 for a host it has never been logged in to, and simply
// prints nothing. Reading the exit code alone would hand back an empty token
// and turn a missing login into a puzzling 401 much later.
func TestNoOutputMeansNoTokenRatherThanAnEmptyOne(t *testing.T) {
	for _, out := range []string{"", "\n", "   \n\t"} {
		c := withRun(func(context.Context, ...string) ([]byte, error) {
			return []byte(out), nil
		})

		token, err := c.Token(context.Background(), "gitlab.example.com")
		if !errors.Is(err, ErrNoToken) {
			t.Errorf("Token() with output %q gave error %v, want ErrNoToken", out, err)
		}
		if token != "" {
			t.Errorf("Token() returned %q alongside the error", token)
		}
	}
}

// The fix for a missing glab and the fix for a missing login are different
// sentences, so the caller has to be able to tell them apart.
func TestAMissingGlabIsDistinguishableFromAMissingLogin(t *testing.T) {
	c := withRun(func(context.Context, ...string) ([]byte, error) {
		return nil, exec.ErrNotFound
	})

	_, err := c.Token(context.Background(), "gitlab.example.com")
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Token() error = %v, want ErrNotInstalled", err)
	}
	if errors.Is(err, ErrNoToken) {
		t.Error("a missing glab was reported as a missing login")
	}
}

// The arguments are the contract with glab. Pinning them here is what makes a
// rename in glab a failing test rather than a session that quietly has no
// token.
func TestTheTokenIsAskedForByHostExplicitly(t *testing.T) {
	var args []string
	c := withRun(answering("t\n", &args))

	if _, err := c.Token(context.Background(), "gitlab.example.com"); err != nil {
		t.Fatal(err)
	}

	want := []string{"config", "get", "token", "--host", "gitlab.example.com"}
	if !slices.Equal(args, want) {
		t.Errorf("ran glab %v, want glab %v", args, want)
	}
}

// glab picks its host from the current directory's git remote when it is not
// told one. datavase runs in whatever directory the user happened to be in,
// which may be a checkout of something else entirely.
func TestTheHostIsNeverLeftForGlabToGuess(t *testing.T) {
	var args []string
	c := withRun(answering("t\n", &args))

	if _, err := c.Token(context.Background(), ""); err == nil {
		t.Error("an empty host was accepted; glab would have guessed one from the working directory")
	}
	if len(args) != 0 {
		t.Errorf("glab was run anyway, with %v", args)
	}
}

// A failure that is neither of the known two must still say what happened.
func TestAnUnrecognisedFailureIsPassedOnRatherThanRelabelled(t *testing.T) {
	sentinel := errors.New("keyring locked")
	c := withRun(func(context.Context, ...string) ([]byte, error) {
		return nil, sentinel
	})

	_, err := c.Token(context.Background(), "gitlab.example.com")
	if !errors.Is(err, sentinel) {
		t.Errorf("Token() error = %v, want it to carry the underlying failure", err)
	}
}
