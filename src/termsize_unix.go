//go:build !windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// winsize mirrors the kernel struct filled by the TIOCGWINSZ ioctl.
type winsize struct {
	row    uint16
	col    uint16
	xpixel uint16
	ypixel uint16
}

// tiocgwinsz returns the TIOCGWINSZ ioctl request number, which differs across kernels.
func tiocgwinsz() uintptr {
	switch runtime.GOOS {
	case "linux", "solaris", "illumos":
		return 0x5413
	default: // darwin and the *BSDs
		return 0x40087468
	}
}

// detectCols returns the current terminal width in columns. It queries the controlling
// terminal (/dev/tty) first so it works even when Claude Code captures the status-line
// command's stdout through a pipe (in which case COLUMNS is unset and stdout is not a
// TTY), then falls back to the standard streams. Returns ok=false when no TTY is found.
func detectCols() (int, bool) {
	if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer f.Close()
		if c, ok := ioctlCols(f.Fd()); ok {
			return c, true
		}
	}
	for _, fd := range []uintptr{os.Stderr.Fd(), os.Stdout.Fd(), os.Stdin.Fd()} {
		if c, ok := ioctlCols(fd); ok {
			return c, true
		}
	}
	return 0, false
}

// ioctlCols reads the terminal column count for a file descriptor via TIOCGWINSZ.
func ioctlCols(fd uintptr) (int, bool) {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, tiocgwinsz(), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.col == 0 {
		return 0, false
	}
	return int(ws.col), true
}
