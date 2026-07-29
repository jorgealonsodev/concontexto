#!/usr/bin/env bash
set -euo pipefail

# scripts/backup/pg_dump.sh — logical PostgreSQL backup via `docker compose exec`.
#
# WHY THIS SCRIPT EXISTS (PRD §14.3 consistency requirement):
# A hot file-level copy of the `pg_data` Docker volume — e.g. a VPS-level
# backup agent snapshotting the volume's files while Postgres is running —
# CAN RESTORE CORRUPT. WAL and heap files can be captured mid-write, across
# an inconsistent point in time, with no guarantee the copied files
# represent a single consistent transaction boundary. `pg_dump` instead
# takes a transactionally consistent logical snapshot (one REPEATABLE READ
# transaction), so this script MUST replace, or run alongside and take
# precedence over, whatever generic file-level backup the VPS provider
# runs — a raw `pg_data` volume copy is not a substitute for this.
#
# Usage:
#   scripts/backup/pg_dump.sh [output-directory]
#
# Restore (custom format, -F c, supports pg_restore's selective/parallel
# restore):
#   docker compose exec -T postgres pg_restore -U "$POSTGRES_USER" \
#     -d "$POSTGRES_DB" --clean --if-exists < backups/concontexto-<ts>.dump
#
# Requires: the `postgres` service running under `docker compose`, and
# POSTGRES_USER/POSTGRES_DB matching docker-compose.yml's environment
# (defaults below match that file's own defaults).

OUT_DIR="${1:-./backups}"
SERVICE="${COMPOSE_POSTGRES_SERVICE:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-concontexto}"
POSTGRES_DB="${POSTGRES_DB:-concontexto}"

mkdir -p "$OUT_DIR"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
out_file="${OUT_DIR}/concontexto-${timestamp}.dump"

echo "pg_dump: backing up '${POSTGRES_DB}' (service: ${SERVICE}) -> ${out_file}"

# Streamed straight from the container's stdout to the host file over the
# exec pipe; no intermediate dump file is ever left inside the container.
docker compose exec -T "$SERVICE" \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -F c > "$out_file"

echo "pg_dump: done ($(du -h "$out_file" | cut -f1))"
