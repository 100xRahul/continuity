#!/bin/sh
# Runs one Continuity hook phase. The harness sends the hook JSON on stdin,
# including the session's working directory.
set -eu

harness="${1:?harness is required}"
phase="${2:?hook phase is required}"

if command -v continuity >/dev/null 2>&1; then
  exec continuity hook "$harness" "$phase"
fi

# The Continuity source repository runs its own hooks without an installed binary.
source_root="$(cd "$(dirname "$0")/../.." && pwd)"
if [ -f "$source_root/go.mod" ] && [ -d "$source_root/cmd/continuity" ] && command -v go >/dev/null 2>&1; then
  cd "$source_root"
  exec go run ./cmd/continuity hook "$harness" "$phase"
fi

echo "continuity is not installed: this session gets no project decisions or context. Install it and put it on PATH." >&2
exit 1
