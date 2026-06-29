#!/bin/sh
# Build the VibePerks Claude Code adapter binary.
#
# Local/dev: builds bin/vibeperks for the host platform (what the hooks invoke).
# Release/CI: pass platform pairs to cross-compile the distribution binaries.
#
# The version is stamped from .claude-plugin/plugin.json so there is one source of truth.
set -eu

cd "$(dirname "$0")"

VERSION="v$(grep '"version"' .claude-plugin/plugin.json | head -1 \
  | sed -E 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"

LDFLAGS="-s -w -X main.version=$VERSION"

mkdir -p bin

# Host binary used by the plugin hooks during local dev and `claude` runs.
# Built as vibeperks.real so the committed bin/vibeperks launcher is never clobbered.
echo "Building bin/vibeperks.real $VERSION (host platform)"
( cd src && CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o ../bin/vibeperks.real . )

# Distribution binaries (built in CI for tagged releases).
if [ "${DIST:-0}" = "1" ]; then
  for pair in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64; do
    os=${pair%/*}; arch=${pair#*/}
    out="bin/vibeperks-$os-$arch"
    [ "$os" = "windows" ] && out="$out.exe"
    echo "  building $out"
    ( cd src && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags "$LDFLAGS" -o "../$out" . )
  done
fi

echo "Done."
