// verify-report CRITICAL-15, the assertion that would have caught it.
//
// THE DEFECT: `resolveLoadOptionsFromEnv` fell back to
// `web/test/fixtures/export` when neither EXPORT_URL nor EXPORT_DIR was set.
// That fixture's own `source.txt` opens with "MOST OF THE NUMBERS IN THIS
// DIRECTORY ARE SYNTHESISED. They were never published by INE" — 98 to 294
// generated observations per series, of which only the newest three are real.
// `Dockerfile`'s web-builder stage ran `npm run build` with neither variable
// set, after `COPY web/ .` had brought the fixture into the build context, and
// there was no `.dockerignore`. So the image that ships rendered a
// straight-line invention under INE attribution, complete with an origin
// identifier and an extraction timestamp, and disclosed nothing. Reproduced
// verbatim before this test existed:
//
//   $ env -u EXPORT_URL -u EXPORT_DIR -u WORKBENCH npm run build
//   ...
//   [build] 7 page(s) built in 643ms                                  EXIT=0
//   $ grep -o "2002-Q1[^<]*" dist/indicador/tasa-de-paro-epa/index.html
//   2002-Q1: 11.85 % población activa, Definitivo
//
// WHY THIS TEST IS A REAL BUILD AND NOT A UNIT TEST. `loader.test.ts` already
// asserts `resolveLoadOptionsFromEnv({})` throws, and that unit test is the
// precise one. It is not the sufficient one: the defect was never that the
// function returned the wrong value in isolation — it was that the whole
// production build pipeline reached page output anyway. A mock of `astro
// build` would have been written against the same wrong assumption the code
// held. So this spawns the real `npm run build`, the exact command the
// Dockerfile runs, with the environment the Dockerfile had.
//
// The two cases below are a matched pair on purpose. The first asserts the
// unconfigured build FAILS; on its own that assertion would also pass if the
// build were broken for some entirely unrelated reason, which is the classic
// way a "loud failure" test becomes a green that proves nothing. The second
// runs the same build with the deliberate opt-in and requires it to SUCCEED
// and emit pages, so the first case's failure is pinned to the guard.
import { describe, expect, it } from "vitest";
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const WEB_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

// The variables that decide where the artifact comes from. Stripped from every
// child environment below and re-added explicitly per case, so a developer who
// happens to have one exported in their shell cannot turn either case green by
// accident.
const ARTIFACT_SOURCE_VARS = ["EXPORT_URL", "EXPORT_DIR", "BUILD_WITH_SYNTHETIC_FIXTURE", "WORKBENCH"] as const;

interface BuildRun {
  status: number | null;
  output: string;
  outDir: string;
}

/** Runs the REAL `npm run build` — the Dockerfile's own command — into a
 * throwaway output directory.
 *
 * `--outDir` rather than the default `dist/`: this suite must not clobber the
 * `dist/` the Playwright e2e run and the Lighthouse budget gate measure. The
 * directory is created INSIDE `web/` rather than in `os.tmpdir()` because
 * Astro finishes a build by `rename`-ing assets out of `web/.astro/.prerender`
 * into the output directory, and `rename` across filesystems fails with EXDEV
 * — a cross-device temp directory makes the build fail for a reason that has
 * nothing to do with what is under test (observed while writing this test). */
async function runBuild(extraEnv: Record<string, string>): Promise<BuildRun> {
  const outDir = await mkdtemp(path.join(WEB_DIR, ".build-guard-out-"));
  const env: Record<string, string | undefined> = { ...process.env, ...extraEnv };
  for (const name of ARTIFACT_SOURCE_VARS) {
    if (!(name in extraEnv)) delete env[name];
  }

  return new Promise<BuildRun>((resolve, reject) => {
    const child = spawn("npm", ["run", "build", "--", "--outDir", outDir], { cwd: WEB_DIR, env });
    let output = "";
    child.stdout.setEncoding("utf-8").on("data", (chunk: string) => {
      output += chunk;
    });
    child.stderr.setEncoding("utf-8").on("data", (chunk: string) => {
      output += chunk;
    });
    child.on("error", reject);
    child.on("close", (status) => resolve({ status, output, outDir }));
  });
}

describe("the production build must refuse to guess an artifact source (verify-report CRITICAL-15)", () => {
  it(
    "exits NON-ZERO when neither EXPORT_URL nor EXPORT_DIR is set, and emits no indicator page",
    async () => {
      const run = await runBuild({});
      try {
        expect(
          run.status,
          `expected a non-zero exit — an unconfigured build that succeeds is a build publishing synthesised numbers as INE statistics.\n${run.output}`,
        ).not.toBe(0);

        // Loud, and actionable: the operator must learn both variable names
        // and why the build refused, not just see a stack trace.
        expect(run.output).toMatch(/EXPORT_URL/);
        expect(run.output).toMatch(/EXPORT_DIR/);
        expect(run.output).toMatch(/synthes/i);

        // Nothing partial may reach the output directory. Astro's contract
        // (design.md D-1: "exits build non-zero, previous deploy stays live")
        // depends on a failed build shipping no page at all.
        expect(existsSync(path.join(run.outDir, "indicador"))).toBe(false);
      } finally {
        await rm(run.outDir, { recursive: true, force: true });
      }
    },
    240_000,
  );

  it(
    "still builds, and emits the indicator pages, when the synthetic fixture is deliberately opted into",
    async () => {
      const run = await runBuild({ BUILD_WITH_SYNTHETIC_FIXTURE: "1" });
      try {
        expect(run.status, `expected exit 0 for an explicitly opted-in fixture build.\n${run.output}`).toBe(0);
        expect(existsSync(path.join(run.outDir, "indicador/tasa-de-paro-epa/index.html"))).toBe(true);
      } finally {
        await rm(run.outDir, { recursive: true, force: true });
      }
    },
    240_000,
  );
});
