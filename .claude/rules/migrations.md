## Database Migrations

Schema changes use `PRAGMA user_version` with sequential migration functions in `migrations.go`.

### Adding a new migration

1. Write a function `migrateVN_Description(d *DB) error` in `migrations.go`
2. Append it to the `migrations` slice — order matters, never reorder or remove entries
3. Make the function idempotent (use `IF NOT EXISTS`, check `pragma_table_info`, etc.)
4. Return errors with context via `fmt.Errorf`

### Rules

- Migrations run sequentially, fail fast — no transactions wrapping individual migrations
- Each migration runs exactly once, tracked by `PRAGMA user_version`
- V1 is the initial schema; it uses `IF NOT EXISTS` to handle both fresh and pre-versioned databases
- Never modify an existing migration — always add a new one
- SQLite limitations: no `ALTER COLUMN`, no `ADD COLUMN IF NOT EXISTS` — check column existence via `pragma_table_info('table')` before `ALTER TABLE`
- `DROP COLUMN` is supported (SQLite 3.35.0+, works with modernc.org/sqlite)
- Always create indexes on new columns in the same migration that adds the column
