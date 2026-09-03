package version

import (
	"runtime/debug"
	"testing"
)

// A go install'd or goreleaser-built binary already carries a real version,
// which is what the handshake's existing check compares. Adding an mtime on
// top of that would make two identical release binaries refuse each other —
// a regression this test exists to catch.
func TestBuildFingerprintIsEmptyForAReleaseVersion(t *testing.T) {
	old := injected
	injected = "v0.7.0"
	t.Cleanup(func() { injected = old })

	vcsInfo := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "deadbeef"}}}
	if got := buildFingerprint(vcsInfo); got != "" {
		t.Errorf("buildFingerprint() = %q, want empty for a released version", got)
	}
}

// `go install module@version` builds from the module proxy's cache, not a
// version-controlled tree, so it carries no vcs.revision setting — that is
// the second exempt case fingerprinting must not disturb.
func TestBuildFingerprintIsEmptyWithoutVCSProvenance(t *testing.T) {
	old := injected
	injected = ""
	t.Cleanup(func() { injected = old })

	noVCSInfo := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "GOOS", Value: "darwin"}}}
	if got := buildFingerprint(noVCSInfo); got != "" {
		t.Errorf("buildFingerprint() = %q, want empty without a vcs.revision setting", got)
	}
}

// Since Go 1.24, a build made inside a git checkout is stamped with a
// pseudo-version and "+dirty" instead of "(devel)", and String() alone
// cannot tell two such builds apart — which is why the fingerprint gates on
// the vcs.revision setting rather than on what String() reports. This is the
// case the fingerprint exists for: a real build in a real checkout.
func TestBuildFingerprintIsSetWithVCSProvenance(t *testing.T) {
	old := injected
	injected = ""
	t.Cleanup(func() { injected = old })

	vcsInfo := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "deadbeef"}}}
	if got := buildFingerprint(vcsInfo); got == "" {
		t.Error("buildFingerprint() = \"\", want a non-empty value with a vcs.revision setting")
	}
}

// Two builds are the same build only if nothing has been rebuilt since; the
// fingerprint exists to say so even though String() cannot.
func TestBuildFingerprintIsStableAcrossCalls(t *testing.T) {
	old := injected
	injected = ""
	t.Cleanup(func() { injected = old })

	vcsInfo := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "deadbeef"}}}
	first := buildFingerprint(vcsInfo)
	second := buildFingerprint(vcsInfo)
	if first != second {
		t.Errorf("buildFingerprint() = %q then %q, want the same value without a rebuild in between", first, second)
	}
}
