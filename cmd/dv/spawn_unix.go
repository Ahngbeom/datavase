//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detach puts the server in a session of its own, so that Ctrl+C in the
// terminal that happened to start it does not take the session down with the
// client.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
