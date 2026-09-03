// Package version reports which build of datavase is running.
//
// A bug report that says "the latest one" is a bug report nobody can act on,
// so this answers as precisely as the build allows, from whichever source
// actually knows.
package version

import (
	"os"
	"runtime/debug"
)

// injected is set at release time with:
//
//	-ldflags "-X github.com/Ahngbeom/datavase/internal/version.injected=v0.1.0"
//
// It is deliberately not the only source. A binary from `go install
// module@v0.1.0` carries no ldflags, and telling that user "dev" would be a
// lie the module proxy could have corrected.
var injected string

// String returns the version, preferring the most trustworthy source
// available.
func String() string {
	if injected != "" {
		return injected
	}

	// The module system records the version a binary was installed from — a
	// real one for `go install module@version`. A build made inside a git
	// checkout is stamped too: since Go 1.24 that is a pseudo-version with a
	// "+dirty" suffix when the tree has uncommitted changes, not "(devel)".
	// "(devel)" is what is left: a build where the toolchain had no VCS
	// information to stamp at all. All three are honest, and left as they are.
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}

// BuildFingerprint distinguishes one local build from another when String()
// cannot. A developer who rebuilds and reattaches to a server still running
// the previous build needs this: since Go 1.24, a build made inside a git
// checkout is stamped with a pseudo-version plus "+dirty", and successive
// rebuilds at the same uncommitted commit produce the identical string — the
// exact edit-rebuild-reattach loop this exists to catch, and String() alone
// cannot tell them apart.
//
// It is gated on provenance rather than on what String() reports, because
// that string is not a reliable signal of which builds need distinguishing:
// depending on toolchain version and environment, a local build can report
// "(devel)" or a "+dirty" pseudo-version for the same tree. What actually
// tells the two exempt cases apart is buildFingerprint's own check, below.
func BuildFingerprint() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return buildFingerprint(info)
}

// buildFingerprint takes build info as a parameter, rather than reading it
// directly, so a test can supply build settings a real binary in this
// sandbox may not carry without needing one that does.
func buildFingerprint(info *debug.BuildInfo) string {
	if injected != "" {
		// A goreleaser release always sets injected, and never needs telling
		// apart by anything else: two identical release binaries must not
		// refuse each other, which comparing mtimes here would risk.
		return ""
	}

	if !fromVCSCheckout(info) {
		// `go install module@version` builds from the module proxy's cache,
		// not a version-controlled tree, so it carries no vcs.* setting.
		// That build is already told apart by a real version in String();
		// fingerprinting it would compare mtimes that mean nothing.
		return ""
	}

	// The executable's own modification time changes on every `go build`,
	// which is the one situation this needs to catch — a content hash would
	// answer the same question at a cost this does not need to pay.
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	stat, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	return stat.ModTime().UTC().Format("2006-01-02T15:04:05.000000000Z")
}

// fromVCSCheckout reports whether the toolchain stamped this build with a
// VCS revision — present only for a build run inside a version-controlled
// tree (`go build`, `go run`), absent for one built from the module proxy's
// cache (`go install module@version`).
func fromVCSCheckout(info *debug.BuildInfo) bool {
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return true
		}
	}
	return false
}
