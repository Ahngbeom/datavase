package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// The status bar truncates from the right, so whatever is last is what a
// narrow terminal loses. Where a message carries both an account of what
// happened and the thing to do about it, the second has to come first.
func TestTheWayBackOutlastsTheDriversAccountOfTheFailure(t *testing.T) {
	cause := lostSessionCause(
		errors.New("acquiring a connection: dial tcp 127.0.0.1:13306: connect: connection refused"),
		"Super+R")

	s := status{phase: phaseFailed, err: cause}
	got := visibleText(s.renderWidth(80))

	if !strings.Contains(got, "Super+R reconnects") {
		t.Errorf("80 cells: %q lost the way back", got)
	}
	if !strings.Contains(got, "connection is gone") {
		t.Errorf("80 cells: %q does not say the connection is gone", got)
	}
}

// A path is long, a reason is short, and the reason is the half that says
// whether to try again or to pick another name.
func TestTheReasonAFileWasRefusedOutlastsThePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), strings.Repeat("report-", 8)+"final.csv")
	if _, err := writeResultFile(path, "first\n"); err != nil {
		t.Fatal(err)
	}
	_, err := writeResultFile(path, "second\n")
	if err == nil {
		t.Fatal("the second write replaced a file that was already there")
	}

	s := status{message: "save failed: " + err.Error()}
	got := visibleText(s.renderWidth(80))
	if !strings.Contains(got, "already exists") {
		t.Errorf("80 cells: %q lost the reason", got)
	}
}
