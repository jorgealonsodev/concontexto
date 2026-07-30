#!/usr/bin/env bash
set -euo pipefail

# scripts/smoke-test.sh — container smoke test for milestone 0.1.
#
# WHY THIS SCRIPT EXISTS:
# tasks.md task 1.21 was marked complete after a human ran this exact
# sequence by hand, once, in a PR review session — it was never automated.
# TestFase0ClosureGate's 0.1 subtest later found that .github/workflows/ci.yml
# has no docker/compose/curl step and logged the gap without failing the
# build. This script is the fix: a real, runnable, CI-wireable smoke test
# that proves what tasks 1.14-1.19 each individually claimed was "proven by
# 1.21's smoke test" but never actually was.
#
# What it proves (PRD §14.3 / design.md "Deployment"):
#   - the image builds from the repository-root context (ADR-1's //go:embed
#     constraint);
#   - the compose stack starts and postgres reports healthy;
#   - neither service publishes a port to the host;
#   - both services carry their configured hard memory limits;
#   - /healthz and / are reachable over the compose network and return the
#     expected status/body;
#   - the shell-free `healthcheck` subcommand works inside the distroless
#     image, in both shallow and --deep form, and a Postgres outage fails
#     only --deep (the app is served statically — a DB outage must not
#     kill the app container);
#   - the image's own exec-form HEALTHCHECK instruction converges to
#     "healthy" (proof the distroless image needs no shell/curl for it);
#   - json-file log rotation is configured on both services;
#   - a pg_dump -> drop -> pg_restore round trip is possible
#     (scripts/backup/pg_dump.sh), proving PRD §14.3's consistency
#     requirement is actually satisfiable.
#
# Usage:
#   ./scripts/smoke-test.sh
#
# Requires: Docker + Docker Compose v2 (`docker compose`), run locally by a
# developer or in CI. Uses a distinct COMPOSE_PROJECT_NAME so it never
# collides with a real local/production stack, and tears down everything it
# creates (including the "proxy" network, if this run created it) on exit —
# the script is re-runnable and leaves nothing behind, success or failure.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

export COMPOSE_PROJECT_NAME="concontexto-smoke"
export POSTGRES_PASSWORD="smoke-test-$(date +%s)"
export POSTGRES_DB="${POSTGRES_DB:-concontexto}"
export POSTGRES_USER="${POSTGRES_USER:-concontexto}"

# Pin the compose file set explicitly. `docker compose` merges
# docker-compose.override.yml automatically when one is present, and the
# committed docker-compose.override.yml.example exists precisely so a
# developer can publish 127.0.0.1:8080 locally. Step 5 below asserts that
# NEITHER service publishes a host port, so a developer with that override in
# place would fail this script against their own machine rather than against
# the topology it is meant to verify. Naming the file makes this run test the
# committed production topology, whoever runs it.
export COMPOSE_FILE="docker-compose.yml"

# The scheduler runs in-process inside `serve` and, since its first tick was
# made immediate, would start fetching from INE, Eurostat and Seguridad Social
# the moment this stack comes up -- on every push, from CI. This script
# verifies the CONTAINER topology (health, ports, restart, backup/restore);
# none of its assertions need real source data, and a smoke test that hammers
# three public statistical agencies on every commit is neither hermetic nor
# neighbourly. The pipeline's own end-to-end proof lives in
# `.github/workflows/ingest-export-build.yml`, which exercises it deliberately.
export APP_SCHEDULE_DISABLED="true"

CURL_IMAGE="curlimages/curl:8.11.1"
PROXY_NETWORK_CREATED=0
SMOKE_BACKUP_DIR=""

step() {
  echo
  echo "==> $*"
}

fail() {
  echo "SMOKE TEST FAILED: $*" >&2
  exit 1
}

cleanup() {
  local status=$?
  if [[ -n "$SMOKE_BACKUP_DIR" ]]; then
    rm -rf "$SMOKE_BACKUP_DIR"
  fi
  step "Teardown: docker compose down -v"
  docker compose down -v --remove-orphans >/dev/null 2>&1 || true
  if [[ "$PROXY_NETWORK_CREATED" == "1" ]]; then
    step "Teardown: removing the 'proxy' network created by this run"
    docker network rm proxy >/dev/null 2>&1 || true
  fi
  if [[ $status -eq 0 ]]; then
    echo
    echo "SMOKE TEST: ALL CHECKS PASSED"
  fi
  exit $status
}
trap cleanup EXIT

container_id() {
  docker compose ps -q "$1"
}

wait_for_healthy() {
  local name="$1" cid="$2" timeout="${3:-120}"
  local waited=0
  while true; do
    local status
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' "$cid")"
    if [[ "$status" == "healthy" ]]; then
      echo "  $name is healthy (waited ${waited}s)"
      return 0
    fi
    if [[ "$status" == "unhealthy" ]]; then
      docker logs "$cid" 2>&1 | tail -40
      fail "$name reported unhealthy after ${waited}s"
    fi
    if [[ $waited -ge $timeout ]]; then
      docker logs "$cid" 2>&1 | tail -40
      fail "$name did not become healthy within ${timeout}s (last status: $status)"
    fi
    sleep 2
    waited=$((waited + 2))
  done
}

curl_via_proxy_network() {
  # Runs curl in a throwaway container on the compose-managed "app" alias
  # of the external "proxy" network — the same network Nginx Proxy Manager
  # would use in production, and the only network app is reachable on
  # since it publishes no ports to the host.
  docker run --rm --network proxy "$CURL_IMAGE" "$@"
}

step "0. Preconditions: build a distinct project so this never collides with a real stack"
echo "  COMPOSE_PROJECT_NAME=$COMPOSE_PROJECT_NAME"

if ! docker network inspect proxy >/dev/null 2>&1; then
  step "1. Creating the external 'proxy' network"
  docker network create proxy >/dev/null
  PROXY_NETWORK_CREATED=1
else
  echo "  'proxy' network already exists — reusing it, will not remove it on teardown"
fi

step "2. Building the image from the repository-root context"
docker compose build app

step "3. Starting the compose stack (POSTGRES_PASSWORD supplied)"
docker compose up -d

postgres_cid="$(container_id postgres)"
app_cid="$(container_id app)"
[[ -n "$postgres_cid" ]] || fail "postgres container did not start"
[[ -n "$app_cid" ]] || fail "app container did not start"

step "4. Waiting for postgres to report healthy"
wait_for_healthy "postgres" "$postgres_cid" 90

step "5. Asserting no published ports on either service"
for svc in app postgres; do
  cid="$(container_id "$svc")"
  ports="$(docker inspect --format '{{json .NetworkSettings.Ports}}' "$cid")"
  # Every entry in the Ports map must be null (no host binding); an empty
  # map ({}) is also acceptable (no exposed ports at all). A bound port
  # appears as a "HostPort" key inside that entry's array.
  if echo "$ports" | grep -q "HostPort"; then
    fail "$svc publishes a port to the host: $ports"
  fi
  echo "  $svc: NetworkSettings.Ports = $ports (no host binding) OK"
done

step "6. Asserting hard memory limits"
app_mem="$(docker inspect --format '{{.HostConfig.Memory}}' "$app_cid")"
postgres_mem="$(docker inspect --format '{{.HostConfig.Memory}}' "$postgres_cid")"
[[ "$app_mem" == "268435456" ]] || fail "app memory limit is $app_mem, expected 268435456 (256 MiB)"
[[ "$postgres_mem" == "536870912" ]] || fail "postgres memory limit is $postgres_mem, expected 536870912 (512 MiB)"
echo "  app: ${app_mem} bytes (256 MiB) OK"
echo "  postgres: ${postgres_mem} bytes (512 MiB) OK"

step "7. Asserting json-file log rotation config on both services"
for svc in app postgres; do
  cid="$(container_id "$svc")"
  logcfg="$(docker inspect --format '{{json .HostConfig.LogConfig}}' "$cid")"
  echo "$logcfg" | grep -q '"Type":"json-file"' || fail "$svc log driver is not json-file: $logcfg"
  echo "$logcfg" | grep -q '"max-size":"10m"' || fail "$svc log max-size is not 10m: $logcfg"
  echo "$logcfg" | grep -q '"max-file":"3"' || fail "$svc log max-file is not 3: $logcfg"
  echo "$logcfg" | grep -q '"compress":"true"' || fail "$svc log compress is not true: $logcfg"
  echo "  $svc: json-file, max-size=10m, max-file=3, compress=true OK"
done

step "8. GET /healthz over the compose network expects HTTP 200 and body 'ok'"
healthz_body="$(curl_via_proxy_network -fsS http://app:8080/healthz)"
[[ "$healthz_body" == "ok" ]] || fail "/healthz body was '$healthz_body', expected 'ok'"
echo "  /healthz -> 200 'ok' OK"

step "9. GET / expects HTTP 200 and the Astro hello-world HTML"
root_body="$(curl_via_proxy_network -fsS http://app:8080/)"
echo "$root_body" | grep -q "ConContexto" || fail "/ response did not contain the expected hello-world content"
echo "  / -> 200, contains 'ConContexto' OK"

step "10. docker exec healthcheck (shallow) inside the app container"
docker exec "$app_cid" /concontexto healthcheck
echo "  healthcheck (shallow) exited 0 OK"

step "11. docker exec healthcheck --deep while postgres is up"
docker exec "$app_cid" /concontexto healthcheck --deep
echo "  healthcheck --deep exited 0 while postgres is up OK"

step "12. Stopping postgres: --deep must fail, shallow must still pass"
docker compose stop postgres >/dev/null
if docker exec "$app_cid" /concontexto healthcheck --deep; then
  fail "healthcheck --deep exited 0 while postgres was stopped (expected failure)"
fi
echo "  healthcheck --deep exited non-zero while postgres is down OK"
docker exec "$app_cid" /concontexto healthcheck
echo "  healthcheck (shallow) still exited 0 while postgres is down OK (DB outage must not kill a static site)"

step "13. Restarting postgres for the backup round trip"
docker compose start postgres >/dev/null
postgres_cid="$(container_id postgres)"
wait_for_healthy "postgres" "$postgres_cid" 90

step "14. Waiting for the app image's own exec-form HEALTHCHECK to report healthy"
wait_for_healthy "app" "$app_cid" 120

step "15. pg_dump -> drop -> pg_restore round trip (scripts/backup/pg_dump.sh)"
SMOKE_BACKUP_DIR="$(mktemp -d)"

docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
  "CREATE TABLE smoke_marker (id int PRIMARY KEY, note text); INSERT INTO smoke_marker VALUES (1, 'round-trip');" \
  >/dev/null

./scripts/backup/pg_dump.sh "$SMOKE_BACKUP_DIR"
dump_file="$(ls -1t "$SMOKE_BACKUP_DIR"/concontexto-*.dump | head -1)"
[[ -n "$dump_file" ]] || fail "pg_dump.sh produced no dump file"

docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
  "DROP TABLE smoke_marker;" >/dev/null

docker compose exec -T postgres pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists \
  < "$dump_file" >/dev/null

restored="$(docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc \
  "SELECT note FROM smoke_marker WHERE id = 1;")"
restored="$(echo "$restored" | tr -d '[:space:]')"
[[ "$restored" == "round-trip" ]] || fail "pg_restore did not bring back the marker row (got '$restored')"
echo "  pg_dump -> drop -> pg_restore round trip OK (dump: $dump_file)"
