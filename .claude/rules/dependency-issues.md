## Dependency & Vulnerability Issues

Weekly CI creates GitHub issues for dependency health:

- **`vulncheck` label**: govulncheck found vulnerabilities in code or dependencies
- **`dependencies` label**: outdated Go modules with available updates

### Working vulncheck issues

1. Read the issue body for the govulncheck output
2. Determine if vulns are in direct deps (update them) or stdlib (update go.mod go version)
3. Run `go get <module>@latest` for affected deps, then `go mod tidy`
4. Run `govulncheck ./...` locally to verify the fix
5. If a vuln has no fix available, note that in the issue and close it

### Working dependency update issues

1. Read the issue body — it has a table of current vs available versions and raw JSON
2. Update direct deps first: `go get <module>@<version>` for each
3. Run `go mod tidy` to update indirect deps
4. Run `go vet ./...` and `go test ./...` to verify nothing breaks
5. Check for breaking changes if a major version bumped
