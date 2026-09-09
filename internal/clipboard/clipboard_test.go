package clipboard

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// found stands in for exec.LookPath: every name is installed.
func found(name string) (string, error) { return "/usr/bin/" + name, nil }

// missing stands in for a machine where none of the helpers are installed.
func missing(string) (string, error) { return "", exec.ErrNotFound }

// only installs one helper, which is how a Linux desktop usually looks.
func only(installed string) func(string) (string, error) {
	return func(name string) (string, error) {
		if name == installed {
			return "/usr/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
}

func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

func TestMacOSUsesPbcopy(t *testing.T) {
	h, ok := find("darwin", env(nil), found)
	if !ok || h.Name != "pbcopy" || len(h.Args) != 0 {
		t.Errorf("find(darwin) = %+v, %v; want pbcopy with no arguments", h, ok)
	}
}

func TestWaylandIsPreferredOverX11(t *testing.T) {
	h, ok := find("linux", env(map[string]string{"WAYLAND_DISPLAY": "wayland-0"}), found)
	if !ok || h.Name != "wl-copy" {
		t.Errorf("find(linux, wayland) = %+v, %v; want wl-copy", h, ok)
	}
}

func TestX11UsesWhicheverHelperIsInstalled(t *testing.T) {
	e := env(map[string]string{"DISPLAY": ":0"})

	if h, ok := find("linux", e, only("xclip")); !ok || h.Name != "xclip" {
		t.Errorf("find(linux, xclip installed) = %+v, %v; want xclip", h, ok)
	}
	if h, ok := find("linux", e, only("xsel")); !ok || h.Name != "xsel" {
		t.Errorf("find(linux, xsel installed) = %+v, %v; want xsel", h, ok)
	}
}

// A headless Linux box — a server reached over SSH, a container — has no
// display and no helper. Reporting "no helper" rather than running one that
// will fail is what lets the caller stay quiet about it.
func TestLinuxWithNoDisplayHasNoHelper(t *testing.T) {
	if h, ok := find("linux", env(nil), found); ok {
		t.Errorf("find(linux, no display) = %+v, want no helper", h)
	}
}

func TestAHelperThatIsNotInstalledIsNotOffered(t *testing.T) {
	if h, ok := find("darwin", env(nil), missing); ok {
		t.Errorf("find(darwin, nothing installed) = %+v, want no helper", h)
	}
	if h, ok := find("linux", env(map[string]string{"DISPLAY": ":0"}), missing); ok {
		t.Errorf("find(linux, nothing installed) = %+v, want no helper", h)
	}
}

func TestAnUnknownPlatformHasNoHelper(t *testing.T) {
	if h, ok := find("plan9", env(nil), found); ok {
		t.Errorf("find(plan9) = %+v, want no helper", h)
	}
}

// A session reached over SSH must not run a local helper: it would put the
// text on the clipboard of the machine being logged into, which nobody is
// sitting at, while the user's own clipboard stayed empty and said nothing.
// OSC 52 is what crosses the connection, and it is sent either way.
func TestASessionOverSSHUsesNoHelper(t *testing.T) {
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
		if !remote(env(map[string]string{key: "set"})) {
			t.Errorf("%s set was not read as a remote session", key)
		}
	}
	if remote(env(nil)) {
		t.Error("a session with no SSH variables was read as remote")
	}
}

func TestCopyRunsTheHelperAndFeedsItTheText(t *testing.T) {
	// `cat > /dev/null` accepts stdin and exits zero, which is all Copy
	// asks of a helper.
	h := Helper{Name: "cat", Args: []string{}}
	if err := run(h, "some rows"); err != nil {
		t.Errorf("run(cat) error = %v, want nil", err)
	}
}

func TestCopyReportsAHelperThatRefuses(t *testing.T) {
	h := Helper{Name: "false"}
	err := run(h, "some rows")
	if err == nil {
		t.Fatal("run(false) = nil, want an error naming the helper")
	}
	if !strings.Contains(err.Error(), "false") {
		t.Errorf("run(false) error = %v; want it to name the helper", err)
	}
}

func TestCopyReportsAHelperThatIsNotThere(t *testing.T) {
	err := run(Helper{Name: "dv-no-such-clipboard-helper"}, "text")
	if err == nil || !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("run(missing) error = %v, want one wrapping exec.ErrNotFound", err)
	}
}
