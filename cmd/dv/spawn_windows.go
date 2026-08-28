//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// detach puts the server in a process group of its own, so a console Ctrl+C
// does not take the session down with the client.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
