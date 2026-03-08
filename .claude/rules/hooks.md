## Claude Code Hooks

Hook scripts live in `hooks/` and are configured in `.claude/settings.json`.

### Hook output JSON schema

When a hook outputs JSON to stdout, the `decision` field must be `"approve"` or `"block"` — not `"allow"` or other values. Invalid values cause a JSON validation error in Claude Code.

### Stop hook (`hooks/on-stop.sh`)

- Uses `stop_hook_active` from stdin JSON to detect re-fires and break infinite loops
- On re-fire (`stop_hook_active: true`): emit `{"decision":"approve"}` and exit 0
- On first fire with active tasks: emit check-active JSON output and exit 2 to block stopping
- The stop hook fires after **every** response, not only when the session ends
