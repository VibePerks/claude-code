// Package core is the shared VibePerks plugin client: config, device-token auth, ad
// fetch + cache, impression buffering, opt-out, and the fail-silent boundary. CLI
// adapters (e.g. claude-code) are thin wrappers that hook the host lifecycle and
// delegate every network/auth/cache concern to this package.
package core

// Ad is the served creative returned by GET /v1/ads/serve.
type Ad struct {
	AdID            string `json:"ad_id"`
	Sentence        string `json:"sentence"`
	Domain          string `json:"domain"`
	ImpressionToken string `json:"impression_token"`
	RotateSeconds   int    `json:"rotate_seconds"`
}

// Impression is the payload posted to POST /v1/impressions. Money/credit is decided
// server-side; the client only reports display facts. Optional fields are omitted
// when empty so the backend treats them as absent.
type Impression struct {
	ImpressionToken   string `json:"impression_token"`
	DisplayedMs       int    `json:"displayed_ms"`
	SessionID         string `json:"session_id,omitempty"`
	SessionDurationMs int    `json:"session_duration_ms,omitempty"`
	PluginVersion     string `json:"plugin_version,omitempty"`
	CLI               string `json:"cli,omitempty"`
	CLIVersion        string `json:"cli_version,omitempty"`
}
