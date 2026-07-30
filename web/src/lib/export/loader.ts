// The Astro build's read-side of the export artifact contract (design.md
// D-1: "Astro fetches that same public artifact, re-validates it (Zod +
// sha256 + exact schema_version equality)"; publishing-export spec, "The
// artifact is validated on read by the build" — "An unsupported version or
// a shape mismatch MUST fail the build loudly... MUST NOT emit a partial
// site and MUST NOT silently skip a series it cannot parse"). Task 9a.1/
// 9a.2.
//
// Two modes (design.md D-1's "EXPORT_URL live / EXPORT_DIR fixture"):
//   - `dir`: reads manifest.json + series/{slug}.json off the filesystem —
//     a /data-derived directory the Go pipeline wrote, or the SYNTHETIC golden
//     fixture in tests and local builds that explicitly opt into it (see
//     `resolveLoadOptionsFromEnv`: there is no default, and the fixture is
//     never reached by accident).
//   - `url`: fetches the same two document shapes over HTTP from a live,
//     already-published /data-derived/ origin.
// Every file, in EITHER mode, is verified against manifest.json's declared
// sha256 digest BEFORE it is parsed — a tampered or corrupted artifact is
// refused, never silently accepted (indicator-page spec's own "zero
// database queries and zero computation at request time" contract means
// this loader's ONLY legitimate build-time read is the artifact itself,
// never a database).
//
// Any failure throws. Astro's static build treats an uncaught error inside
// `getStaticPaths`/a page module as a fatal, non-zero-exit build failure
// that emits no page output — exactly the "exits build non-zero, previous
// deploy stays live" contract this loader exists to guarantee. There is no
// catch-and-continue path anywhere in this module by design.
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";

import { parseManifest, parseSeriesDoc, type Manifest, type SeriesDoc } from "./schema";

export interface LoadArtifactOptions {
  /** Absolute or cwd-relative directory holding manifest.json + series/*.json. */
  dir?: string;
  /** Base URL to fetch manifest.json + series/*.json from over HTTP — the
   * deployed .../data-derived/ origin, trailing slash optional. */
  url?: string;
}

export interface LoadedArtifact {
  manifest: Manifest;
  seriesBySlug: Map<string, SeriesDoc>;
}

/** The golden fixture directory — SYNTHETIC DATA, opt-in only.
 *
 * `web/test/fixtures/export/source.txt` states it in full: "MOST OF THE
 * NUMBERS IN THIS DIRECTORY ARE SYNTHESISED. They were never published by
 * INE." Only the newest three observations of each series are real; the 95 to
 * 291 rows of history before them are a straight line produced by a formula,
 * carrying real INE attribution, real origin identifiers and a real extraction
 * timestamp. Rendered into a page, it is indistinguishable from a statistic.
 *
 * This directory is therefore reachable ONLY through
 * `SYNTHETIC_FIXTURE_OPT_IN` below. It used to be the DEFAULT, and that is
 * verify-report CRITICAL-15: see `resolveLoadOptionsFromEnv`. */
const SYNTHETIC_FIXTURE_DIR = "test/fixtures/export";

/** The one signal that unlocks `SYNTHETIC_FIXTURE_DIR`. Its name is the whole
 * point: it cannot be set by accident, and it cannot be read as anything other
 * than "I am deliberately building against non-production fixture data".
 *
 * Set by the vitest suite, by Playwright's own `webServer` build
 * (`playwright.config.ts`), by ci.yml's preview/production-shape build, and by
 * a developer building locally. NEVER by the Dockerfile, and never by anything
 * that produces bytes a reader could receive.
 *
 * Compared against the exact string "1", matching the convention
 * `astro.config.mjs` already uses for `WORKBENCH` — see
 * `resolveLoadOptionsFromEnv` for why a looser truthiness check would be
 * wrong. */
const SYNTHETIC_FIXTURE_OPT_IN = "BUILD_WITH_SYNTHETIC_FIXTURE";

function sha256Hex(bytes: Buffer): string {
  return createHash("sha256").update(bytes).digest("hex");
}

async function readBytes(opts: LoadArtifactOptions, relativePath: string): Promise<Buffer> {
  if (opts.url) {
    const base = opts.url.endsWith("/") ? opts.url : `${opts.url}/`;
    const target = new URL(relativePath, base);
    const res = await fetch(target);
    if (!res.ok) {
      throw new Error(`export loader: fetching "${target}" failed with HTTP ${res.status}`);
    }
    return Buffer.from(await res.arrayBuffer());
  }
  // No default: `resolveLoadOptionsFromEnv` refuses to produce options with
  // neither field, and every other caller passes one explicitly. An empty
  // `opts` reaching here is a programming error, and saying so beats silently
  // reading some directory nobody chose.
  if (!opts.dir) {
    throw new Error(
      "export loader: loadExportArtifact was called with neither `dir` nor `url` — there is no default artifact source (see resolveLoadOptionsFromEnv)",
    );
  }
  const dir = opts.dir;
  const filePath = path.isAbsolute(dir) ? path.join(dir, relativePath) : path.resolve(process.cwd(), dir, relativePath);
  try {
    return await readFile(filePath);
  } catch (err) {
    // A raw ENOENT here names a path and nothing else, which is the least
    // useful thing to print when the cause is almost always a misconfigured
    // artifact source — a typo in EXPORT_DIR, or a build context that
    // legitimately does not contain the directory (`.dockerignore` removes the
    // test tree, so an image build that opted into the synthetic fixture lands
    // here rather than at the digest check).
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      throw new Error(
        `export loader: "${filePath}" does not exist — the configured artifact directory ("${dir}") holds no ${relativePath}. Check EXPORT_DIR, or, if this is a live deploy, set EXPORT_URL instead.`,
      );
    }
    throw err;
  }
}

function parseJSON(relativePath: string, bytes: Buffer): unknown {
  try {
    return JSON.parse(bytes.toString("utf-8"));
  } catch (err) {
    throw new Error(`export loader: ${relativePath} is not valid JSON: ${(err as Error).message}`);
  }
}

/** Verifies `bytes` against the digest manifest.json declares for
 * `relativePath` — computed BEFORE any Zod parsing, so a byte-level
 * corruption or tamper is caught even if the corrupted content happens to
 * still be syntactically valid JSON matching the schema. */
function verifyDigest(relativePath: string, bytes: Buffer, manifest: Manifest): void {
  const expected = manifest.digests[relativePath];
  if (!expected) {
    throw new Error(`export loader: manifest declares no digest for "${relativePath}"`);
  }
  const actual = sha256Hex(bytes);
  if (actual !== expected) {
    throw new Error(
      `export loader: sha256 mismatch for "${relativePath}" (manifest declares ${expected}, computed ${actual}) — refusing to build from a tampered or corrupted artifact`,
    );
  }
}

/**
 * Loads and fully validates the export artifact: every file's sha256 is
 * checked against manifest.json's declared digest, every document is
 * parsed through the slice-5 Zod schemas (`schema.ts`), and
 * `schema_version` is required to equal `SUPPORTED_SCHEMA_VERSION`
 * EXACTLY — never `>=` (both `parseManifest`/`parseSeriesDoc` already
 * enforce this; this function does not duplicate that check).
 */
export async function loadExportArtifact(opts: LoadArtifactOptions = {}): Promise<LoadedArtifact> {
  const manifestBytes = await readBytes(opts, "manifest.json");
  const manifest = parseManifest(parseJSON("manifest.json", manifestBytes));

  const seriesBySlug = new Map<string, SeriesDoc>();
  for (const slug of manifest.series) {
    const relativePath = `series/${slug}.json`;
    const bytes = await readBytes(opts, relativePath);
    verifyDigest(relativePath, bytes, manifest);
    const doc = parseSeriesDoc(parseJSON(relativePath, bytes));
    if (doc.slug !== slug) {
      throw new Error(
        `export loader: manifest lists slug "${slug}" at "${relativePath}" but its document declares slug "${doc.slug}"`,
      );
    }
    seriesBySlug.set(slug, doc);
  }

  return { manifest, seriesBySlug };
}

/** Resolves loader options from the environment (design.md D-1's
 * "EXPORT_URL live / EXPORT_DIR fixture" switch) — the one call site
 * production `getStaticPaths` uses. Every other caller (tests) passes
 * explicit options rather than touching `process.env` directly, so this
 * function is the single, testable seam.
 *
 * THIS FUNCTION REFUSES TO GUESS. Until verify-report CRITICAL-15 it ended in
 * `return { dir: DEFAULT_FIXTURE_DIR }`, and its own doc comment defended that
 * default with "the real production deploy pipeline always sets EXPORT_URL or
 * EXPORT_DIR explicitly, never relies on this default". That sentence was
 * false, and provably so from one file in this repository: `Dockerfile`'s
 * web-builder stage ran `COPY web/ .` followed by `RUN npm run build` with
 * NEITHER variable set and no `.dockerignore` to keep the fixture out of the
 * build context. The image that ships therefore rendered 98 to 294 SYNTHESISED
 * observations per series as INE statistics — `2002-Q1: 11.85 % población
 * activa, Definitivo` was read straight out of the real built HTML, and 11.85
 * is the first output of the straight-line formula
 * `web/test/fixtures/export/source.txt` documents.
 *
 * A silent fallback to synthesised data is never acceptable in a build that
 * could ship, and no comment can make it acceptable, because a comment is not
 * a mechanism. The mechanism is this: an unconfigured build FAILS, and the
 * fixture is reachable only by explicitly asking for it. The failure mode of
 * getting this wrong is now a red build; it used to be a published lie.
 *
 * Precedence, and why:
 *   1. `EXPORT_URL`  — a live, already-published /data-derived/ origin.
 *   2. `EXPORT_DIR`  — a directory `concontexto export` wrote (design.md D-1's
 *                      "the artifact IS /data-derived/").
 *   3. `BUILD_WITH_SYNTHETIC_FIXTURE=1` — the synthetic fixture, opt-in.
 *   4. Otherwise: throw.
 * A real artifact source outranks the opt-in deliberately: a stale
 * `BUILD_WITH_SYNTHETIC_FIXTURE` left in a shell or a CI environment must never
 * be able to downgrade a build that was handed real data.
 *
 * The opt-in is compared against the exact string "1" rather than tested for
 * truthiness, for the same reason `astro.config.mjs` does it for `WORKBENCH`:
 * loose truthiness turns `BUILD_WITH_SYNTHETIC_FIXTURE=0` and
 * `BUILD_WITH_SYNTHETIC_FIXTURE=false` — both of which an operator writes to
 * mean "no" — into "yes". */
export function resolveLoadOptionsFromEnv(
  env: Record<string, string | undefined> = process.env,
): LoadArtifactOptions {
  if (env.EXPORT_URL) return { url: env.EXPORT_URL };
  if (env.EXPORT_DIR) return { dir: env.EXPORT_DIR };
  if (env[SYNTHETIC_FIXTURE_OPT_IN] === "1") return { dir: SYNTHETIC_FIXTURE_DIR };

  throw new Error(
    "export loader: no artifact source is configured, so this build has nothing honest to render and refuses to continue.\n" +
      "  Set EXPORT_URL to the live, already-published /data-derived/ origin, or\n" +
      "  set EXPORT_DIR to a directory holding manifest.json + series/*.json written by `concontexto export`.\n" +
      `  The only other artifact in this repository is ${SYNTHETIC_FIXTURE_DIR}, whose history is SYNTHESISED — ` +
      "invented numbers carrying real INE attribution (see that directory's source.txt). This build will not fall back to it, " +
      "because a page built from it is indistinguishable from a page built from published statistics.\n" +
      `  A test or local build that deliberately wants that synthetic fixture must say so: ${SYNTHETIC_FIXTURE_OPT_IN}=1.`,
  );
}
