package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// defaultAPIBase targets the prod backend; override with $VIBEPERKS_API.
const defaultAPIBase = "https://api.vibeperks.ai"

// Config is the local plugin configuration, stored at $VIBEPERKS_HOME/config.json
// (default ~/.vibeperks/config.json) with 0600 permissions.
type Config struct {
	APIBase     string `json:"api_base"`
	DeviceToken string `json:"device_token"`
	OptOut      bool   `json:"opt_out"`
}

// Home returns the user's home directory (empty on failure; callers handle that).
func Home() string {
	h, _ := os.UserHomeDir()
	return h
}

// ConfigDir is where the plugin stores config + cache. $VIBEPERKS_HOME overrides it.
func ConfigDir() string {
	if d := os.Getenv("VIBEPERKS_HOME"); d != "" {
		return d
	}
	return filepath.Join(Home(), ".vibeperks")
}

// DefaultAPIBase is the API base URL, overridable with $VIBEPERKS_API.
func DefaultAPIBase() string {
	if a := os.Getenv("VIBEPERKS_API"); a != "" {
		return a
	}
	return defaultAPIBase
}

func configPath(dir string) string { return filepath.Join(dir, "config.json") }

// LoadConfig reads the config file. A missing file is a normal "not yet configured"
// state and yields a default Config with no error. Malformed JSON is an error and
// propagates - the caller (the plugin boundary) decides whether to swallow it.
func LoadConfig(dir string) (Config, error) {
	cfg := Config{APIBase: DefaultAPIBase()}
	b, err := os.ReadFile(configPath(dir))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.APIBase == "" {
		cfg.APIBase = DefaultAPIBase()
	}
	return cfg, nil
}

// SaveConfig writes the config atomically with locked-down permissions.
func SaveConfig(dir string, cfg Config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(configPath(dir), b, 0o600)
}
