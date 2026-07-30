// design.md D-1's anti-drift device: the SAME Zod schemas the real loader
// will use validate the Go-generated golden fixture (slice 3's
// `export --fixture`). tasks.md 5.13 (RED/GREEN, wired as early as both
// sides exist rather than deferred to slice 9).
//
// Slice 9a regenerated this fixture (`go test -run
// TestGenerateSlice9aExportFixture -v ./app/internal/ingestion/...`,
// a temporary local generator deleted after running, mirroring slice 3's
// own throwaway-container precedent) to carry REAL ingested data for all
// three slugs that slice built pages for -- tasa-de-paro-epa, ocupados-epa,
// poblacion-residente. Slice 9b regenerated it AGAIN (`TestGenerateSlice9bExportFixture`,
// same throwaway-generator convention, deleted after running) to add the
// remaining three: ipc-general, ipc-subyacente and pib-cvi (the Go
// pipeline's own slug for the page this project's frozen route table calls
// `pib` -- see content/indicators/pib.ts's `artifactSlug` doc comment).
//
// verify-report WARNING-6 regenerated it a third time (`go test -run
// TestGenerateFullHistoryExportFixture ./app/cmd/concontexto/...`, same
// throwaway-generator convention, deleted after running) so every series
// carries its REAL live length -- 98 / 98 / 294 / 294 / 125 / 122 -- instead
// of a 3-period slice. The values inside that span are synthesised and
// disclosed as such; see `web/test/fixtures/export/source.txt` and the
// provenance test at the bottom of this file.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  ManifestSchema,
  parseManifest,
  parseSeriesDoc,
  SeriesDocSchema,
  SUPPORTED_SCHEMA_VERSION,
} from "../../src/lib/export/schema";

const FIXTURES_DIR = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../fixtures/export",
);

function readFixture(relativePath: string): unknown {
  return JSON.parse(readFileSync(path.join(FIXTURES_DIR, relativePath), "utf-8"));
}

describe("export artifact Zod schemas validate the slice-3 golden fixture", () => {
  it("parses manifest.json", () => {
    const manifest = readFixture("manifest.json");
    const parsed = parseManifest(manifest);
    // All six frozen slugs are now present -- asserting membership
    // (`arrayContaining`), not exact ordering, since `manifest.series`'s own
    // order is a Go map-iteration artifact, not a contract.
    expect(parsed.series).toEqual(
      expect.arrayContaining([
        "tasa-de-paro-epa",
        "ocupados-epa",
        "poblacion-residente",
        "ipc-general",
        "ipc-subyacente",
        "pib-cvi",
      ]),
    );
    expect(parsed.series).toHaveLength(6);
    expect(Object.keys(parsed.digests).sort()).toEqual([
      "series/ipc-general.json",
      "series/ipc-subyacente.json",
      "series/ocupados-epa.json",
      "series/pib-cvi.json",
      "series/poblacion-residente.json",
      "series/tasa-de-paro-epa.json",
    ]);
  });

  it("parses series/tasa-de-paro-epa.json", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json");
    const parsed = parseSeriesDoc(doc);
    expect(parsed.slug).toBe("tasa-de-paro-epa");
    expect(parsed.points.length).toBeGreaterThan(0);
  });

  it("resolves every point's provenance via the vintages lookup with zero further data (design.md D-1)", () => {
    const doc = parseSeriesDoc(readFixture("series/tasa-de-paro-epa.json"));
    for (const point of doc.points) {
      const provenance = doc.vintages[String(point.ingestionRunId)];
      expect(provenance, `no vintages entry for ingestionRunId ${point.ingestionRunId}`).toBeDefined();
      // The SCHEMA deliberately accepts any non-empty string here (see
      // schema.ts's VintageProvenanceSchema comment, written when the
      // slice-3 fixture carried a human-readable placeholder). This
      // FIXTURE, however, is now written by the real pipeline against a
      // real filestore, so its digests are genuine — assert the stronger
      // property the fixture actually has, rather than the weaker one the
      // schema permits.
      expect(provenance.rawFileSha256).toMatch(/^[0-9a-f]{64}$/);
    }
  });

  it("rejects a schema_version other than the one supported — exact equality, not >=", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    expect(() => parseSeriesDoc({ ...doc, schema_version: 2 })).toThrow();
    expect(() => parseSeriesDoc({ ...doc, schema_version: 0 })).toThrow();
  });

  it("rejects a manifest missing a required field", () => {
    const manifest = readFixture("manifest.json") as Record<string, unknown>;
    const { generated_at: _generated_at, ...withoutGeneratedAt } = manifest;
    expect(() => ManifestSchema.parse(withoutGeneratedAt)).toThrow();
  });

  it("rejects a series point with an invalid status value", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    const points = doc.points as Record<string, unknown>[];
    const corrupted = { ...doc, points: [{ ...points[0], status: "X" }] };
    expect(() => SeriesDocSchema.parse(corrupted)).toThrow();
  });
});

// verify-report CRITICAL-4 remediation. The artifact now carries the page
// state PRD §6.1.3's banners are driven by, so the schema is where that
// contract is enforced: the field is REQUIRED (a fixture without it must
// fail the loader, not silently render a fresh page), and the three arms'
// cross-field rules are structural, not conventional.
describe("export artifact schema — the page state that drives PRD §6.1.3's banners", () => {
  const ALL_SIX_FIXTURES = [
    "series/tasa-de-paro-epa.json",
    "series/ocupados-epa.json",
    "series/poblacion-residente.json",
    "series/ipc-general.json",
    "series/ipc-subyacente.json",
    "series/pib-cvi.json",
  ] as const;

  it.each(ALL_SIX_FIXTURES)("%s carries a parsed pageState", (relativePath) => {
    const parsed = parseSeriesDoc(readFixture(relativePath));
    expect(parsed.pageState.kind).toBe("fresh");
    expect(parsed.pageState.lastCorrectUpdate).toBeNull();
    expect(parsed.pageState.successorSlug).toBeNull();
  });

  it("REJECTS a series document with no pageState at all — the field is required, never defaulted — mutation-checked", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    const { pageState: _pageState, ...withoutPageState } = doc;
    expect(() => parseSeriesDoc(withoutPageState)).toThrow();
  });

  it("rejects an unknown pageState kind", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    expect(() =>
      parseSeriesDoc({ ...doc, pageState: { kind: "stale", lastCorrectUpdate: null, successorSlug: null } }),
    ).toThrow();
  });

  // The Go half's documented contract: `lastCorrectUpdate` is non-null ONLY
  // on the validation-failure arm and `successorSlug` ONLY on the
  // discontinued arm. Enforcing that structurally means a page can never
  // render a banner from a field its own state has no claim to.
  it("rejects a lastCorrectUpdate on any arm other than validation-failure", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    expect(() =>
      parseSeriesDoc({ ...doc, pageState: { kind: "fresh", lastCorrectUpdate: "2026-07-29", successorSlug: null } }),
    ).toThrow();
    expect(() =>
      parseSeriesDoc({
        ...doc,
        pageState: { kind: "discontinued", lastCorrectUpdate: "2026-07-29", successorSlug: "ocupados-epa" },
      }),
    ).toThrow();
  });

  it("rejects a successorSlug on any arm other than discontinued", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    expect(() =>
      parseSeriesDoc({ ...doc, pageState: { kind: "fresh", lastCorrectUpdate: null, successorSlug: "ocupados-epa" } }),
    ).toThrow();
    expect(() =>
      parseSeriesDoc({
        ...doc,
        pageState: { kind: "validation-failure", lastCorrectUpdate: "2026-07-29", successorSlug: "ocupados-epa" },
      }),
    ).toThrow();
  });

  it("requires lastCorrectUpdate to be a bare YYYY-MM-DD calendar date, since the banner prints it verbatim", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    expect(() =>
      parseSeriesDoc({
        ...doc,
        pageState: { kind: "validation-failure", lastCorrectUpdate: "2026-07-29T11:04:00Z", successorSlug: null },
      }),
    ).toThrow();
    expect(() =>
      parseSeriesDoc({
        ...doc,
        pageState: { kind: "validation-failure", lastCorrectUpdate: "29/07/2026", successorSlug: null },
      }),
    ).toThrow();
  });

  // THE load-bearing edge case the Go half deliberately emits: a series whose
  // very FIRST ingestion run failed validation has no correct update to name.
  // The Go side refuses to substitute a date it does not have (principle P4)
  // and refuses to downgrade the state to "fresh" (which would suppress a
  // recorded failure), so the schema MUST accept this combination — the web
  // layer, not the schema, owns wording that names no date.
  it("ACCEPTS validation-failure with a null lastCorrectUpdate (a first-ever run that failed validation)", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    const parsed = parseSeriesDoc({
      ...doc,
      pageState: { kind: "validation-failure", lastCorrectUpdate: null, successorSlug: null },
    });
    expect(parsed.pageState).toEqual({ kind: "validation-failure", lastCorrectUpdate: null, successorSlug: null });
  });

  it("ACCEPTS discontinued with a null successorSlug (no successor exists)", () => {
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    const parsed = parseSeriesDoc({
      ...doc,
      pageState: { kind: "discontinued", lastCorrectUpdate: null, successorSlug: null },
    });
    expect(parsed.pageState.kind).toBe("discontinued");
    expect(parsed.pageState.successorSlug).toBeNull();
  });

  it("still supports exactly schema_version 1 — this field was added without a version bump", () => {
    expect(SUPPORTED_SCHEMA_VERSION).toBe(1);
    const doc = readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>;
    expect(doc.schema_version).toBe(1);
  });
});

// verify-report BLOCKING-1 remediation. `breaks` and `events` were
// `z.array(z.unknown())` — structurally present, semantically empty. Two
// consequences, both real rather than theoretical:
//
//  1. `IndicatorPage.astro` consumes both arrays field by field
//     (`b.date`, `b.noteMd`, `e.group`, `e.dateEnd`, ...), so every one of
//     those accesses was a ts18046 "'b' is of type 'unknown'" error — the 12
//     errors that made `astro check` fail as a blocking CI gate.
//  2. Nothing validated the two sections at all. The loader's entire
//     contract is "an unsupported version or a shape mismatch MUST fail the
//     build loudly"; `z.array(z.unknown())` accepts `[{}]`, `[null]`, or a
//     break with no `noteMd`, and the page then renders `undefined` into
//     reader-facing prose.
//
// The authoritative shapes are `BreakRef` and `EventRef` in
// `app/internal/publishing/artifact.go`. These tests pin BOTH directions:
// what the Go writer really emits must parse, and what it can never emit
// must be REJECTED.
//
// The six golden fixtures all carry `breaks: []` / `events: []` (no
// configured series resolves a break or event yet), so these cases inject
// synthetic sections into a real fixture document. That is deliberate: it
// exercises the two sections independently of whatever the fixture happens
// to contain, so regenerating the fixture cannot silently turn these
// assertions vacuous.
describe("export artifact schema — the break and event sections (Go BreakRef/EventRef)", () => {
  function docWith(sections: Record<string, unknown>): Record<string, unknown> {
    return { ...(readFixture("series/tasa-de-paro-epa.json") as Record<string, unknown>), ...sections };
  }

  // Every field the Go writer emits for a NON-nil `SourceURL`.
  const WELL_FORMED_BREAK = {
    key: "epa-2021-metodologia",
    date: "2021-01-01",
    kind: "metodologico",
    noteMd: "El INE cambió el cuestionario de la EPA.",
    sourceUrl: "https://www.ine.es/nota.pdf",
  } as const;

  const WELL_FORMED_EVENT = {
    id: "gobierno-2018",
    group: "governments",
    name: "Gobierno de coalición",
    dateStart: "2018-06-02",
    dateEnd: "2023-11-16",
    noteMd: "Nota editorial.",
  } as const;

  it("parses a well-formed break with every field present", () => {
    const parsed = parseSeriesDoc(docWith({ breaks: [WELL_FORMED_BREAK] }));
    expect(parsed.breaks).toEqual([WELL_FORMED_BREAK]);
  });

  it("parses a well-formed event with every field present", () => {
    const parsed = parseSeriesDoc(docWith({ events: [WELL_FORMED_EVENT] }));
    expect(parsed.events).toEqual([WELL_FORMED_EVENT]);
  });

  // `SourceURL *string \`json:"sourceUrl,omitempty"\`` — a nil pointer OMITS
  // the key entirely, it never emits `null`. Same for EventRef's `dateEnd`
  // and `noteMd`. The schema therefore marks them `.optional()`, and these
  // two cases are what proves the optionality is real and not decorative:
  // the overwhelmingly common artifact shape is the one with the keys
  // absent, and a schema that required them would fail every real build.
  it("parses a break whose optional sourceUrl key is ABSENT, the way Go's omitempty writes it", () => {
    const { sourceUrl: _sourceUrl, ...withoutSourceUrl } = WELL_FORMED_BREAK;
    const parsed = parseSeriesDoc(docWith({ breaks: [withoutSourceUrl] }));
    expect(parsed.breaks[0].sourceUrl).toBeUndefined();
  });

  it("parses an event whose optional dateEnd and noteMd keys are ABSENT (an open-ended government)", () => {
    const { dateEnd: _dateEnd, noteMd: _noteMd, ...openEnded } = WELL_FORMED_EVENT;
    const parsed = parseSeriesDoc(docWith({ events: [openEnded] }));
    expect(parsed.events[0].dateEnd).toBeUndefined();
    expect(parsed.events[0].noteMd).toBeUndefined();
  });

  it.each([
    ["a break with no key at all", { ...structuredClone(WELL_FORMED_BREAK), key: undefined }],
    ["a break with an empty key — Go's validateBreaks rejects this too", { ...WELL_FORMED_BREAK, key: "" }],
    ["a break with no noteMd, which the page prints verbatim", { ...structuredClone(WELL_FORMED_BREAK), noteMd: undefined }],
    ["a break whose date is a series period, not a calendar day", { ...WELL_FORMED_BREAK, date: "2021-Q1" }],
    ["a break whose date is a full RFC 3339 instant", { ...WELL_FORMED_BREAK, date: "2021-01-01T00:00:00Z" }],
    ["a break whose sourceUrl is a number", { ...WELL_FORMED_BREAK, sourceUrl: 7 }],
    ["a break that is not an object at all", null],
  ])("REJECTS %s", (_label, malformedBreak) => {
    expect(() => parseSeriesDoc(docWith({ breaks: [malformedBreak] }))).toThrow();
  });

  it.each([
    ["an event with no id at all", { ...structuredClone(WELL_FORMED_EVENT), id: undefined }],
    ["an event with an empty id — Go's validateEvents rejects this too", { ...WELL_FORMED_EVENT, id: "" }],
    ["an event with no name, which the page prints verbatim", { ...structuredClone(WELL_FORMED_EVENT), name: undefined }],
    ["an event whose dateStart is a series period, not a calendar day", { ...WELL_FORMED_EVENT, dateStart: "2018-Q2" }],
    ["an event whose dateEnd is a series period", { ...WELL_FORMED_EVENT, dateEnd: "2023-Q4" }],
    ["an event that is not an object at all", null],
  ])("REJECTS %s", (_label, malformedEvent) => {
    expect(() => parseSeriesDoc(docWith({ events: [malformedEvent] }))).toThrow();
  });

  // THE case that motivates typing `group` as an enum rather than a bare
  // string, even though Go's `EventRef.Group` is a plain `string` and
  // nothing on the Go side constrains its value (config/loader.go copies it
  // straight out of `eventos.yaml`, defaulting only gobiernos.yaml's).
  //
  // `ChartIsland.svelte`'s `groupedAnnotations` iterates the three KNOWN
  // groups and filters each one's entries out of `annotations`; an event
  // carrying any fourth value matches no group and is silently dropped — an
  // editorial event that the artifact records and the page never shows,
  // with no error anywhere. A typo in `eventos.yaml` ("exogenus") is
  // exactly how that happens. Failing the build here is the whole point of
  // validating at the loader boundary, and it is also what lets
  // `IndicatorPage.astro` stop asserting the union with an `as` cast.
  it("REJECTS an event whose group is not one of the three the page can render", () => {
    expect(() => parseSeriesDoc(docWith({ events: [{ ...WELL_FORMED_EVENT, group: "exogenus" }] }))).toThrow();
  });

  it.each(["governments", "exogenous", "milestones"])("parses the renderable group %s", (group) => {
    const parsed = parseSeriesDoc(docWith({ events: [{ ...WELL_FORMED_EVENT, group }] }));
    expect(parsed.events[0].group).toBe(group);
  });

  // Go's `validateBreaks`/`validateEvents` (validate.go) reject a duplicated
  // key/id as "an unresolved (ambiguous) reference" before the artifact is
  // ever written. Mirroring that here is not redundant belt-and-braces: the
  // island renders both sections with a KEYED `{#each ... (b.key)}`, and
  // Svelte throws at runtime on duplicate keys. A drifted writer would
  // otherwise turn a build-time rejection into a client-side crash on a
  // hydrated page.
  it("REJECTS a duplicated break key, matching Go's validateBreaks unresolved-reference rule", () => {
    expect(() =>
      parseSeriesDoc(docWith({ breaks: [WELL_FORMED_BREAK, { ...WELL_FORMED_BREAK, noteMd: "Otra nota." }] })),
    ).toThrow();
  });

  it("REJECTS a duplicated event id, matching Go's validateEvents unresolved-reference rule", () => {
    expect(() =>
      parseSeriesDoc(docWith({ events: [WELL_FORMED_EVENT, { ...WELL_FORMED_EVENT, name: "Otro nombre." }] })),
    ).toThrow();
  });

  it("still accepts the empty sections every current fixture carries", () => {
    const parsed = parseSeriesDoc(docWith({ breaks: [], events: [] }));
    expect(parsed.breaks).toEqual([]);
    expect(parsed.events).toEqual([]);
  });
});

// verify-report WARNING-6 closure. The fixture's history is long enough to
// exercise production behaviour (range presets, year-on-year), which was only
// possible by SYNTHESISING the values that precede each series' real,
// live-captured tail. This project never presents synthesised values as real,
// so the disclosure is part of the fixture, and this test is what stops the
// disclosure from silently disappearing on a future regeneration.
describe("export fixture — the synthesised history is disclosed in plain words", () => {
  it("web/test/fixtures/export carries a provenance note stating that most values are synthesised", () => {
    const note = readFileSync(path.join(FIXTURES_DIR, "source.txt"), "utf-8");
    expect(note).toContain("SYNTHESISED");
    expect(note).toContain("They were never published by INE");
    expect(note).toContain("MUST NOT be quoted");
    // It must also say which part IS real, or the disclosure is useless.
    expect(note).toContain("WHAT IS REAL HERE");
    expect(note).toContain("WHAT IS SYNTHESISED HERE");
    // And it must be reproducible from the note alone: the recipe and the
    // per-series constants are named, not merely alluded to.
    expect(note).toContain("HOW IT WAS GENERATED");
    for (const slug of [
      "tasa-de-paro-epa",
      "ocupados-epa",
      "ipc-general",
      "ipc-subyacente",
      "pib-cvi",
      "poblacion-residente",
    ]) {
      expect(note, `the provenance note names no constants for ${slug}`).toContain(slug);
    }
  });
});
