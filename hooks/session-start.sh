#!/bin/bash
# SessionStart hook: injects pending tasks as context on session start/resume.
# Receives JSON on stdin with session info including cwd.

set -euo pipefail

input=$(cat)
cwd=$(echo "$input" | jq -r '.cwd // empty')

if [ -z "$cwd" ]; then
    exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="$SCRIPT_DIR/tasks-mcp"

if [ ! -x "$BINARY" ]; then
    # Try go run as fallback.
    cd "$SCRIPT_DIR"
    output=$(go run . pending --workspace "$cwd" 2>/dev/null || true)
else
    output=$("$BINARY" pending --workspace "$cwd" 2>/dev/null || true)
fi

if [ -n "$output" ]; then
    echo "$output"
fi
