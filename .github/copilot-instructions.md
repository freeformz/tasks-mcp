## Code Review Instructions

### Go Conventions
- Follow Effective Go conventions
- Use `any` instead of `interface{}`
- Prefer standard library functions over reimplementations
- Public methods making network calls must accept `context.Context` as first parameter

### Testing
- Tests must use `t.Context()`, never `context.Background()` or `context.TODO()`
- Test files use a shared `testDB(t)` helper for database setup

### Database & Migrations
- SQLite via pure Go driver (modernc.org/sqlite), no CGO
- Migrations are append-only — never modify an existing migration
- New migrations must be idempotent (use `IF NOT EXISTS`, check `pragma_table_info`)
- SQLite limitations: no `DROP COLUMN`, no `ALTER COLUMN`, no `ADD COLUMN IF NOT EXISTS`
- All queries must use parameterized arguments — no string interpolation of user input

### Security
- Check for SQL injection in database queries
- Check for command injection in any shell interactions
- Validate all inputs at system boundaries (MCP tool handlers, CLI flags)

### Architecture
- MCP tool handlers return `(*mcp.CallToolResult, error)` — ensure errors include context
- CLI commands use Cobra; hook subcommands must set `Hidden`, `SilenceUsage`, `SilenceErrors`
- Workspace resolution uses `resolveWorkspace()`, dependency checks use `validateDependencies()`
- Progress notes use `formatProgressNote()` and `appendProgressNote()` helpers
