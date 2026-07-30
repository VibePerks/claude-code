//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// detectCols returns the current console width in columns. It queries the active console
// screen buffer directly (CONOUT$) so it works even when Claude Code captures the
// status-line command's stdout through a pipe (in which case COLUMNS is unset and stdout
// is not a TTY). Returns ok=false when no console is attached.
func detectCols() (int, bool) {
	if name, err := syscall.UTF16PtrFromString("CONOUT$"); err == nil {
		const genericRead = 0x80000000
		const genericWrite = 0x40000000
		const fileShareReadWrite = 0x1 | 0x2
		const openExisting = 3
		r, _, _ := procCreateFileW.Call(
			uintptr(unsafe.Pointer(name)),
			genericRead|genericWrite,
			fileShareReadWrite,
			0,
			openExisting,
			0,
			0,
		)
		h := syscall.Handle(r)
		if h != syscall.InvalidHandle {
			defer syscall.CloseHandle(h)
			if c, ok := consoleCols(h); ok {
				return c, true
			}
		}
	}
	// Fall back to the inherited standard handles.
	for _, h := range []syscall.Handle{syscall.Stderr, syscall.Stdout} {
		if c, ok := consoleCols(h); ok {
			return c, true
		}
	}
	return 0, false
}

type coord struct{ x, y int16 }

type smallRect struct{ left, top, right, bottom int16 }

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procCreateFileW                = kernel32.NewProc("CreateFileW")
)

// consoleCols reads the visible window width (not the buffer width) of a console handle.
func consoleCols(h syscall.Handle) (int, bool) {
	var info consoleScreenBufferInfo
	r, _, _ := procGetConsoleScreenBufferInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 0, false
	}
	w := int(info.window.right-info.window.left) + 1
	if w <= 0 {
		return 0, false
	}
	return w, true
}
