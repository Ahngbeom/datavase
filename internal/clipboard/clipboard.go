// Package clipboard puts text on the clipboard of the machine dv is running
// on, when there is one to put it on.
//
// It exists because OSC 52 — the escape sequence that asks the terminal to do
// this, and the only route that reaches the clipboard of the machine someone
// is sitting at when dv runs over SSH — is a request the terminal is free to
// refuse. Several do by default: Ghostty asks first, iTerm2 has it off until
// the setting is found, tmux drops it without `set-clipboard on`, and
// Terminal.app has never implemented it. A copy that vanishes says nothing
// about why, so a local session also hands the text to the platform's own
// helper, which no terminal setting can veto.
//
// A session over SSH deliberately gets no helper: pbcopy on the far end
// writes to a clipboard nobody is looking at.
package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Helper is the command that puts text on this machine's clipboard.
type Helper struct {
	Name string
	Args []string
}

// Copy writes text to the local clipboard, reporting whether a helper was
// found to do it with.
//
// False and a nil error is the ordinary answer on a machine that has no
// helper — a headless server, a container, a Linux desktop without xclip.
// The caller has already sent OSC 52 by then, so there is nothing to report.
func Copy(text string) (bool, error) {
	if remote(os.Getenv) {
		return false, nil
	}

	h, ok := find(runtime.GOOS, os.Getenv, exec.LookPath)
	if !ok {
		return false, nil
	}
	return true, run(h, text)
}

// remote reports whether this session arrived over SSH.
//
// Any one of the three is enough: sshd sets SSH_CONNECTION and SSH_CLIENT for
// every session and SSH_TTY only for interactive ones, and a shell that
// forwards only some of them is common enough to plan for.
func remote(env func(string) string) bool {
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
		if env(key) != "" {
			return true
		}
	}
	return false
}

// find returns the helper this machine can copy with.
//
// Wayland is checked before X11 because a Wayland session usually also sets
// DISPLAY, for the X11 programs running under XWayland; taking DISPLAY first
// would route a Wayland desktop's copy through a compatibility layer that may
// not be there at all.
func find(goos string, env func(string) string, look func(string) (string, error)) (Helper, bool) {
	var candidates []Helper

	switch goos {
	case "darwin":
		candidates = []Helper{{Name: "pbcopy"}}
	case "windows":
		candidates = []Helper{{Name: "clip"}}
	case "linux", "freebsd", "openbsd", "netbsd":
		switch {
		case env("WAYLAND_DISPLAY") != "":
			candidates = []Helper{{Name: "wl-copy"}}
		case env("DISPLAY") != "":
			candidates = []Helper{
				{Name: "xclip", Args: []string{"-selection", "clipboard"}},
				{Name: "xsel", Args: []string{"--clipboard", "--input"}},
			}
		}
	}

	for _, h := range candidates {
		if _, err := look(h.Name); err == nil {
			return h, true
		}
	}
	return Helper{}, false
}

// run feeds text to the helper on standard input.
func run(h Helper, text string) error {
	cmd := exec.Command(h.Name, h.Args...)
	cmd.Stdin = strings.NewReader(text)

	if out, err := cmd.CombinedOutput(); err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%s: %w: %s", h.Name, err, msg)
		}
		return fmt.Errorf("%s: %w", h.Name, err)
	}
	return nil
}
