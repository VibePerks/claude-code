package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readSettings(t *testing.T, claudeDir string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(settingsPath(claudeDir))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func statusCommand(t *testing.T, claudeDir string) string {
	t.Helper()
	sl, ok := readSettings(t, claudeDir)["statusLine"].(map[string]any)
	if !ok {
		t.Fatal("statusLine missing or wrong type")
	}
	cmd, _ := sl["command"].(string)
	return cmd
}

func TestInstallStatusLineFresh(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	if err := InstallStatusLine(claude, data, "'/x/vibeperks' status"); err != nil {
		t.Fatal(err)
	}
	if statusCommand(t, claude) != "'/x/vibeperks' status" {
		t.Errorf("command = %q", statusCommand(t, claude))
	}
}

func TestInstallPreservesOtherSettings(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	seed := map[string]any{"model": "sonnet", "permissions": map[string]any{"allow": []any{"Read"}}}
	b, _ := json.Marshal(seed)
	if err := os.WriteFile(settingsPath(claude), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallStatusLine(claude, data, "'/x/vibeperks' status"); err != nil {
		t.Fatal(err)
	}
	got := readSettings(t, claude)
	if got["model"] != "sonnet" {
		t.Errorf("existing settings not preserved: %+v", got)
	}
}

func TestInstallBacksUpExistingStatusLine(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	seed := map[string]any{"statusLine": map[string]any{"type": "command", "command": "mystatus.sh"}}
	b, _ := json.Marshal(seed)
	if err := os.WriteFile(settingsPath(claude), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallStatusLine(claude, data, "'/x/vibeperks' status"); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(data, "prev_statusline.json")) {
		t.Error("previous status line should be backed up")
	}
	if statusCommand(t, claude) != "'/x/vibeperks' status" {
		t.Errorf("our command not installed: %q", statusCommand(t, claude))
	}
}

func TestInstallIdempotentWhenAlreadyOurs(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	cmd := "'/x/vibeperks' status"
	if err := InstallStatusLine(claude, data, cmd); err != nil {
		t.Fatal(err)
	}
	if err := InstallStatusLine(claude, data, cmd); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(data, "prev_statusline.json")) {
		t.Error("should not back up our own status line")
	}
}

func TestInstallMalformedSettingsLeftUntouched(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	if err := os.WriteFile(settingsPath(claude), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallStatusLine(claude, data, "'/x/vibeperks' status"); err == nil {
		t.Fatal("expected error for malformed settings")
	}
	b, _ := os.ReadFile(settingsPath(claude))
	if string(b) != "{not json" {
		t.Errorf("malformed settings should be untouched, got %q", string(b))
	}
}

func TestDeregisterRestoresPrevious(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	seed := map[string]any{"statusLine": map[string]any{"type": "command", "command": "mystatus.sh"}}
	b, _ := json.Marshal(seed)
	_ = os.WriteFile(settingsPath(claude), b, 0o644)
	_ = InstallStatusLine(claude, data, "'/x/vibeperks' status")

	if err := DeregisterStatusLine(claude, data); err != nil {
		t.Fatal(err)
	}
	sl := readSettings(t, claude)["statusLine"].(map[string]any)
	if sl["command"] != "mystatus.sh" {
		t.Errorf("previous status line not restored: %+v", sl)
	}
}

func TestDeregisterRemovesWhenNoPrevious(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	_ = InstallStatusLine(claude, data, "'/x/vibeperks' status")
	if err := DeregisterStatusLine(claude, data); err != nil {
		t.Fatal(err)
	}
	if _, ok := readSettings(t, claude)["statusLine"]; ok {
		t.Error("status line should be removed when no backup exists")
	}
}

func TestDeregisterLeavesForeignStatusLine(t *testing.T) {
	claude := t.TempDir()
	data := t.TempDir()
	seed := map[string]any{"statusLine": map[string]any{"type": "command", "command": "someoneelse.sh"}}
	b, _ := json.Marshal(seed)
	_ = os.WriteFile(settingsPath(claude), b, 0o644)

	if err := DeregisterStatusLine(claude, data); err != nil {
		t.Fatal(err)
	}
	sl := readSettings(t, claude)["statusLine"].(map[string]any)
	if sl["command"] != "someoneelse.sh" {
		t.Errorf("foreign status line should be left intact: %+v", sl)
	}
}

func TestDeregisterMissingSettings(t *testing.T) {
	if err := DeregisterStatusLine(t.TempDir(), t.TempDir()); err != nil {
		t.Fatalf("missing settings should be a no-op: %v", err)
	}
}
