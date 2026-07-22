package core

import (
	"errors"
	"testing"
)

func TestGuardRunsFn(t *testing.T) {
	ran := false
	Guard(func() error {
		ran = true
		return nil
	})
	if !ran {
		t.Error("Guard should run the function")
	}
}

func TestGuardSwallowsError(t *testing.T) {
	// Must not panic or propagate.
	Guard(func() error { return errors.New("boom") })
}

func TestGuardRecoversPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Guard let a panic escape: %v", r)
		}
	}()
	Guard(func() error { panic("kaboom") })
}

func TestGuardDebugBranches(t *testing.T) {
	// With VIBEPERKS_DEBUG set, the swallowed error and panic are logged to stderr.
	t.Setenv("VIBEPERKS_DEBUG", "1")
	Guard(func() error { return errors.New("boom") })
	Guard(func() error { panic("kaboom") })
}
