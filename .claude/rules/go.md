---
paths:
  - "**/*.go"
---

## Go Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Always check and handle errors — never ignore returned errors
- Wrap errors with `%w` (e.g., `fmt.Errorf("context: %w", err)`) to preserve the error chain
- All public methods that make network calls must accept `context.Context` as first parameter
- Use `any` instead of `interface{}`
- Use standard library functions — don't reimplement stdlib
- Tests use `t.Context()` — never `context.Background()` or `context.TODO()` in tests
- Use sentinel errors (e.g., `ErrTaskNotFound`) with `errors.Is` for control flow — don't match error strings
- Check `sql.ErrNoRows` (via `errors.Is`) before falling back to alternative lookups
- Escape SQLite LIKE wildcards (`%`, `_`) with `escapeLikePattern()` when building suffix/pattern queries
