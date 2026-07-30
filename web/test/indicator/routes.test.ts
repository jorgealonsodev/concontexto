// verify-report CRITICAL-27, at the level where the defect actually lives:
// ROUTE RESOLUTION.
//
// THE DEFECT: `/indicador/[slug].astro`'s `getStaticPaths` resolved its
// route list with a FILTER —
//
//   const slugs = Object.keys(INDICATOR_CONTENT).filter(
//     (slug) => seriesBySlug.has(artifactSlugFor(slug)) && METHODOLOGY_CONTENT[slug] !== undefined,
//   );
//
// A frozen slug the export artifact does not carry was silently dropped.
// Nothing downstream noticed: `app/internal/publishing/export.go` skips a
// series with zero observations (`if len(obs) == 0 { continue }`), so the
// slug is absent from `series/` AND from `manifest.series` — the digest
// chain is complete and the Zod loader validates cleanly. Green build,
// exit 0, five pages, and `/indicador/ocupados-epa/` returning 404 on a
// permalink the spec froze.
//
// WHY A UNIT TEST HERE IS THE HONEST PLACE FOR THE ASSERTION. The only
// all-six check that existed lived in `.github/workflows/ingest-export-build.yml`,
// which asserts the six pages exist in `web/dist/` after building from a
// fixture that ALWAYS contains all six. It cannot go red for this defect —
// the failing input never reaches it. This suite constructs the failing
// input directly: an artifact map with a frozen slug removed. Remove the
// guard from `src/lib/indicator/routes.ts` and every case below goes red.
//
// Its end-to-end counterpart is `test/export/missing-slug-fails-build.test.ts`,
// which runs the REAL `astro build` against a REAL artifact directory with
// a series removed. This file proves the rule; that one proves the rule is
// wired into the build.
import { describe, expect, it } from "vitest";

import {
  FROZEN_INDICATOR_SLUGS,
  resolveIndicatorRouteSlugs,
} from "../../src/lib/indicator/routes";
import { INDICATOR_CONTENT } from "../../src/content/indicators";
import { METHODOLOGY_CONTENT } from "../../src/content/indicators/methodology";

/** The slugs the export artifact carries when every series has published.
 * `pib-cvi`, not `pib`: the artifact uses the Go pipeline's own slug for
 * that one series (`config/series/pib-cvi.yaml`), while the frozen public
 * route is `pib` — see `IndicatorContentConfig.artifactSlug`. Spelling the
 * artifact side out here rather than deriving it keeps this fixture honest
 * about what the two identifier spaces really contain. */
const FULL_ARTIFACT_SLUGS = [
  "tasa-de-paro-epa",
  "ocupados-epa",
  "ipc-general",
  "ipc-subyacente",
  "pib-cvi",
  "poblacion-residente",
] as const;

/** The unit each artifact document declares, spelled out rather than
 * derived from `INDICATOR_CONTENT` — deriving it would make every fixture
 * below agree with the content catalog by construction, which is precisely
 * the comparison `resolveIndicatorRouteSlugs` now makes. `ipc-general` and
 * `ipc-subyacente` really are the bare `índice` here while their pages read
 * `índice (base 2021=100)`; that legitimate difference is what the guard's
 * "elaborates" rule exists to allow, and it is exercised in
 * `unit-agreement.test.ts`. */
const ARTIFACT_UNITS: Record<string, string> = {
  "tasa-de-paro-epa": "% población activa",
  "ocupados-epa": "miles de personas",
  "ipc-general": "índice",
  "ipc-subyacente": "índice",
  "pib-cvi": "índice de volumen encadenado",
  "poblacion-residente": "personas",
};

/** Builds the shape `loadExportArtifact` returns as `seriesBySlug`. The
 * resolver reads exactly one field off a document (`unit`, to check the
 * page's label does not contradict the measurement), so that is the one
 * field these stand-ins carry — a real `SeriesDoc` per slug would be
 * several hundred lines of provenance no assertion here reads.
 *
 * A slug with no entry in `ARTIFACT_UNITS` gets its own name as a unit.
 * That is only ever reached by the unfrozen-route case below, whose slug is
 * never unit-checked because it is not one of the six. */
function artifactWith(slugs: readonly string[]): ReadonlyMap<string, { unit: string }> {
  return new Map(slugs.map((slug) => [slug, { unit: ARTIFACT_UNITS[slug] ?? slug }]));
}

function withoutKey<T>(record: Record<string, T>, key: string): Record<string, T> {
  const copy = { ...record };
  delete copy[key];
  return copy;
}

describe("resolveIndicatorRouteSlugs (verify-report CRITICAL-27)", () => {
  it("returns all six frozen slugs, in frozen order, from a complete artifact", () => {
    expect(resolveIndicatorRouteSlugs(artifactWith(FULL_ARTIFACT_SLUGS))).toEqual([
      ...FROZEN_INDICATOR_SLUGS,
    ]);
  });

  it("resolves `pib` through its diverging artifact slug `pib-cvi`, not the route slug", () => {
    // Guards the alias in the direction that matters: an artifact carrying
    // `pib` and NOT `pib-cvi` is the artifact the Go pipeline does not
    // write, so it must be treated as a missing series rather than quietly
    // accepted by a route-slug-first lookup.
    const wrongWayRound = FULL_ARTIFACT_SLUGS.map((slug) => (slug === "pib-cvi" ? "pib" : slug));
    expect(() => resolveIndicatorRouteSlugs(artifactWith(wrongWayRound))).toThrow(/"pib"/);
  });

  it("THROWS when a frozen slug is absent from the artifact, instead of shipping five pages", () => {
    const missingOcupados = FULL_ARTIFACT_SLUGS.filter((slug) => slug !== "ocupados-epa");
    expect(() => resolveIndicatorRouteSlugs(artifactWith(missingOcupados))).toThrow();
  });

  it("names the missing slug, calls it frozen, and sends the reader to the pipeline", () => {
    const missingOcupados = FULL_ARTIFACT_SLUGS.filter((slug) => slug !== "ocupados-epa");
    let message = "";
    try {
      resolveIndicatorRouteSlugs(artifactWith(missingOcupados));
    } catch (err) {
      message = (err as Error).message;
    }

    // Which slug.
    expect(message).toMatch(/ocupados-epa/);
    // That it is one of the six permanent permalinks, not an optional page.
    expect(message).toMatch(/frozen/i);
    // Where to look. A build failure whose message sends the reader to the
    // right place is the entire point of failing rather than filtering:
    // both causes are upstream, in the Go pipeline, never in this file.
    expect(message).toMatch(/never been published|zero observations/i);
    expect(message).toMatch(/publish gate|validation/i);
    // The artifact's actual contents, so the reader can see what DID
    // publish without going and opening manifest.json.
    expect(message).toMatch(/pib-cvi/);
  });

  it("does not blame the pipeline when the real gap is an unauthored methodology entry", () => {
    // The two filter conditions were collapsed into one boolean, so both
    // failures produced the same (absent) outcome. They have different
    // fixes: this one is fixed in the repository, by a human writing prose.
    let message = "";
    try {
      resolveIndicatorRouteSlugs(artifactWith(FULL_ARTIFACT_SLUGS), {
        methodology: withoutKey(METHODOLOGY_CONTENT, "ipc-general"),
      });
    } catch (err) {
      message = (err as Error).message;
    }

    expect(message).toMatch(/ipc-general/);
    expect(message).toMatch(/methodology\.ts/);
    expect(message).not.toMatch(/publish gate/i);
    expect(message).not.toMatch(/never been published/i);
  });

  it("reports a missing presentation-config entry as its own, third failure", () => {
    let message = "";
    try {
      resolveIndicatorRouteSlugs(artifactWith(FULL_ARTIFACT_SLUGS), {
        indicators: withoutKey(INDICATOR_CONTENT, "poblacion-residente"),
      });
    } catch (err) {
      message = (err as Error).message;
    }

    expect(message).toMatch(/poblacion-residente/);
    expect(message).toMatch(/content\/indicators/);
    expect(message).not.toMatch(/publish gate/i);
  });

  it("refuses an authored route that is not one of the frozen six, rather than ignoring it", () => {
    // The mirror image of the defect. Resolving from the frozen list means
    // a seventh `content/indicators/` entry would build nothing and say
    // nothing — the same silent drop, pointing the other way. The spec
    // says "exactly these six", so an extra entry is a decision someone
    // has to make deliberately, by editing the frozen list.
    let message = "";
    try {
      resolveIndicatorRouteSlugs(artifactWith([...FULL_ARTIFACT_SLUGS, "afiliacion-ss"]), {
        indicators: { ...INDICATOR_CONTENT, "afiliacion-ss": INDICATOR_CONTENT["pib"] },
      });
    } catch (err) {
      message = (err as Error).message;
    }

    expect(message).toMatch(/afiliacion-ss/);
    expect(message).toMatch(/frozen/i);
  });

  it("keeps the frozen list identical to the spec's own six slugs", () => {
    // indicator-page spec, "Six indicator routes with frozen slugs": the
    // build MUST produce /indicador/{slug} for exactly these six. If this
    // list ever drifts from the spec, every guard above is checking the
    // wrong thing.
    expect([...FROZEN_INDICATOR_SLUGS].sort()).toEqual([
      "ipc-general",
      "ipc-subyacente",
      "ocupados-epa",
      "pib",
      "poblacion-residente",
      "tasa-de-paro-epa",
    ]);
  });
});
