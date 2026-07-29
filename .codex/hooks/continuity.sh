#!/bin/sh
set -eu

phase="${1:?hook phase is required}"
root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$root" ]; then
  exit 0
fi

if command -v continuity >/dev/null 2>&1; then
  exec continuity codex-hook "$phase" --repo "$root"
fi

# This fallback makes the source repository self-hosting during development.
if [ -f "$root/go.mod" ] && [ -d "$root/cmd/continuity" ]; then
  cd "$root"
  exec go run ./cmd/continuity codex-hook "$phase" --repo "$root"
fi

exit 0
