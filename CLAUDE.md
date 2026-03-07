# tasks-mcp

Task management MCP server for AI agents, built in Go.

## Project Structure

- `main.go` — Entry point, MCP server + CLI subcommands (pending, check-active)
- `server.go` — MCP server setup and tool registration
- `tools.go` — MCP tool handlers (task_create, task_list, task_get, task_update, task_delete)
- `models.go` — Task model, status/priority enums
- `db.go` — SQLite database layer (CRUD, queries)
- `migrations.go` — Versioned schema migrations (PRAGMA user_version)
- `hooks/` — Claude Code hook scripts (session-start, on-stop)
- `.claude/rules/taskqueue.md` — Rules instructing Claude how to use the MCP
- `.claude/settings.json` — Hook configuration
- `.goreleaser.yml` — GoReleaser config (builds, archives, MCPB bundles)
- `Makefile` — Build, test, and release tasks
- `.github/workflows/` — CI (test on push/PR) and Release (on semver tags)

## Code Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go)
- All public methods that make network calls must accept `context.Context` as first parameter
- Use `any` instead of `interface{}`
- Use standard library functions — don't reimplement stdlib
- Tests use `t.Context()` — never `context.Background()` or `context.TODO()` in tests

## Build & Test

```sh
make build              # build binary
make test               # run tests
make test-coverage      # tests with coverage report
make vet                # go vet
make lint               # vet + test
make release-snapshot   # local goreleaser build (no publish)
```

## Releases

- Uses GoReleaser with MCPB bundle support
- Triggered by pushing a semver tag: `git tag v0.1.0 && git push origin v0.1.0`
- Tags must follow Go module versioning: `v{MAJOR}.{MINOR}.{PATCH}`
- CI runs on push to main and PRs; release workflow runs on `v*` tags
- Version is injected into the binary via ldflags (`main.version`)

## Architecture

- **Database**: SQLite (pure Go via modernc.org/sqlite), WAL mode, stored at `~/.local/share/tasks-mcp/tasks.db`
- **MCP transport**: stdio via mark3labs/mcp-go
- **Workspace isolation**: Tasks scoped by absolute project directory path
- **Multi-process safe**: SQLite WAL mode allows concurrent readers/writers
