# tasks-mcp

A task management [MCP](https://modelcontextprotocol.io/) server for AI agents, built in Go. Tracks multi-step work across sessions with workspace isolation, subtasks, dependencies, and tags.

## Features

- **Workspace-aware**: Tasks are scoped to the project directory, so multiple agents in different projects don't interfere
- **Multi-process safe**: SQLite with WAL mode supports concurrent access from multiple MCP server instances
- **Persistent**: Tasks survive across sessions, enabling long-running work tracking
- **Subtasks & dependencies**: Break complex work into subtasks and define ordering constraints
- **Tags**: Categorize tasks for filtering
- **Progress notes**: Timestamped log entries track what was done and when
- **Claude Code hooks**: Automatically surfaces pending tasks on session start and reminds about in-progress tasks on stop

## MCP Tools

| Tool | Description |
|------|-------------|
| `task_create` | Create a task with title, description, priority, tags, subtasks, and dependencies |
| `task_list` | List tasks filtered by status, tag, or parent |
| `task_get` | Get full task details including subtasks and dependencies |
| `task_update` | Update fields, add/remove tags and dependencies, append progress notes |
| `task_delete` | Delete a task and its subtasks |

## Install

```sh
go install github.com/freeformz/tasks-mcp@latest
```

Or pull the Docker image:

```sh
docker pull ghcr.io/freeformz/tasks-mcp:latest
```

## Configure

Add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "tasks": {
      "type": "stdio",
      "command": "tasks-mcp",
      "args": ["mcp"]
    }
  }
}
```

### Docker

```json
{
  "mcpServers": {
    "tasks": {
      "type": "stdio",
      "command": "docker",
      "args": ["run", "--rm", "-i", "-v", "tasks-mcp-data:/data", "ghcr.io/freeformz/tasks-mcp:latest", "mcp"]
    }
  }
}
```

The database is stored at `/data/tasks.db` inside the container. The volume mount persists it across runs.

### Hooks (optional)

Add to `.claude/settings.json` for automatic task context on session start and stop reminders:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/tasks-mcp/hooks/session-start.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/tasks-mcp/hooks/on-stop.sh",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

### Rules

Copy `.claude/rules/taskqueue.md` to your project's `.claude/rules/` directory to instruct Claude when and how to use the task tools.

## Database

Tasks are stored in SQLite at `~/.local/share/tasks-mcp/tasks.db`. The database is created automatically on first run. Schema changes are managed via versioned migrations tracked with SQLite's `PRAGMA user_version`.

## License

MIT
