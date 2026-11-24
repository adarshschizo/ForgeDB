# SQL Subset

Supported statements:

- `CREATE TABLE name (column TYPE, ...)`
- `INSERT INTO name VALUES (value, ...)`
- `SELECT * | columns FROM name [WHERE column operator value]`
- `UPDATE name SET column = value [WHERE column operator value]`
- `DELETE FROM name [WHERE column operator value]`
- `DROP TABLE name`
- `BEGIN`, `COMMIT`, `ROLLBACK`

Types: `INTEGER`, `TEXT`, `BOOLEAN`, `FLOAT`.

Comparison operators: `=`, `!=`, `<`, `<=`, `>`, `>=`.

Compound WHERE expressions support `AND` and `OR`, for example:
`WHERE id >= 10 AND active = true`.
