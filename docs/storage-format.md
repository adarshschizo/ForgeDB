# Storage Format

- Page size: 4096 bytes
- Header: page ID, page type, flags, checksum, and next-page pointer
- Page 0: fixed database header (`FORGEDB1`)
- Page 1: JSON catalog
- Table pages: encoded row sets
