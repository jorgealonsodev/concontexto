// The homepage's own listing derivation (milestone 1.2: `/` lists the six
// frozen indicators and links to them). Drives `src/lib/indicator/homeListing.ts`.
//
// WHY THIS IS A LIBRARY TEST AND NOT ONLY A PAGE TEST. The interesting
// behaviour of a homepage listing is not its markup — it is WHICH indicators
// it agrees to list, and what it does when it cannot list all of them. That
// is the same question `routes.ts` answers for `/indicador/{slug}`
// (verify-report CRITICAL-27: a `.filter()` turned a pipeline outage into a
// silently smaller website), and it has to be answerable here too, because a
// homepage that quietly lists five of six frozen indicators hides the
// missing one from every reader who has no other way to reach it. Asserting
// it against a real rendered page would mean asserting the ABSENCE of a card
// — a test that passes for the wrong reason the moment the markup changes.
import { describe, expect, it } from "vitest";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { homeIndicatorListing } from "../../src/lib/indicator/homeListing";
import { FROZEN_INDICATOR_SLUGS } from "../../src/lib/indicator/routes";
import { loadExportArtifact } from "../../src/lib/export/loader";
import { INDICATOR_CONTENT } from "../../src/content/indicators";
import type { SeriesDoc } from "../../src/lib/export/schema";

const FIXTURES_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../fixtures/export");

async function fixtureSeries(): Promise<Map<string, SeriesDoc>> {
  const { seriesBySlug } = await loadExportArtifact({ dir: FIXTURES_DIR });
  return seriesBySlug;
}

describe("homeIndicatorListing — which indicators the homepage agrees to list", () => {
  it("lists exactly the six frozen slugs, in the order the spec froze them", async () => {
    const entries = homeIndicatorListing(await fixtureSeries());

    expect(entries.map((entry) => entry.slug)).toEqual([...FROZEN_INDICATOR_SLUGS]);
  });

  it("refuses to list anything when a frozen slug is absent from the artifact, naming the slug", async () => {
    // The live situation this guard exists for: `ocupados-epa` is currently
    // held back by the ingestion publish gate, so `concontexto export` writes
    // no series document for it and the artifact simply does not carry it.
    // A homepage that responded by listing the other five would look
    // complete, and the one indicator a reader could no longer reach would be
    // the one nothing on the site mentions again.
    const seriesBySlug = new Map(await fixtureSeries());
    seriesBySlug.delete("ocupados-epa");

    expect(() => homeIndicatorListing(seriesBySlug)).toThrow(/ocupados-epa/);
    // Not merely "it threw": the message must say which slug and that the
    // cause is upstream, because that is the difference between a build
    // failure someone can act on and one they work around.
    expect(() => homeIndicatorListing(seriesBySlug)).toThrow(/PIPELINE PROBLEM/);
  });

  it("carries each indicator's editorial name, unit and decimals from the content catalog", async () => {
    const entries = homeIndicatorListing(await fixtureSeries());
    const paro = entries.find((entry) => entry.slug === "tasa-de-paro-epa");

    expect(paro).toBeDefined();
    expect(paro!.name).toBe(INDICATOR_CONTENT["tasa-de-paro-epa"].name);
    expect(paro!.unit).toBe(INDICATOR_CONTENT["tasa-de-paro-epa"].unit);
    expect(paro!.decimals).toBe(INDICATOR_CONTENT["tasa-de-paro-epa"].decimals);
  });

  it("carries each indicator's latest published observation and freshness from the artifact", async () => {
    const entries = homeIndicatorListing(await fixtureSeries());
    const paro = entries.find((entry) => entry.slug === "tasa-de-paro-epa")!;

    // The newest observation of every fixture series is a verbatim copy of a
    // live-verified INE response (web/test/fixtures/export/source.txt), so
    // these two numbers are real published figures, not invented ones.
    expect(paro.latestPeriod).toBe("2026-Q2");
    expect(paro.latestValue).toBe(9.87);
    expect(paro.freshness).toBe("fresh");
  });

  it("resolves `pib` through its diverging artifact slug `pib-cvi`", async () => {
    // The one route slug that is not the pipeline's own series slug
    // (config/series/pib-cvi.yaml). A listing that looked the artifact up by
    // route slug would find nothing for PIB and would have to either drop it
    // or invent a value for it.
    const pib = homeIndicatorListing(await fixtureSeries()).find((entry) => entry.slug === "pib")!;

    expect(pib.latestPeriod).toBe("2026-Q1");
    expect(pib.latestValue).toBe(121.9959);
  });

  it("refuses to list a series the artifact carries with no observations, rather than showing an empty card", async () => {
    // P4, and the reason there is no fourth "no data yet" card state: a card
    // reading "— %" under a real indicator's name asserts a measurement that
    // was never published. `concontexto export` never writes such a document
    // today (it skips a zero-observation series outright), so reaching this
    // branch means the artifact contract changed — which is a thing to be
    // told about, not to paper over with a dash.
    const seriesBySlug = new Map(await fixtureSeries());
    const empty = { ...seriesBySlug.get("ipc-general")!, points: [] };
    seriesBySlug.set("ipc-general", empty);

    expect(() => homeIndicatorListing(seriesBySlug)).toThrow(/ipc-general/);
    expect(() => homeIndicatorListing(seriesBySlug)).toThrow(/no observations/);
  });
});
