package core

import (
	"fmt"
	"os"
)

// Guard runs fn at the outermost plugin boundary, swallowing any returned error or
// panic so a plugin failure can never break or slow the host CLI. This is the ONLY
// place in the codebase where errors are intentionally swallowed. Set $VIBEPERKS_DEBUG to
// surface what was swallowed on stderr.
func Guard(fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			debug("panic: %v", r)
		}
	}()
	if err := fn(); err != nil {
		debug("error: %v", err)
	}
}

func debug(format string, args ...any) {
	if os.Getenv("VIBEPERKS_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "vibeperks: "+format+"\n", args...)
}
