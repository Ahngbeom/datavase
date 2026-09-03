package version

import "testing"

// A go install'd or goreleaser-built binary already carries a real version,
// which is what the handshake's existing check compares. Adding an mtime on
// top of that would make two identical release binaries refuse each other —
// a regression this test exists to catch.
func TestBuildFingerprintIsEmptyForAReleaseVersion(t *testing.T) {
	old := injected
	injected = "v0.7.0"
	t.Cleanup(func() { injected = old })

	if got := BuildFingerprint(); got != "" {
		t.Errorf("BuildFingerprint() = %q, want empty for a released version", got)
	}
}

// A `go test` binary reports "(devel)" the same way an ordinary local build
// does, which is what makes this test exercise the real path rather than a
// stand-in for it.
func TestBuildFingerprintIsSetForADevelopmentBuild(t *testing.T) {
	if String() != "(devel)" {
		t.Skipf("this binary reports %q, not (devel); nothing to distinguish", String())
	}

	if got := BuildFingerprint(); got == "" {
		t.Error("BuildFingerprint() = \"\", want a non-empty value for a development build")
	}
}

// Two builds are the same build only if nothing has been rebuilt since; the
// fingerprint exists to say so even though String() cannot.
func TestBuildFingerprintIsStableAcrossCalls(t *testing.T) {
	if String() != "(devel)" {
		t.Skipf("this binary reports %q, not (devel); nothing to distinguish", String())
	}

	first := BuildFingerprint()
	second := BuildFingerprint()
	if first != second {
		t.Errorf("BuildFingerprint() = %q then %q, want the same value without a rebuild in between", first, second)
	}
}
