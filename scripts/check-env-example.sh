#!/usr/bin/env bash
set -euo pipefail

# scripts/check-env-example.sh — fail the build when env.example drifts.
#
# WHY THIS EXISTS:
# Deployment wiring is an integration surface with no unit-test coverage.
# During slice 2 the `migrate` subcommand started reading DATABASE_URL while
# docker-compose.yml exported only POSTGRES_ADDR. Every Go test passed. The
# defect would have surfaced in production as migrations silently never
# running on the VPS.
#
# This script closes that gap for the documentation half: every environment
# variable the Go source reads must be documented in env.example. It is
# intentionally simple and has no dependencies beyond grep.
#
# Usage:
#   scripts/check-env-example.sh
#
# Exit status: 0 when every variable is documented, 1 otherwise.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

env_example="env.example"

if [[ ! -f "$env_example" ]]; then
  echo "check-env-example: $env_example is missing" >&2
  exit 1
fi

# Collect the variable names read by non-test Go source. Test files are
# excluded: they may legitimately set up throwaway variables that have no
# place in a deployment reference.
mapfile -t used < <(
  grep -rlE 'os\.(Getenv|LookupEnv)' --include='*.go' . 2>/dev/null |
    grep -v '_test\.go$' |
    xargs -r grep -hoE 'os\.(Getenv|LookupEnv)\("[A-Z0-9_]+"\)' |
    sed -E 's/.*\("([A-Z0-9_]+)"\).*/\1/' |
    sort -u
)

if [[ ${#used[@]} -eq 0 ]]; then
  echo "check-env-example: no environment variables found in Go source"
  exit 0
fi

missing=()
for name in "${used[@]}"; do
  # Matches both a live assignment (NAME=) and a commented-out one
  # (# NAME=), because optional variables are documented commented out.
  if ! grep -qE "^[[:space:]]*#?[[:space:]]*${name}=" "$env_example"; then
    missing+=("$name")
  fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "check-env-example: FAIL — read by the Go source but absent from $env_example:" >&2
  for name in "${missing[@]}"; do
    echo "  - $name" >&2
    grep -rn "os\.\(Getenv\|LookupEnv\)(\"${name}\")" --include='*.go' . 2>/dev/null |
      grep -v '_test\.go:' | sed 's/^/      /' >&2 || true
  done
  echo >&2
  echo "Document each one in $env_example, in the same commit that reads it." >&2
  exit 1
fi

echo "check-env-example: OK — ${#used[@]} variable(s) documented: ${used[*]}"
