#!/bin/bash
# Stop hook: reminds Claude to update in-progress tasks before ending.
# Receives JSON on stdin with session info including cwd.
# Exit code 2 blocks the stop; stdout JSON with decision/reason.

set -euo pipefail

input=$(cat)
cwd=$(echo "$input" | jq -r '.cwd // empty')

if [ -z "$cwd" ]; then
    exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="$SCRIPT_DIR/tasks-mcp"

if [ ! -x "$BINARY" ]; then
    cd "$SCRIPT_DIR"
    output=$(go run . check-active --workspace "$cwd" 2>/dev/null || true)
else
    output=$("$BINARY" check-active --workspace "$cwd" 2>/dev/null || true)
fi

if [ -n "$output" ]; then
    echo "$output"
    exit 2
fi
