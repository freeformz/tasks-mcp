# tasks-mcp

Task management MCP server for AI agents, built in Go.

## Project Structure

- `main.go` — Entry point, MCP server + CLI subcommands (pending, check-active)
- `server.go` — MCP server setup and tool registration
- `tools.go` — MCP tool handlers (task_create, task_list, task_get, task_update, task_delete)
- `models.go` — Task model, status/priority enums
- `db.go` — SQLite database layer (schema, CRUD, queries)
- `hooks/` — Claude Code hook scripts (session-start, on-stop)
- `.claude/rules/taskqueue.md` — Rules instructing Claude how to use the MCP
- `.claude/settings.json` — Hook configuration

## Code Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go)
- All public methods that make network calls must accept `context.Context` as first parameter
- Use `any` instead of `interface{}`
- Use standard library functions — don't reimplement stdlib
- Tests use `t.Context()` — never `context.Background()` or `context.TODO()` in tests

## Build & Test

```sh
go build -o tasks-mcp .    # build binary
go vet ./...                # lint
go test ./...               # run tests
```

## Architecture

- **Database**: SQLite (pure Go via modernc.org/sqlite), WAL mode, stored at `~/.local/share/tasks-mcp/tasks.db`
- **MCP transport**: stdio via mark3labs/mcp-go
- **Workspace isolation**: Tasks scoped by absolute project directory path
- **Multi-process safe**: SQLite WAL mode allows concurrent readers/writers
