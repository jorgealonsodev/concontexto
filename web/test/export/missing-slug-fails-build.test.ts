// verify-report CRITICAL-27, the assertion that would have caught it, at
// the level the finding says the previous one was missing from.
//
// THE DEFECT: `/indicador/[slug].astro` resolved its route list by
// FILTERING `INDICATOR_CONTENT` down to the slugs the export artifact
// happened to carry. A frozen slug the artifact does not carry was dropped,
// and every layer that could have objected was satisfied:
// `app/internal/publishing/export.go` skips a series with zero observations
// (`if len(obs) == 0 { continue }`), so the slug is written into neither
// `series/` nor `manifest.series`; the digest chain is therefore complete
// and the Zod loader validates cleanly. Exit 0, five pages, and
// `/indicador/ocupados-epa/` returning 404 on a permalink the spec froze
// permanently.
//
// WHY THIS TEST EXISTS ALONGSIDE `test/indicator/routes.test.ts`. That
// suite proves the resolver's rule; it cannot prove the rule is what the
// BUILD uses. The previous all-six assertion —
// `.github/workflows/ingest-export-build.yml`'s "for slug in ...; do test -f
// web/dist/indicador/$slug/index.html" loop — had the opposite problem: it
// exercises the real build, but only ever against an artifact that contains
// all six, so no input it can receive makes it red. This test closes that
// gap from the only direction that works: it builds the REAL `npm run
// build`, against a REAL artifact directory, with a series genuinely
// removed from `manifest.series`, `manifest.digests` and `series/` — the
// exact three-part absence `export.go`'s `continue` produces.
//
// The two cases are a matched pair, for the same reason
// `unconfigured-build-fails.test.ts` uses one: case 1 alone would also pass
// if the build were broken for an unrelated reason, which is how a "loud
// failure" test becomes a green that proves nothing. Case 2 builds the same
// copied directory UNMUTATED and requires exit 0 with all six pages
// emitted, pinning case 1's failure to the removed slug and nothing else.
import { describe, expect, it } from "vitest";
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { cp, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const WEB_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const FIXTURE_DIR = path.join(WEB_DIR, "test/fixtures/export");

/** The frozen route whose absence is simulated, and the artifact slug that
 * backs it. They are the same string for this series (only `pib` diverges,
 * to `pib-cvi`), but they are named separately because the guard under test
 * has to cross that boundary. `ocupados-epa` is not an arbitrary choice: it
 * is the series `rule3-plausibility` blocks on a real ingest today, so this
 * is the case actually occurring in production, not a hypothetical. */
const MISSING_ROUTE_SLUG = "ocupados-epa";
const MISSING_ARTIFACT_SLUG = "ocupados-epa";

const ALL_SIX_ROUTE_SLUGS = [
  "tasa-de-paro-epa",
  "ocupados-epa",
  "ipc-general",
  "ipc-subyacente",
  "pib",
  "poblacion-residente",
] as const;

// Same reasoning as `unconfigured-build-fails.test.ts`: stripped from every
// child environment and re-added per case, so a variable exported in a
// developer's shell cannot turn a case green by accident.
const ARTIFACT_SOURCE_VARS = ["EXPORT_URL", "EXPORT_DIR", "BUILD_WITH_SYNTHETIC_FIXTURE", "WORKBENCH"] as const;

interface BuildRun {
  status: number | null;
  output: string;
  outDir: string;
}

/** Copies the golden fixture into a throwaway directory so a case can
 * mutate it. Never mutate `test/fixtures/export` in place: it is the input
 * of half this suite, and the Playwright run reads it too. */
async function copyArtifact(): Promise<string> {
  const dir = await mkdtemp(path.join(WEB_DIR, ".missing-slug-artifact-"));
  await cp(FIXTURE_DIR, dir, { recursive: true });
  return dir;
}

/** Removes one series from an artifact directory exactly the way
 * `export.go` omits a never-published one: the document is not written, and
 * the slug appears in neither `manifest.series` nor `manifest.digests`. A
 * removal that left the digest entry behind would be caught by the loader's
 * own digest check and would prove nothing about route resolution. */
async function removeSeries(dir: string, artifactSlug: string): Promise<void> {
  const manifestPath = path.join(dir, "manifest.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf-8")) as {
    series: string[];
    digests: Record<string, string>;
  };

  const relativePath = `series/${artifactSlug}.json`;
  expect(manifest.series, "fixture no longer carries the slug this test removes").toContain(artifactSlug);
  expect(manifest.digests, "fixture no longer digests the slug this test removes").toHaveProperty(relativePath);

  manifest.series = manifest.series.filter((slug) => slug !== artifactSlug);
  delete manifest.digests[relativePath];
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf-8");
  await rm(path.join(dir, relativePath), { force: true });
}

/** Runs the REAL `npm run build` against `artifactDir` via EXPORT_DIR — the
 * same switch `.github/workflows/ingest-export-build.yml` sets for the
 * exported artifact — into a throwaway output directory.
 *
 * `--outDir` inside `web/` for the two reasons the sibling guard documents:
 * it must not clobber the `dist/` the Playwright run and the Lighthouse
 * budget gate measure, and Astro finishes a build by `rename`-ing out of
 * `web/.astro/.prerender`, which fails with EXDEV across filesystems. */
async function runBuild(artifactDir: string): Promise<BuildRun> {
  const outDir = await mkdtemp(path.join(WEB_DIR, ".missing-slug-out-"));
  const env: Record<string, string | undefined> = { ...process.env, EXPORT_DIR: artifactDir };
  for (const name of ARTIFACT_SOURCE_VARS) {
    if (name !== "EXPORT_DIR") delete env[name];
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

describe("the build must fail when the artifact is missing a frozen slug (verify-report CRITICAL-27)", () => {
  it(
    "exits NON-ZERO, names the slug, and emits no page at all",
    async () => {
      const artifactDir = await copyArtifact();
      await removeSeries(artifactDir, MISSING_ARTIFACT_SLUG);
      const run = await runBuild(artifactDir);
      try {
        expect(
          run.status,
          `expected a non-zero exit — a green build here is a site that 404s /indicador/${MISSING_ROUTE_SLUG}/, a permalink the spec froze permanently.\n${run.output}`,
        ).not.toBe(0);

        // The message has to do the work: which slug, that it is frozen,
        // and where the fix lives (upstream, in the pipeline — never in
        // the web tree).
        expect(run.output).toMatch(new RegExp(MISSING_ROUTE_SLUG));
        expect(run.output).toMatch(/frozen/i);
        expect(run.output).toMatch(/never been published|zero observations/i);
        expect(run.output).toMatch(/publish gate|validation/i);

        // Nothing partial reaches the output directory. The whole point of
        // failing rather than filtering is that a five-page site never
        // gets built; design.md D-1's "exits build non-zero, previous
        // deploy stays live" depends on it.
        expect(existsSync(path.join(run.outDir, "indicador"))).toBe(false);
      } finally {
        await rm(run.outDir, { recursive: true, force: true });
        await rm(artifactDir, { recursive: true, force: true });
      }
    },
    240_000,
  );

  it(
    "still builds all six pages from the same directory when nothing is removed",
    async () => {
      const artifactDir = await copyArtifact();
      const run = await runBuild(artifactDir);
      try {
        expect(run.status, `expected exit 0 for an intact artifact.\n${run.output}`).toBe(0);
        for (const slug of ALL_SIX_ROUTE_SLUGS) {
          expect(
            existsSync(path.join(run.outDir, "indicador", slug, "index.html")),
            `missing built page for frozen slug "${slug}"`,
          ).toBe(true);
        }
      } finally {
        await rm(run.outDir, { recursive: true, force: true });
        await rm(artifactDir, { recursive: true, force: true });
      }
    },
    240_000,
  );
});
