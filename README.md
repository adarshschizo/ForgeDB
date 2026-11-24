# ForgeDB

ForgeDB is an experimental embedded relational database engine written in Go. It is an educational systems project,
not a production database.

## Features

- 4 KiB checksummed pages and a persistent database file
- Record encoding, table catalog, and basic CRUD
- SQL lexer/parser for `CREATE TABLE`, `INSERT`, `SELECT`, `UPDATE`, `DELETE`, and `DROP TABLE`
- Comparisons and compound `AND`/`OR` filters
- Explicit `BEGIN`, `COMMIT`, and `ROLLBACK`
- Write-ahead logging and committed-page replay on reopen
- Standalone persistent B+ tree package and LRU buffer-pool package
- Interactive SQL shell with `.tables`, `.schema`, `.stats`, `.history`, and `.read`
- Docker image and Compose configuration

## Requirements

- Go 1.26 or newer
- Docker Desktop with WSL integration for container use (optional)

## Build and run

```bash
go build -o bin/forgedb ./cmd/forgedb
./bin/forgedb create app.db
./bin/forgedb shell app.db
```

Or build and launch in one step:

```bash
go run ./cmd/forgedb app.db
```

Example:

```sql
CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users VALUES (1, 'Ada', true);
SELECT name FROM users WHERE id >= 1 AND active = true;
EXPLAIN SELECT * FROM users WHERE id = 1;
```

In the shell, enter `.help` to see the available dot commands and `exit` or `.exit` to leave.

## Current limitations

- A table's rows currently fit in one page; larger tables return an explicit size error.
- The B+ tree package is not yet used by the SQL planner; `EXPLAIN` reports a sequential scan.
- SQL intentionally supports a small subset and does not provide full SQL compatibility.
- Transactions and recovery are prototype implementations and have not received production-grade crash/isolation validation.
- Do not use ForgeDB for important or production data.

See [the SQL subset](docs/sql-spec.md), [storage format](docs/storage-format.md), and
[architecture notes](docs/architecture.md) for implementation details.

## Development and verification

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go test -coverprofile=coverage.out ./...
go test -bench=. -benchmem ./benchmarks
```

Convenience targets are available in the Makefile, including `make docker-smoke`. GitHub Actions runs formatting,
tests, race detection, vet, coverage, and a Docker build on pushes and pull requests.

## Docker

```bash
docker build -t forgedb:local .
docker run --rm -it -v "$PWD/data:/data" forgedb:local shell /data/app.db
```

Or use Compose:

```bash
docker compose up
```

Database files created in `data/` are local and ignored by Git.

## License

No license has been selected for this repository yet. Add a license before publishing if you want to grant others
permission to use, modify, or redistribute the project.
