package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State is the cached current ad plus its display bookkeeping, stored at
// $VIBEPERKS_HOME/state.json. Render timestamps double as the "the ad was actually on
// screen" signal used to compute displayed_ms; Recorded prevents double-counting an
// impression across the prompt and stop hooks. NeedsLogin is set when the device token
// was rejected (401/403) so the surface shows a sign-in notice instead of an ad.
type State struct {
	Ad            *Ad   `json:"ad"`
	ServedAt      int64 `json:"served_at"`
	FirstRenderAt int64 `json:"first_render_at"`
	LastRenderAt  int64 `json:"last_render_at"`
	Recorded      bool  `json:"recorded"`
	RotateCount   int   `json:"rotate_count"`
	NeedsLogin    bool  `json:"needs_login,omitempty"`
	// NeedsLoginReason is the user-facing reason the token was rejected (e.g. "device
	// token invalid or revoked"), shown alongside the sign-in prompt.
	NeedsLoginReason string `json:"needs_login_reason,omitempty"`
}

func statePath(dir string) string { return filepath.Join(dir, "state.json") }

// LoadState reads the cached state; a missing file yields the zero State (no ad).
func LoadState(dir string) (State, error) {
	var s State
	b, err := os.ReadFile(statePath(dir))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	return s, nil
}

// SaveState writes the cached state atomically with locked-down permissions.
func SaveState(dir string, s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(statePath(dir), b, 0o600)
}
