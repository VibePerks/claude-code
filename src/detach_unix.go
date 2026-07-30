//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detach starts the child in its own session so it outlives the short-lived hook process.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
