// Command vibeperks is the Claude Code adapter for VibePerks: a thin wrapper that hooks the
// host lifecycle (SessionStart, UserPromptSubmit, Stop, status line) and delegates every
// network, auth, cache, and privacy concern to package core. Every command runs inside
// core.Guard, the single boundary where errors are swallowed so the host CLI is never
// broken or slowed.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"vibeperks/core"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "version", "-v", "--version":
		fmt.Println(version)
		return
	}
	core.Guard(func() error { return dispatch(os.Args[1]) })
}

func dispatch(cmd string) error {
	dir := core.ConfigDir()
	switch cmd {
	case "setup":
		return cmdSetup(dir)
	case "status":
		return cmdStatus(dir)
	case "prompt":
		return cmdPrompt()
	case "stop":
		return cmdStop(dir)
	case "refresh":
		return cmdRefresh(dir)
	case "login":
		return cmdLogin(dir)
	case "optout":
		return cmdOptOut(dir, true)
	case "optin":
		return cmdOptOut(dir, false)
	case "uninstall":
		return core.DeregisterStatusLine(filepath.Join(core.Home(), ".claude"), dir)
	}
	return nil
}

func meta(sessionID string) core.Meta {
	return core.Meta{
		CLI:           "claude-code",
		CLIVersion:    os.Getenv("CLAUDE_CODE_VERSION"),
		PluginVersion: version,
		SessionID:     sessionID,
	}
}

// cmdSetup (SessionStart) installs the plugin's status line into the user's settings.
func cmdSetup(dir string) error {
	command := shQuote(selfBinPath()) + " status"
	return core.InstallStatusLine(filepath.Join(core.Home(), ".claude"), dir, command)
}

// cmdStatus (status line) renders the cached ad alongside the host status fields. It
// makes no network call, so the line is always instant.
func cmdStatus(dir string) error {
	raw, _ := io.ReadAll(os.Stdin)
	var in core.StatusInput
	_ = json.Unmarshal(raw, &in)
	adLine, notice, err := core.Render(dir, time.Now().Unix(), "vibeperks login")
	if err != nil {
		return err
	}
	fmt.Print(core.StatusLine(in, adLine, notice, terminalCols()))
	return nil
}

// cmdPrompt (UserPromptSubmit) signals thinking-start. It spawns a detached refresh so
// the prompt path never waits on the network, and prints nothing.
func cmdPrompt() error {
	raw, _ := io.ReadAll(os.Stdin)
	var in struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(raw, &in)
	self, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.Command(self, "refresh", in.SessionID)
	detach(c)
	return c.Start()
}

// cmdRefresh is the detached background worker: serve + cache the next ad and flush
// buffered impressions.
func cmdRefresh(dir string) error {
	sessionID := ""
	if len(os.Args) > 2 {
		sessionID = os.Args[2]
	}
	cfg, err := core.LoadConfig(dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return core.Refresh(ctx, dir, core.NewClient(cfg), meta(sessionID), time.Now().Unix(), true)
}

// cmdStop (Stop) signals thinking-end: record the current ad's impression and flush.
func cmdStop(dir string) error {
	raw, _ := io.ReadAll(os.Stdin)
	var in struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(raw, &in)
	cfg, err := core.LoadConfig(dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return core.EndSession(ctx, dir, core.NewClient(cfg), meta(in.SessionID), time.Now().Unix())
}

// cmdLogin stores the device token (from arg or stdin) in the local config.
func cmdLogin(dir string) error {
	token := ""
	if len(os.Args) > 2 {
		token = strings.TrimSpace(os.Args[2])
	}
	if token == "" {
		b, _ := io.ReadAll(os.Stdin)
		token = strings.TrimSpace(string(b))
	}
	if token == "" {
		return fmt.Errorf("login: no device token provided")
	}
	cfg, err := core.LoadConfig(dir)
	if err != nil {
		return err
	}
	cfg.DeviceToken = token
	if err := core.SaveConfig(dir, cfg); err != nil {
		return err
	}
	fmt.Println("vibeperks: device token saved.")
	return nil
}

// cmdOptOut toggles the opt-out flag; when opted out the plugin fetches and reports
// nothing.
func cmdOptOut(dir string, out bool) error {
	cfg, err := core.LoadConfig(dir)
	if err != nil {
		return err
	}
	cfg.OptOut = out
	if err := core.SaveConfig(dir, cfg); err != nil {
		return err
	}
	if out {
		fmt.Println("vibeperks: opted out. No ads will be fetched or reported.")
	} else {
		fmt.Println("vibeperks: opted back in.")
	}
	return nil
}

func selfBinPath() string {
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		return filepath.Join(root, "bin", binName())
	}
	if self, err := os.Executable(); err == nil {
		return self
	}
	return binName()
}

func binName() string {
	if runtime.GOOS == "windows" {
		return "vibeperks.exe"
	}
	return "vibeperks"
}

func terminalCols() int {
	if c := os.Getenv("COLUMNS"); c != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(c)); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

// shQuote single-quotes a path for safe use in the shell command Claude runs, so a path
// containing shell metacharacters stays inert.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
