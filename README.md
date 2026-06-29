# VibePerks for Claude Code

A quiet, one-line sponsor in your Claude Code status line that earns you a little
credit while you code. One line, no popups, nothing to click - and **nothing about
your code, prompts, or files ever leaves your machine.**

```
Sonnet - context 30% - limit 10%   Fast APIs for every chain - alchemy.com
```

That's the whole thing: your model, context use, weekly limit, and one sponsor line.

## How it works

Two parts, deliberately separate so the terminal never waits on a server:

- **The status line** (`vibeperks status`) runs on every repaint. It only reads a cached
  ad file and prints - **zero network calls**, so the line is always instant.
- **Hooks** run around a prompt: `UserPromptSubmit` (thinking start) spawns a detached
  worker that fetches the next ad and reports the previous impression; `Stop` (thinking
  end) reports the displayed impression. The host CLI is never blocked.

All network, auth, caching, and privacy live in the shared client
([`src/core`](src/core)); the adapter ([`src/main.go`](src/main.go)) only hooks the
host lifecycle. Every hook runs inside a single fail-silent boundary
(`core.Guard`) - if anything goes wrong, the error is swallowed and Claude Code
proceeds normally. That boundary is the **only** place errors are swallowed.

## What leaves your machine

| Leaves your machine | Never leaves your machine |
|---|---|
| Device token (to authenticate) | Your code or file contents |
| Display facts: how long an ad was shown, CLI + plugin version | Your prompts or Claude's replies |
| | File names, paths, or repo names |

The impression payload is only: the served ad's token, how long it was displayed,
the session id, and the CLI/plugin versions. IP and country are derived server-side
and disclosed in the privacy policy.

## Install

```
/plugin marketplace add <marketplace>
/plugin install vibeperks@vibeperks
```

Then link your device once (token from the VibePerks website):

```
bin/vibeperks login <device-token>
```

On `SessionStart` the plugin installs its status line into `~/.claude/settings.json`.
If you already have a custom status line, it is backed up to
`~/.vibeperks/prev_statusline.json` and restored on uninstall.

Local state lives in `~/.vibeperks/` (override with `$VIBEPERKS_HOME`). The API base can be
overridden with `$VIBEPERKS_API`.

## Opt out

```
bin/vibeperks optout   # fetch nothing, report nothing
bin/vibeperks optin    # re-enable
```

## Build

Requires Go 1.23+. `bin/vibeperks` is a committed launcher; it auto-builds or runs a
prebuilt distribution binary, so installs work without a manual build.

```
./build.sh            # builds bin/vibeperks.real for your platform
DIST=1 ./build.sh     # also cross-compiles the distribution binaries
```

## Uninstall

```
/plugin uninstall vibeperks@vibeperks
bin/vibeperks uninstall   # restores your previous status line
```

## Commands

| Command | Hook / use | Purpose |
|---------|------------|---------|
| `setup` | SessionStart | install the status line (idempotent, backs up an existing one) |
| `status` | status line | render the cached ad + host status fields (no network) |
| `prompt` | UserPromptSubmit | thinking start -> spawn detached `refresh` |
| `refresh` | (detached) | serve + cache the next ad, flush impressions |
| `stop` | Stop | thinking end -> report the displayed impression |
| `login <token>` | manual | store the device token |
| `optout` / `optin` | manual | toggle ad fetching/reporting |
| `uninstall` | manual | restore the previous status line |

See [`context/plans/plugin/`](../../context/plans/plugin) for the full design.

## License

Source-available under the [PolyForm Shield License 1.0.0](LICENSE). You may read,
audit, and use this code, but not to build a product that competes with VibePerks.
Copyright (c) 2026 VibePerks.