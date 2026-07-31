//go:build integration

package glab

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// Everything here rests on one measured claim about a tool nobody in this
// repository controls: that `glab config get token --host H` exits
// successfully and prints nothing for a host it does not know.
//
// If a future glab starts failing instead, Token would report that failure
// verbatim rather than the sentence that actually helps ("run glab auth
// login"). This pins the claim against the real binary. A host that cannot
// exist is used so the check needs no login and reaches no network.
func TestGlabAnswersWithSilenceForAHostItDoesNotKnow(t *testing.T) {
	if _, err := exec.LookPath("glab"); err != nil {
		t.Skip("glab is not installed")
	}

	_, err := New().Token(context.Background(), "no-such-host.invalid")
	if !errors.Is(err, ErrNoToken) {
		t.Fatalf("Token() error = %v, want ErrNoToken — glab's contract has changed", err)
	}
}

// And the arguments are still the ones it accepts. A renamed key or flag would
// otherwise surface as an unexplained missing token.
func TestTheRealGlabAcceptsTheArgumentsWeSend(t *testing.T) {
	if _, err := exec.LookPath("glab"); err != nil {
		t.Skip("glab is not installed")
	}

	// Rejected arguments make glab exit non-zero, which Token reports as a
	// wrapped failure rather than ErrNoToken.
	_, err := New().Token(context.Background(), "no-such-host.invalid")
	if err != nil && !errors.Is(err, ErrNoToken) {
		t.Errorf("glab rejected the arguments: %v", err)
	}
}
