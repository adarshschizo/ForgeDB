# ForgeDB Architecture

ForgeDB is an embedded, page-oriented database. Page 0 is the file header, page 1 is the metadata/catalog page, and subsequent pages store table data.

Components:
- `storage`: disk manager and page pager
- `record`: row serialization
- `catalog`: table and column metadata
- `sql`: lexer, parser, and AST
- `executor`: SQL execution and basic transaction state
- `cmd/forgedb`: interactive SQL shell
- `index`: persistent B+ tree node storage and range search
- `buffer`: pin-based LRU page cache
- `wal`: durable, checksummed log records and recovery helpers
- `transaction`: transaction state and table lock management

Concurrent `Execute` calls through one engine instance are serialized by its mutex. Writes use table locks and
implicit transactions when no explicit transaction is active. WAL records are flushed before dirty pages are
written.

The CLI provides table, schema, row, page, database-size, and WAL observability. SQL files can be replayed with
`.read`; query history is retained for the lifetime of the shell.

## Testing and release

Verification includes unit tests, package-level integration tests, storage corruption tests, reopen tests,
concurrent-operation tests, parser fuzz seeds, benchmarks, race detection, vet, coverage, and GitHub Actions CI.

The B+ tree is a standalone package and is not yet selected by the SQL query planner. Table storage is currently
limited to one data page per table. Transactions, isolation, and recovery remain prototype implementations.

Dockerfile and docker-compose definitions provide container packaging. The container uses `/data` as its database
working directory.
