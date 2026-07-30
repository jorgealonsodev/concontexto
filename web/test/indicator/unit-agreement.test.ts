// The editorial unit label must never contradict the unit the pipeline
// measured in.
//
// THE DEFECT. `web/src/content/indicators/ocupados-epa.ts` declared
// `unit: "personas"`. Every other source of truth said otherwise:
// `config/series/ocupados-epa.yaml` says `miles de personas`, the export
// artifact says `"unit": "miles de personas"`, and INE's own API for that
// series (`DATOS_SERIE/EPA387796`) returns `T3_Unidad: "Personas"` with
// `T3_Escala: "Miles"` beside `Valor: 22779.0`. The page header, and later
// the homepage card, therefore published `22779 personas` for a figure that
// means 22.8 million people — wrong by a factor of a thousand, in the one
// number a reader takes away.
//
// WHY THE ASSERTION LIVES HERE AND NOT IN A CONTENT TEST. This is the same
// shape as verify-report CRITICAL-27: a fact derived twice, with the weaker
// derivation winning silently. The artifact carries the unit and the
// editorial catalog restates it, so nothing anywhere compared the two and
// the restatement was free to drift. A test that merely pinned the string
// `"miles de personas"` would fix today's value and leave the SHAPE intact —
// the next series to be rescaled upstream would drift exactly the same way.
// So the rule is checked against the REAL artifact, inside the same guard
// `/` and `/indicador/{slug}` already both call, and a drifted label fails
// the build the way a missing frozen route does.
//
// WHAT "AGREE" MEANS, AND WHY IT IS NOT STRING EQUALITY. Two of the six
// labels legitimately differ, and must keep differing: `ipc-general` and
// `ipc-subyacente` are `índice` in the artifact and
// `índice (base 2021=100)` on the page, because the artifact's own `base`
// field is a disclosed gap (`schema.ts`: "always null until a future slice
// adds index-base config") and the editorial layer is what tells a reader
// which base the index is on. The rule is therefore that editorial copy may
// only ELABORATE the pipeline's unit — never replace, shorten or rescale
// it. `personas` is not an elaboration of `miles de personas`; it is a
// different measurement.
import { describe, expect, it } from "vitest";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { resolveIndicatorRouteSlugs, FROZEN_INDICATOR_SLUGS, artifactSlugFor } from "../../src/lib/indicator/routes";
import { INDICATOR_CONTENT } from "../../src/content/indicators";
import { loadExportArtifact } from "../../src/lib/export/loader";
import type { SeriesDoc } from "../../src/lib/export/schema";

const FIXTURES_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../fixtures/export");

async function fixtureSeries(): Promise<Map<string, SeriesDoc>> {
  const { seriesBySlug } = await loadExportArtifact({ dir: FIXTURES_DIR });
  return seriesBySlug;
}

/** Overrides one route's editorial config, leaving the other five real. */
function withUnit(routeSlug: string, unit: string) {
  return { indicators: { ...INDICATOR_CONTENT, [routeSlug]: { ...INDICATOR_CONTENT[routeSlug], unit } } };
}

describe("editorial unit labels agree with the artifact they describe", () => {
  it("accepts the repository as it stands: all six labels agree with the real artifact", async () => {
    // The assertion that goes red on the live defect. It reads the checked-in
    // artifact rather than a synthesised one, so it is the six real labels
    // being checked against the six real units — not a shape rehearsal.
    const seriesBySlug = await fixtureSeries();
    expect(() => resolveIndicatorRouteSlugs(seriesBySlug)).not.toThrow();
  });

  it("agrees on every frozen slug when compared field by field, not just in aggregate", async () => {
    // The aggregate assertion above passes if the guard is deleted. This one
    // states the property itself, so the six labels are pinned even while
    // the guard is being rewritten.
    const seriesBySlug = await fixtureSeries();
    for (const routeSlug of FROZEN_INDICATOR_SLUGS) {
      const artifactUnit = seriesBySlug.get(artifactSlugFor(routeSlug))!.unit;
      const editorialUnit = INDICATOR_CONTENT[routeSlug].unit;
      expect(
        editorialUnit.startsWith(artifactUnit),
        `"${routeSlug}": the page says "${editorialUnit}" but the artifact measured in "${artifactUnit}"`,
      ).toBe(true);
    }
  });

  it("REFUSES to build when a label drops the artifact's scale — the live `personas`/`miles de personas` defect", async () => {
    const seriesBySlug = await fixtureSeries();
    expect(() => resolveIndicatorRouteSlugs(seriesBySlug, withUnit("ocupados-epa", "personas"))).toThrow(
      /ocupados-epa/,
    );
  });

  it("names both strings and sends the reader to the editorial file, not to the pipeline", async () => {
    const seriesBySlug = await fixtureSeries();
    let message = "";
    try {
      resolveIndicatorRouteSlugs(seriesBySlug, withUnit("ocupados-epa", "personas"));
    } catch (err) {
      message = (err as Error).message;
    }

    // Which slug, and both sides of the contradiction — a message naming
    // only one of them leaves the reader to go and open the artifact.
    expect(message).toMatch(/ocupados-epa/);
    expect(message).toMatch(/"personas"/);
    expect(message).toMatch(/"miles de personas"/);
    // Where the fix goes. Unlike a missing series, this one is NOT a
    // pipeline outage: the artifact is right and the prose is wrong, so the
    // message must not send anyone to the ingestion logs.
    expect(message).toMatch(/content\/indicators\/ocupados-epa\.ts/);
    expect(message).not.toMatch(/publish gate/i);
  });

  it("still accepts a label that ELABORATES the artifact's unit — IPC's index base", async () => {
    // `ipc-general` really is `índice` in the artifact and
    // `índice (base 2021=100)` on the page, and that is the behaviour being
    // protected, not tolerated: a guard that demanded equality would delete
    // the base from two public pages to make itself pass.
    const seriesBySlug = await fixtureSeries();
    expect(seriesBySlug.get("ipc-general")!.unit).toBe("índice");
    expect(INDICATOR_CONTENT["ipc-general"].unit).toBe("índice (base 2021=100)");
    expect(() => resolveIndicatorRouteSlugs(seriesBySlug)).not.toThrow();
  });

  it("rejects an elaboration that is welded onto the unit rather than appended after a separator", async () => {
    // `índicex` starts with `índice` but is a different word. Requiring a
    // space or an opening parenthesis after the artifact's unit keeps the
    // rule "the pipeline's unit, then more" rather than "any string with
    // the right prefix".
    const seriesBySlug = await fixtureSeries();
    expect(() => resolveIndicatorRouteSlugs(seriesBySlug, withUnit("ipc-general", "índicex"))).toThrow(
      /ipc-general/,
    );
  });

  it("rejects the drift pointing the other way: a label that INVENTS a scale the pipeline never applied", async () => {
    // The mirror image of the live defect, and the more dangerous of the
    // two: the artifact says `personas` and the page would claim thousands.
    const seriesBySlug = await fixtureSeries();
    expect(() =>
      resolveIndicatorRouteSlugs(seriesBySlug, withUnit("poblacion-residente", "miles de personas")),
    ).toThrow(/poblacion-residente/);
  });

  it("compares Unicode-normalised text, so an NFD accent is not reported as a mismatch", async () => {
    // `índice` written with a combining acute is byte-different and
    // reader-identical. A guard that failed on it would send someone
    // hunting for a difference they cannot see in their editor.
    const seriesBySlug = await fixtureSeries();
    const decomposed = "índice (base 2021=100)".normalize("NFD");
    expect(decomposed).not.toBe("índice (base 2021=100)");
    expect(() => resolveIndicatorRouteSlugs(seriesBySlug, withUnit("ipc-general", decomposed))).not.toThrow();
  });
});
