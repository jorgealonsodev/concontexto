#!/usr/bin/env bash
set -euo pipefail

# scripts/assert-blocked-series-fails-build.sh — prove that a series a real
# validation rule BLOCKED makes `astro build` exit non-zero.
#
# WHY THIS SCRIPT EXISTS (verify-report CRITICAL-37).
# `.github/workflows/ingest-export-build.yml` runs the real chain — ingest,
# export, astro build — but it ingested with `config.ValidationConfig{}`:
# no thresholds at all, over three-period fixtures. So the one job proving
# the Go→Astro hand-off had the guard that blocks a series in production
# switched off, and nothing in the CI corpus could go red for it. That is
# the fifth time a check has been found positioned where its failure cannot
# occur.
#
# The failure it could not see is live right now:
#
#   1. `ocupados-epa` breaches `config/series/ocupados-epa.yaml`'s
#      `max_delta_abs: 1000` at 2020-Q2 — a fall of 1074.1 thousand
#      employed persons, the COVID-19 lockdown quarter (INE press release
#      of 28 July 2020). `config/reconocimientos.yaml` carries the record
#      that would resolve it, UNSIGNED, so the block stands.
#   2. A blocked run commits no observations, and
#      `app/internal/publishing/export.go` skips a series with zero
#      observations (`if len(obs) == 0 { continue }`) — the slug reaches
#      neither `series/` nor `manifest.series`, and the digest chain stays
#      complete, so the artifact is perfectly VALID and simply incomplete.
#   3. `web/src/lib/indicator/routes.ts` refuses to build five of six
#      frozen permalinks, so `astro build` exits non-zero.
#
# Step 3 is what this script asserts, against an artifact that reached step
# 2 through steps 1 and 2 for real — not through a golden fixture with a
# slug deleted out of it. `web/test/export/missing-slug-fails-build.test.ts`
# already proves the guard against a SYNTHESISED absence; what had never
# run anywhere is the chain that produces that absence from a real ingest.
#
# WHAT WOULD HAVE TO BREAK FOR THIS TO GO RED, AND CAN IT?
#   - The plausibility rule stops blocking the COVID quarter (a raised
#     `max_delta_abs`, a disabled rule, a reverted `ineIngestConfig`): the
#     artifact then carries all six and this build SUCCEEDS → red here.
#   - `publishing.Export` stops skipping zero-observation series: same.
#   - `resolveIndicatorRouteSlugs` goes back to filtering instead of
#     throwing: the build emits five pages and exits 0 → red here.
# Every one of those is an ordinary edit, and each is caught here.
#
# THE ASSERTION IS NOT INVERTIBLE BY ACCIDENT — the same three guards
# `assert-corrupt-artifact-fails-build.sh` uses:
#   - a build that SUCCEEDS on the blocked artifact fails this script;
#   - a build that fails for the WRONG reason fails this script (the output
#     must name the frozen-route guard AND the missing slug);
#   - the artifact is inspected BEFORE the build, so an export that wrote
#     nothing at all cannot masquerade as "the blocked series is absent".
#
# And a matched positive control, for the reason case 2 of
# `missing-slug-fails-build.test.ts` exists: the same `npm run build`, in
# the same shell, against the HONEST artifact, must exit 0. Without it a
# build broken for an unrelated reason — a missing dependency, a syntax
# error — would read as proof that validation blocking reaches the build.
#
# Usage:
#   scripts/assert-blocked-series-fails-build.sh <blocked-artifact-dir> <honest-artifact-dir>
#
# Both are produced by one `go test` run over ./app/internal/ingestion:
#   E2E_BLOCKED_EXPORT_DIR  → TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact
#   E2E_EXPORT_DIR          → TestEndToEndIngestExportBuild
# Neither is modified here; this script only builds from them.
#
# Exit status: 0 when the blocked artifact failed the build for the right
# reason AND the honest one built cleanly, 1 otherwise.

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <blocked-artifact-dir> <honest-artifact-dir>" >&2
  exit 2
fi

blocked_dir="$(cd "$1" && pwd)"
honest_dir="$(cd "$2" && pwd)"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# The frozen route the real pipeline cannot publish today, and the artifact
# slug backing it. Identical strings for this series — only `pib` diverges,
# to `pib-cvi` — but named separately because the guard under test has to
# cross that boundary.
blocked_route_slug="ocupados-epa"
blocked_artifact_slug="ocupados-epa"

# The other five, which MUST be present. Their presence is what makes the
# build's failure attributable to one missing slug rather than to a broken
# or empty artifact.
present_artifact_slugs=(tasa-de-paro-epa ipc-general ipc-subyacente pib-cvi poblacion-residente)

echo "=== the blocked artifact, before any build runs"

if [[ ! -f "$blocked_dir/manifest.json" ]]; then
  echo "FAIL: $blocked_dir holds no manifest.json — not an export artifact." >&2
  echo "      The Go step did not export here, so nothing below would mean anything." >&2
  exit 1
fi

if [[ -f "$blocked_dir/series/$blocked_artifact_slug.json" ]]; then
  echo "FAIL: $blocked_dir still carries series/$blocked_artifact_slug.json." >&2
  echo "      The ingest that produced it was NOT blocked, so this artifact cannot" >&2
  echo "      demonstrate anything about a blocked series reaching the build." >&2
  echo "      Check TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact first: it" >&2
  echo "      asserts the block itself and will have failed for the same cause." >&2
  exit 1
fi

for slug in "${present_artifact_slugs[@]}"; do
  if [[ ! -f "$blocked_dir/series/$slug.json" ]]; then
    echo "FAIL: $blocked_dir is missing series/$slug.json too." >&2
    echo "      Exactly ONE series must be absent. An artifact missing several (or all)" >&2
    echo "      would also fail the build below, and the failure would prove nothing" >&2
    echo "      about validation blocking." >&2
    exit 1
  fi
done
echo "exactly one frozen slug is absent ($blocked_artifact_slug); the other five are present."

# run_build <artifact-dir> <log-path>; echoes nothing, returns the build's
# exit status. `npm --prefix web` rather than a `cd`, matching
# assert-corrupt-artifact-fails-build.sh.
run_build() {
  local dir="$1" log="$2" status=0
  EXPORT_DIR="$dir" npm --prefix web run build >"$log" 2>&1 || status=$?
  return $status
}

work_root="$(mktemp -d)"
trap 'rm -rf "$work_root"' EXIT

echo
echo "=== the blocked artifact MUST fail the build"
blocked_log="$work_root/blocked-build.log"
blocked_status=0
run_build "$blocked_dir" "$blocked_log" || blocked_status=$?

if [[ $blocked_status -eq 0 ]]; then
  echo "FAIL: astro build SUCCEEDED on an artifact missing the blocked series." >&2
  echo "      A site is being shipped without /indicador/$blocked_route_slug/, which is a" >&2
  echo "      frozen permalink, and nothing objected — verify-report CRITICAL-27 all over" >&2
  echo "      again, this time reached through a REAL validation block." >&2
  echo "--- build output ---" >&2
  cat "$blocked_log" >&2
  exit 1
fi

# The failure must be the frozen-route guard's, naming the slug. Anything
# else — a Vite error, a missing module — is a non-zero exit that proves
# nothing about the route guard.
guard_origin='cannot produce all 6 frozen indicator routes'
if ! grep -qF "$guard_origin" "$blocked_log"; then
  echo "FAIL: astro build exited $blocked_status, but its output never mentions" >&2
  echo "      \"$guard_origin\" — so it broke for some OTHER reason and proves nothing" >&2
  echo "      about the frozen-route guard." >&2
  echo "--- build output ---" >&2
  cat "$blocked_log" >&2
  exit 1
fi
if ! grep -qF "\"$blocked_route_slug\" has no series in the export artifact" "$blocked_log"; then
  echo "FAIL: the build failed on the frozen-route guard, but not because of" >&2
  echo "      $blocked_route_slug. Some other slug is missing, so this run is not the" >&2
  echo "      scenario this script claims to prove." >&2
  echo "--- build output ---" >&2
  cat "$blocked_log" >&2
  exit 1
fi

echo "PASS: astro build exited $blocked_status, on the frozen-route guard, naming $blocked_route_slug."
grep -F -A4 "$guard_origin" "$blocked_log" | head -8

echo
echo "=== the honest artifact MUST still build (the control)"
honest_log="$work_root/honest-build.log"
honest_status=0
run_build "$honest_dir" "$honest_log" || honest_status=$?

if [[ $honest_status -ne 0 ]]; then
  echo "FAIL: astro build exited $honest_status on the HONEST artifact." >&2
  echo "      The build is broken independently of anything this script tests, so the" >&2
  echo "      failure above cannot be credited to the blocked series." >&2
  echo "--- build output ---" >&2
  cat "$honest_log" >&2
  exit 1
fi

for slug in tasa-de-paro-epa "$blocked_route_slug" ipc-general ipc-subyacente pib poblacion-residente; do
  if [[ ! -f "web/dist/indicador/$slug/index.html" ]]; then
    echo "FAIL: the honest build exited 0 but emitted no page for $slug." >&2
    exit 1
  fi
done

echo "PASS: the same build, same command, exits 0 on the honest artifact and emits all six routes."
echo
echo "A series blocked by a real validation rule cannot produce a green build."
