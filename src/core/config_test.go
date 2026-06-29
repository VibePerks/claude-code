package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadConfigMissingReturnsDefault(t *testing.T) {
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIBase != defaultAPIBase {
		t.Errorf("APIBase = %q, want default %q", cfg.APIBase, defaultAPIBase)
	}
	if cfg.OptOut || cfg.DeviceToken != "" {
		t.Errorf("expected empty default config, got %+v", cfg)
	}
}

func TestLoadConfigMalformedErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("expected error for malformed config")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Config{APIBase: "https://example.test", DeviceToken: "tok", OptOut: true}
	if err := SaveConfig(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("round trip mismatch: got %+v want %+v", got, want)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Join(dir, "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("config perms = %o, want 600", fi.Mode().Perm())
		}
	}
}

func TestLoadConfigEmptyAPIBaseFallsBack(t *testing.T) {
	dir := t.TempDir()
	if err := SaveConfig(dir, Config{DeviceToken: "tok"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIBase != defaultAPIBase {
		t.Errorf("APIBase = %q, want default", got.APIBase)
	}
}

func TestDefaultAPIBaseEnvOverride(t *testing.T) {
	t.Setenv("VIBEPERKS_API", "https://override.test")
	if DefaultAPIBase() != "https://override.test" {
		t.Errorf("DefaultAPIBase = %q, want override", DefaultAPIBase())
	}
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIBase != "https://override.test" {
		t.Errorf("missing-config APIBase = %q, want override", cfg.APIBase)
	}
}

func TestConfigDirEnvOverride(t *testing.T) {
	t.Setenv("VIBEPERKS_HOME", filepath.Join("custom", "vibeperks"))
	if ConfigDir() != filepath.Join("custom", "vibeperks") {
		t.Errorf("ConfigDir = %q, want override", ConfigDir())
	}
}
