#!/usr/bin/env bash
set -euo pipefail

# scripts/assert-corrupt-artifact-fails-build.sh — prove that a CORRUPTED
# export artifact makes `astro build` exit non-zero.
#
# WHY THIS SCRIPT EXISTS:
# .github/workflows/ingest-export-build.yml proves the HAPPY path: the
# artifact the Go end-to-end test exports is the artifact `astro build`
# consumes, and the six indicator pages render from it. A green happy path
# alone cannot distinguish "the loader validated the artifact and was
# satisfied" from "the loader never looked". Only a build that FAILS on a
# deliberately broken artifact separates those two, and that half was
# unproven at the CI level until this script existed — design.md's Open
# Question for slice 4 named it explicitly.
#
# The three corruptions below are the three ways the artifact contract can
# realistically break in production, one per validation layer the loader
# owns (web/src/lib/export/loader.ts):
#
#   1. sha256 mismatch  — bytes changed in transit/at rest without the
#      manifest being reissued (a truncated upload, a partial rsync, a
#      tamper). Caught BEFORE parsing.
#   2. schema_version   — the Go writer bumped the artifact schema and the
#      loader was not upgraded in lockstep. The digest is RECOMPUTED here
#      on purpose, so the digest check passes and the exact-integer version
#      check is demonstrably what rejects it.
#   3. Zod shape        — a writer drifted and stopped emitting a required
#      field (`pageState`, whose absence would otherwise silently render
#      every series as "fresh"). Digest recomputed for the same reason.
#
# These mirror web/test/export/loader.test.ts's unit assertions on purpose.
# The unit tests prove the loader FUNCTION rejects them; this script proves
# the same rejection actually reaches `astro build`'s exit status, through
# getStaticPaths, in the real build — which no unit test can show.
#
# THE ASSERTION IS NOT INVERTIBLE BY ACCIDENT. Two guards, both required:
#   - a build that SUCCEEDS on a corrupted artifact fails this script;
#   - a build that fails for the WRONG reason also fails this script, so a
#     missing dependency or a syntax error can never be mistaken for proof
#     that the artifact validation worked.
#
# Usage:
#   scripts/assert-corrupt-artifact-fails-build.sh <honest-artifact-dir>
#
# <honest-artifact-dir> must be an artifact that BUILDS CLEANLY — normally
# the one TestEndToEndIngestExportBuild just exported via E2E_EXPORT_DIR.
# It is never modified: every scenario runs against its own throwaway copy.
#
# Exit status: 0 when all three corruptions failed the build for the right
# reason, 1 otherwise.

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <honest-artifact-dir>" >&2
  exit 2
fi

honest_dir="$(cd "$1" && pwd)"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [[ ! -f "$honest_dir/manifest.json" ]]; then
  echo "error: $honest_dir holds no manifest.json — not an export artifact" >&2
  exit 2
fi

work_root="$(mktemp -d)"
trap 'rm -rf "$work_root"' EXIT

# The slug every scenario corrupts. Any of the six frozen slugs would do;
# this one is pinned so the failure output is predictable.
target_slug="tasa-de-paro-epa"

# corrupt <dir> <scenario> applies one corruption in place. Written in node
# rather than jq/sed because the schema_version and Zod scenarios must
# RECOMPUTE the manifest digest, and node is already a hard requirement of
# this script (it runs astro build).
#
# Arguments travel through the ENVIRONMENT, not argv: `node -e` shifts
# process.argv by one relative to `node script.js`, and getting that offset
# wrong silently corrupts NOTHING while still exiting non-zero from a path
# the caller could mistake for success. The environment has no such offset.
corrupt() {
  local dir="$1" scenario="$2"
  # The single quotes around the program are required, so SC2016 is expected
  # here: the JavaScript owns its own ${...} template literals and reads its
  # arguments from process.env. Shell expansion inside this block would
  # corrupt the program rather than parameterise it.
  # shellcheck disable=SC2016
  CORRUPT_DIR="$dir" CORRUPT_SCENARIO="$scenario" CORRUPT_SLUG="$target_slug" \
  node --input-type=module -e '
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const dir = process.env.CORRUPT_DIR;
const scenario = process.env.CORRUPT_SCENARIO;
const slug = process.env.CORRUPT_SLUG;
const manifestPath = path.join(dir, "manifest.json");
const relative = `series/${slug}.json`;
const docPath = path.join(dir, relative);

if (scenario === "sha256-mismatch") {
  // A byte-level change with NO manifest reissue. Left as raw text on
  // purpose: this is what a truncated or tampered file looks like, and the
  // loader must refuse it before it ever parses the JSON.
  const raw = readFileSync(docPath, "utf-8");
  const mutated = raw.replace(/"value": [-0-9.]+/, `"value": 999999.99`);
  if (mutated === raw) throw new Error(`corruption no-op: found no value to mutate in ${relative}`);
  writeFileSync(docPath, mutated);
  process.exit(0);
}

// Both remaining scenarios must reach a LATER validation layer than the
// digest check, so the digest is reissued to match the corrupted bytes.
const manifest = JSON.parse(readFileSync(manifestPath, "utf-8"));
const doc = JSON.parse(readFileSync(docPath, "utf-8"));

if (scenario === "schema-version") {
  doc.schema_version = 2;
} else if (scenario === "zod-shape") {
  delete doc.pageState;
} else {
  throw new Error(`unknown scenario ${scenario}`);
}

const rewritten = JSON.stringify(doc, null, 2);
writeFileSync(docPath, rewritten);
manifest.digests[relative] = createHash("sha256").update(rewritten).digest("hex");
writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
'
}

# Every accepted failure must be attributable to the export loader itself,
# not merely coincide with a non-zero exit. Scenario 1 fails in the loader's
# own thrown message ("export loader: ..."); scenarios 2 and 3 fail inside
# the Zod parsers the loader calls, which name themselves in the stack. A
# build that died in Vite, in a missing dependency, or in an unrelated page
# matches none of these.
loader_origin='export loader:|parseSeriesDoc|parseManifest'

# run_scenario <scenario> <expected-error-regex> <human description>
run_scenario() {
  local scenario="$1" expected="$2" description="$3"
  local dir="$work_root/$scenario"
  local log="$work_root/$scenario.log"

  echo
  echo "=== scenario: $scenario — $description"

  cp -R "$honest_dir" "$dir"
  if ! corrupt "$dir" "$scenario"; then
    echo "FAIL: the $scenario corruption itself errored — nothing was proven." >&2
    return 1
  fi

  # A corruption that silently no-ops would leave the build passing on an
  # INTACT artifact, and a naive reading of that green build as "no
  # corruption tolerated" is the exact inversion this script must not
  # permit. So the mutation is confirmed to have landed before the build is
  # allowed to mean anything.
  if diff -q "$honest_dir/series/$target_slug.json" "$dir/series/$target_slug.json" >/dev/null 2>&1; then
    echo "FAIL: the $scenario corruption changed nothing in series/$target_slug.json." >&2
    return 1
  fi

  local status=0
  EXPORT_DIR="$dir" npm --prefix web run build >"$log" 2>&1 || status=$?

  if [[ $status -eq 0 ]]; then
    echo "FAIL: astro build SUCCEEDED on a corrupted artifact ($scenario)." >&2
    echo "      The loader tolerated the corruption instead of refusing it," >&2
    echo "      so a broken artifact would reach production as a green build." >&2
    echo "--- build output ---" >&2
    cat "$log" >&2
    return 1
  fi

  if ! grep -Eq "$expected" "$log"; then
    echo "FAIL: astro build exited $status on a corrupted artifact ($scenario)," >&2
    echo "      but its output never mentions /$expected/ — so the build broke" >&2
    echo "      for some OTHER reason and proves nothing about artifact" >&2
    echo "      validation." >&2
    echo "--- build output ---" >&2
    cat "$log" >&2
    return 1
  fi

  if ! grep -Eq "$loader_origin" "$log"; then
    echo "FAIL: astro build exited $status on a corrupted artifact ($scenario)" >&2
    echo "      and mentioned /$expected/, but nothing in its output is" >&2
    echo "      attributable to the export loader (/$loader_origin/) — the" >&2
    echo "      rejection cannot be credited to artifact validation." >&2
    echo "--- build output ---" >&2
    cat "$log" >&2
    return 1
  fi

  echo "PASS: astro build exited $status, named the expected failure, and the"
  echo "      failure came from the export loader."
  grep -Eo ".{0,120}$expected.{0,160}" "$log" | head -3
  return 0
}

failures=0
run_scenario sha256-mismatch "sha256 mismatch" \
  "bytes edited, manifest digest left stale" || failures=$((failures + 1))
run_scenario schema-version "schema_version" \
  "schema_version 2 with a REISSUED digest — the version check must reject it" || failures=$((failures + 1))
run_scenario zod-shape "pageState" \
  "required pageState removed with a REISSUED digest — Zod must reject it" || failures=$((failures + 1))

echo
if [[ $failures -ne 0 ]]; then
  echo "$failures of 3 corruption scenarios did NOT fail the build as required." >&2
  exit 1
fi

echo "All 3 corruption scenarios failed the build for the right reason."
echo "A corrupted export artifact cannot produce a green build."
