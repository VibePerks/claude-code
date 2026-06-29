package main

import (
	"os"
	"runtime"
	"testing"

	"vibeperks/core"
)

func TestTerminalCols(t *testing.T) {
	t.Setenv("COLUMNS", "120")
	if terminalCols() != 120 {
		t.Errorf("COLUMNS=120 => %d", terminalCols())
	}
	t.Setenv("COLUMNS", "nope")
	if terminalCols() != 80 {
		t.Errorf("invalid COLUMNS => %d, want default 80", terminalCols())
	}
	t.Setenv("COLUMNS", "")
	if terminalCols() != 80 {
		t.Errorf("empty COLUMNS => %d, want 80", terminalCols())
	}
}

func TestShQuote(t *testing.T) {
	if got := shQuote("/usr/bin/vibeperks"); got != "'/usr/bin/vibeperks'" {
		t.Errorf("got %q", got)
	}
	if got := shQuote("/it's/here"); got != `'/it'\''s/here'` {
		t.Errorf("got %q", got)
	}
}

func TestBinName(t *testing.T) {
	got := binName()
	if runtime.GOOS == "windows" && got != "vibeperks.exe" {
		t.Errorf("windows bin = %q", got)
	}
	if runtime.GOOS != "windows" && got != "vibeperks" {
		t.Errorf("unix bin = %q", got)
	}
}

func TestMeta(t *testing.T) {
	m := meta("sess-9")
	if m.CLI != "claude-code" || m.SessionID != "sess-9" || m.PluginVersion != version {
		t.Errorf("meta = %+v", m)
	}
}

func TestDispatchUnknownReturnsNil(t *testing.T) {
	if err := dispatch("totally-unknown"); err != nil {
		t.Errorf("unknown command should be a no-op, got %v", err)
	}
}

func TestDispatchLoginStoresToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIBEPERKS_HOME", dir)
	old := os.Args
	os.Args = []string{"vibeperks", "login", "my-device-token"}
	defer func() { os.Args = old }()

	if err := dispatch("login"); err != nil {
		t.Fatal(err)
	}
	cfg, err := core.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DeviceToken != "my-device-token" {
		t.Errorf("token = %q", cfg.DeviceToken)
	}
}

func TestDispatchOptOutAndIn(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIBEPERKS_HOME", dir)
	if err := dispatch("optout"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := core.LoadConfig(dir); !cfg.OptOut {
		t.Error("optout should set OptOut")
	}
	if err := dispatch("optin"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := core.LoadConfig(dir); cfg.OptOut {
		t.Error("optin should clear OptOut")
	}
}

// A failing background command must never escape the Guard boundary into the host CLI.
func TestRefreshFailureContainedByGuard(t *testing.T) {
	t.Setenv("VIBEPERKS_HOME", t.TempDir())
	t.Setenv("VIBEPERKS_API", "http://127.0.0.1:1") // connection refused
	core.Guard(func() error { return dispatch("refresh") })
}
