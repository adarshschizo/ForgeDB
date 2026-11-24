#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
data_dir="$(mktemp -d)"
trap 'rm -rf "$data_dir"' EXIT

docker build -t forgedb:local "$repo_root"
docker run --rm -v "$data_dir:/data" forgedb:local create /data/smoke.db
output="$(
  docker run --rm -i -v "$data_dir:/data" forgedb:local shell /data/smoke.db <<'SQL'
CREATE TABLE docker_smoke (id INTEGER);
INSERT INTO docker_smoke VALUES (77);
SELECT * FROM docker_smoke;
.exit
SQL
)"

printf '%s\n' "$output"
grep -q '77' <<<"$output"
