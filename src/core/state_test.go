package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStateMissing(t *testing.T) {
	s, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if s.Ad != nil || s.ServedAt != 0 {
		t.Errorf("missing state should be zero, got %+v", s)
	}
}

func TestSaveLoadStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := State{
		Ad:            &Ad{AdID: "x", Sentence: "s", ImpressionToken: "tok", RotateSeconds: 20},
		ServedAt:      100,
		FirstRenderAt: 105,
		LastRenderAt:  110,
		Recorded:      true,
	}
	if err := SaveState(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ad == nil || *got.Ad != *want.Ad || got.ServedAt != want.ServedAt ||
		got.FirstRenderAt != want.FirstRenderAt || got.LastRenderAt != want.LastRenderAt || !got.Recorded {
		t.Errorf("round trip mismatch: got %+v", got)
	}
}

func TestLoadStateMalformed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(dir); err == nil {
		t.Fatal("expected error for malformed state")
	}
}
