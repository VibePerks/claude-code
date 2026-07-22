//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// detach starts the child detached from the hook process's console so it can outlive it.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}
