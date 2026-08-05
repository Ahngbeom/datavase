#!/bin/sh
#
# Install dv, the terminal MySQL/MariaDB client.
#
#	curl -fsSL https://raw.githubusercontent.com/Ahngbeom/datavase/main/install.sh | sh
#
# Downloading with curl rather than a browser is not a convenience here: a file
# a browser fetched carries com.apple.quarantine, and macOS refuses to run an
# unnotarized binary that has it. Nothing downloaded by this script is ever
# quarantined, so `dv` runs the moment it lands.
#
# Knobs, all optional:
#
#	DV_VERSION       tag to install          (default: the latest release)
#	DV_INSTALL_DIR   where to put the binary (default: the first writable of
#	                 /usr/local/bin, ~/.local/bin)
#	DV_SKIP_CHECKSUM set to 1 to install without verifying the download
set -eu

REPO="Ahngbeom/datavase"
BINARY="dv"

say() { printf '%s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------- downloading

if command -v curl >/dev/null 2>&1; then
	HAVE=curl
elif command -v wget >/dev/null 2>&1; then
	HAVE=wget
else
	die "need curl or wget"
fi

# fetch writes a URL to a file, failing on any HTTP error rather than saving
# the error page and letting tar report the confusion instead.
fetch() {
	case "$HAVE" in
	curl) curl -fsSL "$1" -o "$2" ;;
	wget) wget -qO "$2" "$1" ;;
	esac
}

# latest_tag resolves the release tag without calling the API, which is rate
# limited to 60 requests an hour per address — shared by everyone behind one
# office NAT. The /latest page redirects to the tag, so the resolved URL is the
# answer.
latest_tag() {
	case "$HAVE" in
	curl)
		curl -fsSL -o /dev/null -w '%{url_effective}' \
			"https://github.com/$REPO/releases/latest" 2>/dev/null |
			sed 's|.*/tag/||'
		;;
	wget)
		wget -qO- "https://api.github.com/repos/$REPO/releases/latest" |
			sed -n 's/.*"tag_name" *: *"\([^"]*\)".*/\1/p' | head -1
		;;
	esac
}

# ------------------------------------------------------------------- platform

os=$(uname -s)
case "$os" in
Darwin) os=darwin ;;
Linux) os=linux ;;
MINGW* | MSYS* | CYGWIN*)
	die "Windows is not installed by this script — take the .zip from
              https://github.com/$REPO/releases/latest"
	;;
*) die "unsupported operating system: $os" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "unsupported architecture: $arch" ;;
esac

# --------------------------------------------------------------------- version

tag="${DV_VERSION-}"
if [ -z "$tag" ]; then
	tag=$(latest_tag) || true
	[ -n "$tag" ] || die "could not work out the latest version — set DV_VERSION"
fi
# Tolerate "0.5.0" for a tag that is really "v0.5.0", since the version the
# release notes show and the version someone types are not always the same.
case "$tag" in v*) ;; *) tag="v$tag" ;; esac

archive="${BINARY}_${tag}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

# ------------------------------------------------------------------ where to

dir="${DV_INSTALL_DIR-}"
if [ -z "$dir" ]; then
	# Writability, not root: on a machine where /usr/local/bin needs sudo, an
	# install into the home directory is the one that can actually go ahead.
	if [ -w /usr/local/bin ]; then
		dir=/usr/local/bin
	else
		dir="$HOME/.local/bin"
	fi
fi

# -------------------------------------------------------------------- install

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "Downloading $BINARY $tag ($os/$arch)"
fetch "$base/$archive" "$tmp/$archive" ||
	die "no release asset $archive — check that $tag exists and builds for $os/$arch"

if [ "${DV_SKIP_CHECKSUM-}" != "1" ]; then
	if command -v sha256sum >/dev/null 2>&1; then
		sum=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
	elif command -v shasum >/dev/null 2>&1; then
		sum=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
	else
		die "no sha256sum or shasum to verify the download with;
              set DV_SKIP_CHECKSUM=1 to install without verifying"
	fi

	fetch "$base/checksums.txt" "$tmp/checksums.txt" ||
		die "could not fetch checksums.txt for $tag"

	want=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
	[ -n "$want" ] || die "checksums.txt for $tag does not list $archive"
	[ "$sum" = "$want" ] || die "checksum mismatch for $archive
              expected $want
              got      $sum"
fi

tar -xzf "$tmp/$archive" -C "$tmp" "$BINARY" ||
	die "could not extract $BINARY from $archive"

mkdir -p "$dir" || die "could not create $dir"

# install(1) is not on every minimal image; cp and chmod are. The staging name
# is cleaned up by the same trap as the download, so a failure between here and
# the rename does not leave a stray file next to the real binary.
staged="$dir/$BINARY.new.$$"
trap 'rm -rf "$tmp" "$staged"' EXIT INT TERM

cp "$tmp/$BINARY" "$staged" ||
	die "could not write to $dir — set DV_INSTALL_DIR to somewhere you own"
chmod 755 "$staged" || die "could not make $staged executable"

# Replacing by rename rather than writing in place, so a dv that is running
# right now is never the half-written file.
mv "$staged" "$dir/$BINARY" || die "could not replace $dir/$BINARY"

say "Installed $dir/$BINARY"

# ----------------------------------------------------------------- next steps

case ":$PATH:" in
*":$dir:"*)
	say ""
	say "Run: $BINARY init"
	;;
*)
	say ""
	say "$dir is not on your PATH. Add it:"
	say ""
	say "    export PATH=\"$dir:\$PATH\""
	say ""
	say "Then run: $BINARY init"
	;;
esac
