# Contributing

## Development setup

- Go 1.26 or newer
- Docker Desktop with WSL integration for container checks

## Before opening a pull request

Run these from the repository root:

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go test -coverprofile=coverage.out ./...
docker build -t forgedb:local .
```

Keep changes focused, add tests for behavior changes, and update the documentation when SQL or storage behavior changes.
