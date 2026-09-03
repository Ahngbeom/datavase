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

	// The module system records the version a binary was installed from.
	// Local builds report "(devel)", which is honest and left as it is.
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}

// BuildFingerprint distinguishes one local build from another when String()
// cannot. Every development build reports "(devel)" — that is what String's
// own comment calls honest — so a developer who rebuilds and reattaches to a
// server still running the previous build passes the handshake's version
// check against their own past self. This exists to catch exactly that.
//
// It returns "" for anything but a development build: a released or
// go-installed binary already carries a real version, distinguishable by
// String() alone, and comparing mtimes across two identical release binaries
// would be a regression by itself rather than the safety net this is.
func BuildFingerprint() string {
	if String() != "(devel)" {
		return ""
	}

	// The executable's own modification time changes on every `go build`,
	// which is the one situation this needs to catch — a content hash would
	// answer the same question at a cost this does not need to pay.
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	info, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format("2006-01-02T15:04:05.000000000Z")
}
