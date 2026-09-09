// Package version reports which build of datavase is running.
//
// A bug report that says "the latest one" is a bug report nobody can act on,
// so this answers as precisely as the build allows, from whichever source
// actually knows.
package version

import (
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
