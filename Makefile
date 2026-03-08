.PHONY: build test vet staticcheck lint clean release release-snapshot

# Build the binary
build:
	go build -o tasks-mcp .

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Run go vet
vet:
	go vet ./...

# Run staticcheck
staticcheck:
	staticcheck ./...

# Run vet + staticcheck + test
lint: vet staticcheck test

# Remove build artifacts
clean:
	rm -f tasks-mcp coverage.out
	rm -rf dist/

# GoReleaser snapshot (local build, no publish)
release-snapshot:
	goreleaser release --snapshot --clean

# GoReleaser release (requires GITHUB_TOKEN, triggered by CI on tags)
release:
	goreleaser release --clean
