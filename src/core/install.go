package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func settingsPath(claudeDir string) string { return filepath.Join(claudeDir, "settings.json") }

// InstallStatusLine writes the plugin's status-line command into Claude Code's
// settings.json. If a different status line already exists, it is backed up to
// prev_statusline.json (once) so it can be restored on uninstall. Malformed settings are
// left untouched and reported as an error - the plugin never clobbers the user's config.
func InstallStatusLine(claudeDir, dataDir, command string) error {
	path := settingsPath(claudeDir)
	settings := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("claude settings.json is malformed; not modifying: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	current := ""
	if sl, ok := settings["statusLine"].(map[string]any); ok {
		current, _ = sl["command"].(string)
	}
	if current == command {
		return nil // already installed
	}
	if current != "" && !isOurs(current) {
		prev := filepath.Join(dataDir, "prev_statusline.json")
		if !fileExists(prev) {
			if b, err := json.Marshal(settings["statusLine"]); err == nil {
				if err := atomicWrite(prev, b, 0o600); err != nil {
					return err
				}
			}
		}
	}

	settings["statusLine"] = map[string]any{
		"type":            "command",
		"command":         command,
		"refreshInterval": 10,
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, b, 0o644)
}

// DeregisterStatusLine removes the plugin's status line from settings.json, restoring a
// previously backed-up status line when one exists. It only acts when the current status
// line is ours, so it never disturbs a status line the user set afterwards.
func DeregisterStatusLine(claudeDir, dataDir string) error {
	path := settingsPath(claudeDir)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return err
	}
	sl, ok := settings["statusLine"].(map[string]any)
	if !ok {
		return nil
	}
	cmd, _ := sl["command"].(string)
	if !isOurs(cmd) {
		return nil
	}
	if pb, err := os.ReadFile(filepath.Join(dataDir, "prev_statusline.json")); err == nil {
		var prev any
		if json.Unmarshal(pb, &prev) == nil && prev != nil {
			settings["statusLine"] = prev
		} else {
			delete(settings, "statusLine")
		}
	} else {
		delete(settings, "statusLine")
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, out, 0o644)
}

func isOurs(command string) bool { return strings.Contains(command, "vibeperks") }
