// indicator-page spec — tasks.md 9a.3 (RED, page anatomy), 9a.5 (RED,
// freshness semaphore), 9a.7 (RED, three page states), 9a.9 (RED,
// methodology traceability), 9a.11 (RED, no-inlined-copy scan). Drives
// `IndicatorPage.astro` directly via `experimental_AstroContainer` with
// real artifact data (loaded through the slice-9a loader, exercising the
// same golden fixture the real build reads) plus synthetic overrides for
// states the checked-in fixture does not itself carry (source-pending
// freshness, validation-failure, discontinued).
import { describe, expect, it } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import svelteServerRenderer from "@astrojs/svelte/server.js";
import path from "node:path";
import { globSync } from "node:fs";
import { fileURLToPath } from "node:url";
import IndicatorPage, { type RelatedCardData } from "../../src/templates/IndicatorPage.astro";
import { loadExportArtifact } from "../../src/lib/export/loader";
import { INDICATOR_CONTENT } from "../../src/content/indicators";
import { METHODOLOGY_CONTENT } from "../../src/content/indicators/methodology";
import type { SeriesDoc } from "../../src/lib/export/schema";

const FIXTURES_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../fixtures/export");

// All six frozen slugs (indicator-page spec). `pib`'s own export document is
// slugged `pib-cvi` in the fixture (see content/indicators/pib.ts's
// `artifactSlug` doc comment) — `docFor` below resolves that alias exactly
// like `[slug].astro`'s own `artifactSlugFor` does, so these tests exercise
// real per-page data for all six, not just the three slice 9a shipped.
const ALL_SIX_SLUGS = [
  "tasa-de-paro-epa",
  "ocupados-epa",
  "poblacion-residente",
  "ipc-general",
  "ipc-subyacente",
  "pib",
] as const;

function docFor(slug: string, seriesBySlug: Map<string, SeriesDoc>): SeriesDoc {
  const artifactSlug = INDICATOR_CONTENT[slug]?.artifactSlug ?? slug;
  const doc = seriesBySlug.get(artifactSlug);
  if (!doc) throw new Error(`no fixture document for slug "${slug}" (artifact slug "${artifactSlug}")`);
  return doc;
}

async function loadFixtureArtifact() {
  return loadExportArtifact({ dir: FIXTURES_DIR });
}

/** The rendered TEXT of the element carrying `data-testid`, or `null` when
 * no such element exists. Attribute-presence assertions pass while the
 * figure inside is a placeholder dash (verify-report's own assertion-quality
 * finding against `page-yoy-variation`); reading the text is what makes the
 * assertion about the number rather than about the markup. */
function renderedText(html: string, testId: string): string | null {
  const match = new RegExp(`<(\\w+)[^>]*data-testid="${testId}"[^>]*>([\\s\\S]*?)</\\1>`).exec(html);
  return match ? match[2].replace(/<[^>]+>/g, "").replace(/\s+/g, " ").trim() : null;
}

function relatedCardsFor(slugs: string[], seriesBySlug: Map<string, SeriesDoc>): RelatedCardData[] {
  return slugs.map((slug) => {
    const doc = seriesBySlug.get(slug);
    const content = INDICATOR_CONTENT[slug];
    const latest = doc?.points[doc.points.length - 1] ?? null;
    return {
      slug,
      name: content?.name ?? slug,
      unit: content?.unit ?? doc?.unit ?? "",
      latestValue: latest?.value ?? null,
      latestPeriod: latest?.period ?? "—",
      freshness: doc?.freshness ?? "source-pending",
      decimals: content?.decimals,
    };
  });
}

describe("IndicatorPage — page anatomy (task 9a.3)", () => {
  it("renders header, chart, action bar, methodology sheet and related-indicators sections, in order, for a real artifact-backed page", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = seriesBySlug.get("tasa-de-paro-epa")!;
    const content = INDICATOR_CONTENT["tasa-de-paro-epa"];
    const methodology = METHODOLOGY_CONTENT["tasa-de-paro-epa"];

    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content,
        methodology,
        relatedCards: relatedCardsFor(methodology.relatedSlugs, seriesBySlug),
        canonicalPath: "/indicador/tasa-de-paro-epa",
      },
    });

    const order = [
      'data-testid="page-header"',
      'data-testid="page-chart-section"',
      'data-testid="action-bar"',
      'data-testid="methodology-sheet"',
      'data-testid="related-indicators"',
    ];
    const indices = order.map((marker) => html.indexOf(marker));
    for (const index of indices) expect(index).toBeGreaterThan(-1);
    for (let i = 1; i < indices.length; i++) {
      expect(indices[i]).toBeGreaterThan(indices[i - 1]);
    }
  });

  it("the header carries the latest value, its period and both variation figures — no free prose", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = seriesBySlug.get("tasa-de-paro-epa")!;
    const content = INDICATOR_CONTENT["tasa-de-paro-epa"];
    const methodology = METHODOLOGY_CONTENT["tasa-de-paro-epa"];

    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: { doc, content, methodology, relatedCards: [], canonicalPath: "/indicador/tasa-de-paro-epa" },
    });

    const headerHtml = html.slice(html.indexOf('data-testid="page-header"'), html.indexOf('data-testid="page-chart-section"'));
    expect(headerHtml).toContain('data-testid="page-latest-value"');
    expect(headerHtml).toContain('data-testid="page-latest-period"');
    expect(headerHtml).toContain('data-testid="page-yoy-variation"');
    expect(headerHtml).toContain('data-testid="page-intra-annual-variation"');
    expect(headerHtml).toContain("2026-Q2"); // this fixture's latest period
    expect(headerHtml).toContain("9.87"); // this fixture's latest value
    // The latest value and period above are REAL published INE figures —
    // the newest three observations of every series in the fixture are
    // verbatim copies of the live-verified response (see
    // `web/test/fixtures/export/source.txt`).
  });

  // verify-report WARNING-6: the previous fixture carried three observations
  // per series, so both variation figures rendered `—` (no same-period-
  // prior-year observation existed) and the only covering assertion was
  // attribute-presence, which passed while the figure was a dash. With a
  // full-length history the figures resolve, so the test now reads the
  // rendered TEXT of both `<dd>` elements and requires a signed percentage.
  it("both header variation figures resolve to a real signed percentage, never the not-available dash, on every page", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: {
          doc: docFor(slug, seriesBySlug),
          content: INDICATOR_CONTENT[slug],
          methodology: METHODOLOGY_CONTENT[slug],
          relatedCards: [],
          canonicalPath: `/indicador/${slug}`,
        },
      });

      for (const testId of ["page-yoy-variation", "page-intra-annual-variation"]) {
        const rendered = renderedText(html, testId);
        expect(rendered, `${slug}: no <dd data-testid="${testId}"> found`).not.toBeNull();
        expect(rendered, `${slug}: ${testId} rendered the not-available dash`).not.toBe("—");
        expect(rendered, `${slug}: ${testId} is not a signed percentage`).toMatch(/^[+-]\d+(\.\d+)?%$/);
      }
    }
  });

  it("3–5 related cards resolve to slugs within the frozen six-slug catalog (disclosed: not every card is a page built THIS slice, per methodology.ts's own scope note)", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = seriesBySlug.get("tasa-de-paro-epa")!;
    const methodology = METHODOLOGY_CONTENT["tasa-de-paro-epa"];
    expect(methodology.relatedSlugs.length).toBeGreaterThanOrEqual(3);
    expect(methodology.relatedSlugs.length).toBeLessThanOrEqual(5);
    for (const slug of methodology.relatedSlugs) {
      expect(Object.keys(INDICATOR_CONTENT)).toContain(slug);
    }

    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT["tasa-de-paro-epa"],
        methodology,
        relatedCards: relatedCardsFor(methodology.relatedSlugs, seriesBySlug),
        canonicalPath: "/indicador/tasa-de-paro-epa",
      },
    });
    for (const slug of methodology.relatedSlugs) {
      expect(html).toContain(`href="/indicador/${slug}"`);
    }
  });
});

// verify-report SUGGESTION-13. The template composes `MethodologySheet` with
// a `slug` prop that becomes a DOM id (`methodology-heading-${slug}`, the
// target of the section's own `aria-labelledby`). It used to pass
// `doc.slug` — the ARTIFACT's series slug — so `/indicador/pib` rendered
// `id="methodology-heading-pib-cvi"`.
//
// Not reader-visible, and not a rendering defect: the id is internally
// consistent and the landmark resolves either way. It is an identifier-space
// mistake, and this project keeps the two spaces genuinely separate (see
// `routeSlugForArtifactSlug`'s doc comment, and `[slug].astro`'s). Everything
// else on the page that identifies WHICH PAGE this is — `data-slug`, the
// chart island's `slug`, every id derived from it — already uses the route
// slug; the methodology heading was the one holdout, so an id built from it
// silently belonged to a different namespace than its neighbours.
//
// Why `content.slug` and NOT `routeSlugForArtifactSlug(doc.slug)`: the
// template is already HANDED the route slug as a prop. The reverse lookup
// exists for artifact-supplied slugs with no other provenance — the
// discontinued state's `successorSlug` — and it can return `null`, which
// would force this call site to invent a fallback for a value it already
// holds with certainty. Using the prop is both simpler and more truthful
// about where the route slug comes from.
// The chart island's break-list heading is level-appropriate for the
// WORKBENCH by default (it sits under an `<h3>` component-demo heading there,
// so `<h4>` is correct), but an indicator page gives the chart section no
// heading of its own — it is labelled with `aria-label`. On a page, therefore,
// the island's heading follows the `<h1>` page title directly, and a hard-coded
// `<h4>` skips two levels.
//
// This was invisible until the export fixture began carrying REAL breaks: the
// whole `{#if breakDisplay.length > 0}` block never rendered while every
// series had `breaks: []`, so the axe `heading-order` gate had nothing to
// judge. It fails on exactly the four series that now resolve a break, and
// passes on the two that do not — which is what identifies the heading level,
// not the breaks themselves, as the defect.
//
// The level is a property of the CONTEXT, not of the component, so the context
// supplies it rather than the component guessing. The workbench keeps the
// default.
describe("IndicatorPage — the chart island's break heading does not skip levels (axe heading-order)", () => {
  it("renders the break list under an h2, directly below the page h1, on a series that has breaks", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = docFor("tasa-de-paro-epa", seriesBySlug);
    expect(doc.breaks.length, "this assertion is vacuous unless the fixture carries a break").toBeGreaterThan(0);

    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT["tasa-de-paro-epa"],
        methodology: METHODOLOGY_CONTENT["tasa-de-paro-epa"],
        relatedCards: [],
        canonicalPath: "/indicador/tasa-de-paro-epa",
      },
    });

    // The FIRST heading after the break list opens — matched on the tag alone
    // rather than on the surrounding bytes, because Svelte's `<svelte:element>`
    // emits an anchor comment (`<!---->`) immediately before the element and
    // the exact marker is an implementation detail, not a contract.
    const breaksBlock = html.slice(html.indexOf('data-testid="chart-breaks"'));
    const firstHeadingLevel = /<h([1-6])[\s>]/.exec(breaksBlock)?.[1];
    expect(firstHeadingLevel, "no heading found inside the break list").toBeDefined();
    expect(firstHeadingLevel, `the break list heading is <h${firstHeadingLevel}>, which skips levels after the page <h1>`).toBe("2");
  });

  // The real gate is axe's `heading-order` rule in the e2e suite; this is its
  // fast unit-level equivalent, and it reads the page as a whole rather than
  // one block, so a future heading added anywhere between the h1 and the break
  // list is also caught.
  it("no heading on any indicator page increases by more than one level", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: {
          doc: docFor(slug, seriesBySlug),
          content: INDICATOR_CONTENT[slug],
          methodology: METHODOLOGY_CONTENT[slug],
          relatedCards: [],
          canonicalPath: `/indicador/${slug}`,
        },
      });

      const levels = [...html.matchAll(/<h([1-6])[\s>]/g)].map((m) => Number(m[1]));
      expect(levels.length, `${slug}: no headings found at all`).toBeGreaterThan(0);
      for (let i = 1; i < levels.length; i++) {
        expect(
          levels[i] - levels[i - 1],
          `${slug}: heading order jumps from h${levels[i - 1]} to h${levels[i]} (sequence: ${levels.join(" → ")})`,
        ).toBeLessThanOrEqual(1);
      }
    }
  });
});

describe("IndicatorPage — the methodology heading id lives in the ROUTE slug space (verify-report SUGGESTION-13)", () => {
  it.each(ALL_SIX_SLUGS)("%s renders methodology-heading-<route slug>", async (slug) => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = docFor(slug, seriesBySlug);
    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT[slug],
        methodology: METHODOLOGY_CONTENT[slug],
        relatedCards: [],
        canonicalPath: `/indicador/${slug}`,
      },
    });
    expect(html).toContain(`id="methodology-heading-${slug}"`);
    expect(html).toContain(`aria-labelledby="methodology-heading-${slug}"`);
  });

  // The assertion above is only load-bearing for the ONE slug whose two
  // identifier spaces actually diverge. Stating that divergence explicitly
  // keeps the case from quietly becoming vacuous if `pib`'s `artifactSlug`
  // is ever removed, and names the exact string the bug produced.
  it("pib is the case that discriminates: its artifact slug is pib-cvi, and that must NOT reach the DOM id", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = docFor("pib", seriesBySlug);
    expect(doc.slug).toBe("pib-cvi");
    expect(INDICATOR_CONTENT["pib"].slug).toBe("pib");

    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT["pib"],
        methodology: METHODOLOGY_CONTENT["pib"],
        relatedCards: [],
        canonicalPath: "/indicador/pib",
      },
    });
    expect(html).not.toContain("methodology-heading-pib-cvi");
  });

  // The artifact slug is still correct — and still required — for the two
  // download hrefs, which must resolve to the real PUBLISHED filenames
  // (`/data-derived/series/pib-cvi.json`). Pinning that here means the
  // SUGGESTION-13 fix cannot be "corrected" into a route slug everywhere and
  // silently break both downloads on the pib page.
  it("the CSV and JSON hrefs still use the ARTIFACT slug, because that is the published filename", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = docFor("pib", seriesBySlug);
    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT["pib"],
        methodology: METHODOLOGY_CONTENT["pib"],
        relatedCards: [],
        canonicalPath: "/indicador/pib",
      },
    });
    expect(html).toContain('href="/data-derived/csv/pib-cvi.csv"');
    expect(html).toContain('href="/data-derived/series/pib-cvi.json"');
  });
});

describe("IndicatorPage — freshness semaphore (task 9a.5/9a.6)", () => {
  it("a fresh artifact renders the green/fresh state", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = seriesBySlug.get("tasa-de-paro-epa")!;
    expect(doc.freshness).toBe("fresh"); // this fixture's real, currently-ingested state

    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT["tasa-de-paro-epa"],
        methodology: METHODOLOGY_CONTENT["tasa-de-paro-epa"],
        relatedCards: [],
        canonicalPath: "/indicador/tasa-de-paro-epa",
      },
    });
    expect(html).toContain('data-testid="freshness-fresh"');
  });

  it("a source-pending artifact renders amber with the EXACT spec-mandated label, with no network-freshness element", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const pendingDoc: SeriesDoc = { ...seriesBySlug.get("tasa-de-paro-epa")!, freshness: "source-pending" };

    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc: pendingDoc,
        content: INDICATOR_CONTENT["tasa-de-paro-epa"],
        methodology: METHODOLOGY_CONTENT["tasa-de-paro-epa"],
        relatedCards: [],
        canonicalPath: "/indicador/tasa-de-paro-epa",
      },
    });
    expect(html).toContain('data-testid="freshness-source-pending"');
    expect(html).toContain("Pendiente de actualización por la fuente");
    // No "page older than data" element and no client-side freshness script.
    expect(html.toLowerCase()).not.toContain("desactualizada");
    expect(html).not.toContain("fetch(");
  });
});

// verify-report CRITICAL-4 remediation. These tests previously drove the
// banner by overriding `methodology.pageState` — a hand-maintained constant
// that made the requirement a rendering proof with no data path. Every case
// below now overrides the ARTIFACT's own `doc.pageState`, which is what the
// live site actually reads, so a real validation failure recorded by the Go
// pipeline reaches the page with no source edit and no redeploy.
describe("IndicatorPage — three page states (task 9a.7/9a.8, rewired to the artifact by CRITICAL-4)", () => {
  function withPageState(doc: SeriesDoc, pageState: SeriesDoc["pageState"]): SeriesDoc {
    return { ...doc, pageState };
  }

  async function renderWithPageState(pageState: SeriesDoc["pageState"], slug = "tasa-de-paro-epa"): Promise<string> {
    const { seriesBySlug } = await loadFixtureArtifact();
    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    return container.renderToString(IndicatorPage, {
      props: {
        doc: withPageState(docFor(slug, seriesBySlug), pageState),
        content: INDICATOR_CONTENT[slug],
        methodology: METHODOLOGY_CONTENT[slug],
        relatedCards: [],
        canonicalPath: `/indicador/${slug}`,
      },
    });
  }

  it("the six real fixture documents all carry a fresh state and render no banner", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const doc = docFor(slug, seriesBySlug);
      expect(doc.pageState.kind, `${slug}: fixture page state`).toBe("fresh");

      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: {
          doc,
          content: INDICATOR_CONTENT[slug],
          methodology: METHODOLOGY_CONTENT[slug],
          relatedCards: [],
          canonicalPath: `/indicador/${slug}`,
        },
      });
      expect(html, `${slug}`).not.toContain('data-testid="page-banner-validation-failure"');
      expect(html, `${slug}`).not.toContain('data-testid="page-banner-discontinued"');
    }
  });

  it("a validation failure recorded in the ARTIFACT renders the EXACT spec-mandated banner, and the chart is still present — mutation-checked", async () => {
    const html = await renderWithPageState({
      kind: "validation-failure",
      lastCorrectUpdate: "2026-07-29",
      successorSlug: null,
    });
    expect(html).toContain(
      "Última actualización correcta: 2026-07-29. La fuente ha publicado un dato que no ha superado nuestra validación automática; estamos revisándolo",
    );
    expect(html).toContain('data-testid="page-banner-validation-failure"');
    expect(html).toContain('data-testid="page-chart-section"');
  });

  // The load-bearing edge case: the Go half emits a validation failure with
  // NO last-correct-update date when a series' very first ingestion run
  // failed validation. The page must state the failure without naming a date
  // and without implying a previous correct update — and must never leak the
  // literal "null" into reader-facing prose.
  it("a validation failure with NO last-correct-update date renders the dateless banner, naming no date — mutation-checked", async () => {
    const html = await renderWithPageState({
      kind: "validation-failure",
      lastCorrectUpdate: null,
      successorSlug: null,
    });
    expect(html).toContain('data-testid="page-banner-validation-failure"');
    expect(html).toContain("La fuente ha publicado un dato que no ha superado nuestra validación automática");
    expect(html).not.toContain("Última actualización correcta");
    expect(html).not.toContain("Última actualización correcta: null");
    expect(html).toContain('data-testid="page-chart-section"');
  });

  it("a discontinued state renders a permanent banner with a successor link resolving to that indicator's route", async () => {
    const html = await renderWithPageState({
      kind: "discontinued",
      lastCorrectUpdate: null,
      successorSlug: "ocupados-epa",
    });
    expect(html).toContain('data-testid="page-banner-discontinued"');
    expect(html).toContain('data-testid="page-banner-successor-link"');
    expect(html).toContain('href="/indicador/ocupados-epa"');
    // The link names the successor the way a reader knows it — its indicator
    // name — not the pipeline slug (`es.page.discontinuedSuccessorLink`'s own
    // parameter is `name`, and a slug in reader-facing prose is not one).
    expect(html).toContain(INDICATOR_CONTENT["ocupados-epa"].name);
    expect(html).toContain('data-testid="page-chart-section"'); // chart never hidden
  });

  it("a discontinued state with NO successor renders the banner with no dangling link", async () => {
    const html = await renderWithPageState({ kind: "discontinued", lastCorrectUpdate: null, successorSlug: null });
    expect(html).toContain('data-testid="page-banner-discontinued"');
    expect(html).not.toContain('data-testid="page-banner-successor-link"');
    expect(html).not.toContain('href="/indicador/null"');
    expect(html).not.toContain('href="/indicador/undefined"');
    expect(html).toContain('data-testid="page-chart-section"');
  });

  // The successor slug the Go pipeline records is a SERIES slug, which is not
  // always the frozen ROUTE slug: `pib`'s artifact document is slugged
  // `pib-cvi` (see `[slug].astro`'s own doc comment). Linking the raw value
  // would 404 — the spec requires "a successor link is shown", which can only
  // mean a link that resolves.
  it("resolves an artifact series slug to its frozen route slug in the successor link", async () => {
    const html = await renderWithPageState({
      kind: "discontinued",
      lastCorrectUpdate: null,
      successorSlug: "pib-cvi",
    });
    expect(html).toContain('href="/indicador/pib"');
    expect(html).not.toContain('href="/indicador/pib-cvi"');
  });

  it("a successor slug with no built page renders the banner without a link to a page that does not exist", async () => {
    const html = await renderWithPageState({
      kind: "discontinued",
      lastCorrectUpdate: null,
      successorSlug: "serie-que-no-existe",
    });
    expect(html).toContain('data-testid="page-banner-discontinued"');
    expect(html).not.toContain('data-testid="page-banner-successor-link"');
    expect(html).not.toContain("/indicador/serie-que-no-existe");
  });

  // indicator-page spec, Scenario "The chart is never hidden": "GIVEN any of
  // the three page states ... THEN the chart is present and rendered".
  it.each([
    ["fresh", { kind: "fresh", lastCorrectUpdate: null, successorSlug: null }],
    ["validation-failure with a date", { kind: "validation-failure", lastCorrectUpdate: "2026-07-29", successorSlug: null }],
    ["validation-failure with no date", { kind: "validation-failure", lastCorrectUpdate: null, successorSlug: null }],
    ["discontinued with a successor", { kind: "discontinued", lastCorrectUpdate: null, successorSlug: "ocupados-epa" }],
    ["discontinued with no successor", { kind: "discontinued", lastCorrectUpdate: null, successorSlug: null }],
  ] as const)("the chart is never hidden — %s", async (_label, pageState) => {
    const html = await renderWithPageState(pageState as SeriesDoc["pageState"]);
    expect(html).toContain('data-testid="page-chart-section"');
    expect(html).toContain('data-testid="accessible-data-table"');
    expect(html).toContain("<svg");
  });

  // The defect itself, pinned: the page state must have NO hand-maintained
  // representation left in the editorial content module. If one reappears,
  // this fails — a live validation failure would once again need a source
  // edit and a redeploy to reach readers.
  it("no editorial content entry carries a hand-maintained page state any more (CRITICAL-4 regression pin)", () => {
    for (const [slug, methodology] of Object.entries(METHODOLOGY_CONTENT)) {
      expect(Object.keys(methodology), `${slug} still carries an editorial page state`).not.toContain("pageState");
    }
  });
});

describe("IndicatorPage — methodology traceability (task 9a.9, widened to all six pages by task 9b.1)", () => {
  it("every traceability field is present and non-empty, for all six pages", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const doc = docFor(slug, seriesBySlug);
      const content = INDICATOR_CONTENT[slug];
      const methodology = METHODOLOGY_CONTENT[slug];

      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: { doc, content, methodology, relatedCards: [], canonicalPath: `/indicador/${slug}` },
      });

      expect(html).toContain(methodology.measures);
      expect(html).toContain(methodology.doesNotMeasure);
      expect(html).toContain(doc.source.name);
      expect(html).toContain(doc.origin.ref); // origin identifier
      // HTML-escaped attribute value (Astro escapes "&" to "&amp;" in href
      // attributes) — this fixture's own requestUrl carries a query string.
      expect(html).toContain(`href="${doc.origin.requestUrl.replace(/&/g, "&amp;")}"`); // origin link resolves to the source
      expect(html).toContain(doc.vintage.extractedAt); // extraction timestamp
      expect(html).toContain(doc.unit);
      expect(html).toContain("Versión"); // vintage label
      expect(html).toContain(`href="${methodology.ingestionScriptHref}"`); // ingestion-script link
    }
  });

  it("a per-capita series states the denominator attribution rule; a non-per-capita series does not", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();

    const perCapitaContainer = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const perCapitaHtml = await perCapitaContainer.renderToString(IndicatorPage, {
      props: {
        doc: seriesBySlug.get("ocupados-epa")!,
        content: INDICATOR_CONTENT["ocupados-epa"],
        methodology: METHODOLOGY_CONTENT["ocupados-epa"],
        relatedCards: [],
        population: seriesBySlug.get("poblacion-residente")!.points,
        canonicalPath: "/indicador/ocupados-epa",
      },
    });
    expect(perCapitaHtml).toContain("Atribución per cápita");

    const noPerCapitaContainer = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const noPerCapitaHtml = await noPerCapitaContainer.renderToString(IndicatorPage, {
      props: {
        doc: seriesBySlug.get("tasa-de-paro-epa")!,
        content: INDICATOR_CONTENT["tasa-de-paro-epa"],
        methodology: METHODOLOGY_CONTENT["tasa-de-paro-epa"],
        relatedCards: [],
        canonicalPath: "/indicador/tasa-de-paro-epa",
      },
    });
    expect(noPerCapitaHtml).not.toContain("Atribución per cápita");
  });
});

describe("IndicatorPage — action bar hrefs resolve to the artifact's REAL on-disk layout (verify-report CRITICAL-1 remediation)", () => {
  function extractHref(html: string, testId: string): string | null {
    const tagMatch = new RegExp(`<a[^>]*data-testid="${testId}"[^>]*>`).exec(html);
    if (!tagMatch) return null;
    const hrefMatch = /href="([^"]+)"/.exec(tagMatch[0]);
    return hrefMatch ? hrefMatch[1] : null;
  }

  it("csvHref and jsonHref point at paths that actually exist under the artifact's real /data-derived layout, for all six pages", async () => {
    const { existsSync } = await import("node:fs");
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const doc = docFor(slug, seriesBySlug);
      const content = INDICATOR_CONTENT[slug];
      const methodology = METHODOLOGY_CONTENT[slug];

      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: { doc, content, methodology, relatedCards: [], canonicalPath: `/indicador/${slug}` },
      });

      const csvHref = extractHref(html, "action-bar-csv");
      const jsonHref = extractHref(html, "action-bar-json");
      expect(csvHref, `${slug}: no CSV action-bar href found`).not.toBeNull();
      expect(jsonHref, `${slug}: no JSON action-bar href found`).not.toBeNull();

      // Do NOT compare against a second hard-coded string — assert against
      // the SAME golden fixture directory `export --fixture` really writes
      // (this fixture's own `csv/` and `series/` subdirectories), so the web
      // href and the Go writer's real layout cannot silently drift apart
      // again the way they did (CRITICAL-1: csvHref pointed at
      // `/data-derived/{slug}.csv`, the real file is at
      // `/data-derived/csv/{slug}.csv`).
      const csvRelative = csvHref!.replace(/^\/data-derived\//, "");
      const jsonRelative = jsonHref!.replace(/^\/data-derived\//, "");
      expect(
        existsSync(path.join(FIXTURES_DIR, csvRelative)),
        `${slug}: csvHref "${csvHref}" does not exist in the real artifact layout under ${FIXTURES_DIR}`,
      ).toBe(true);
      expect(
        existsSync(path.join(FIXTURES_DIR, jsonRelative)),
        `${slug}: jsonHref "${jsonHref}" does not exist in the real artifact layout under ${FIXTURES_DIR}`,
      ).toBe(true);
    }
  });
});

describe("IndicatorPage — no-inlined-copy scan (verify-report WARNING-18 remediation: widened from a hand-maintained list to every reader-facing source file)", () => {
  // verify-report CRITICAL-3: the ONLY covering test scanned exactly
  // `IndicatorPage.astro` (1 of 10 files that actually render reader-facing
  // text on every one of the six pages). 37 Spanish strings were inlined
  // across the other 9 — including `FreshnessSemaphore.astro` hard-coding
  // the spec's own verbatim "Pendiente de actualización por la fuente"
  // instead of resolving it from `es.ts`. Its remediation replaced the
  // single scanned file with an explicit 11-file list.
  //
  // verify-report WARNING-18: that list was ITSELF hand-maintained, so
  // `index.astro` — never added to it — kept its own inlined Spanish
  // unnoticed. A guard that only looks where it was pointed will keep
  // missing things whenever a new file is added and nobody remembers to
  // extend the list. The fix widens discovery to every `.astro`/`.svelte`
  // file under `src/`, glob-derived at test-run time via `node:fs`'s
  // `globSync` (Node 22, this project's own CI runtime — see `ci.yml` /
  // `ingest-export-build.yml`), so a new file is covered automatically
  // instead of depending on someone remembering to add it here.
  const SRC_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../src");

  function discoverReaderFacingSourceFiles(): string[] {
    return globSync("**/*.{astro,svelte}", { cwd: SRC_DIR }).sort();
  }

  const READER_FACING_SOURCE_FILES = discoverReaderFacingSourceFiles();

  // Discovery itself must not silently degrade to an empty or truncated
  // list (e.g. a `cwd` typo that matches nothing would make every
  // `it.each` below vacuously pass). Pinned to "at least" the 15 files
  // known at the time this scan was widened, not to an exact count, so a
  // genuinely new file does not require touching this test.
  it("discovers at least every currently-known reader-facing source file", () => {
    expect(READER_FACING_SOURCE_FILES.length).toBeGreaterThanOrEqual(15);
    expect(READER_FACING_SOURCE_FILES).toContain("pages/index.astro");
    expect(READER_FACING_SOURCE_FILES).toContain("templates/IndicatorPage.astro");
  });

  it.each(READER_FACING_SOURCE_FILES)(
    "%s has no bare Spanish string literal outside an es.* call or a props/data passthrough",
    async (relativePath) => {
      const { readFile } = await import("node:fs/promises");
      const rawSource = await readFile(path.join(SRC_DIR, relativePath), "utf-8");
      // Strip non-markup code before scanning: Astro's `--- ... ---`
      // frontmatter and (for `.svelte` files) the `<script>...</script>` and
      // `<style>...</style>` blocks. Slice 9a's original single-file version
      // of this scan never needed this (it only ever scanned
      // `IndicatorPage.astro`'s own markup, whose frontmatter happened not to
      // trip the regex) — widening to 11 files, several of which format text
      // nodes across multiple lines (see below), makes stripping necessary to
      // avoid false positives from comparison operators / JS string literals
      // inside frontmatter or `<script>` bodies.
      const isSvelte = relativePath.endsWith(".svelte");
      const template = isSvelte
        ? rawSource.replace(/<script[^>]*>[\s\S]*?<\/script>/g, "").replace(/<style[^>]*>[\s\S]*?<\/style>/g, "")
        : rawSource.replace(/^---\r?\n[\s\S]*?\r?\n---/, "");
      // Every rendered text node in this file is either an expression
      // ({content.name}, {es.xxx}, {methodology.xxx}, {doc.xxx}) or one of
      // the handful of markup-structural characters (":", punctuation) —
      // there is no bare Spanish prose string sitting directly in the
      // markup outside an expression. This regex looks for a run of 4+
      // lowercase Spanish letters (incl. accents) between two tags, allowing
      // the text node to span multiple lines (most of these components
      // format inline text on its own indented line, which slice 9a's
      // original single-line-only regex could not see — the false negative
      // that let CRITICAL-3's 37 strings go undetected), with no `{`/`}`
      // anywhere in the run (which would indicate an expression, not bare
      // text). Works identically for `.astro` and `.svelte` markup, both of
      // which use the same `>text<` / `{expression}` shape.
      const suspicious = [...template.matchAll(/>([^<>{}]{4,})</g)]
        .map((m) => m[1].replace(/\s+/g, " ").trim())
        .filter((text) => text.length > 0 && /[a-záéíóúñ]{4,}/i.test(text));
      expect(suspicious, `${relativePath}: bare text nodes found: ${JSON.stringify(suspicious)}`).toEqual([]);
    },
  );

  it("technical identifiers (slugs, origin refs) render verbatim, not translated", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = seriesBySlug.get("tasa-de-paro-epa")!;
    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT["tasa-de-paro-epa"],
        methodology: METHODOLOGY_CONTENT["tasa-de-paro-epa"],
        relatedCards: [],
        canonicalPath: "/indicador/tasa-de-paro-epa",
      },
    });
    expect(html).toContain("tasa-de-paro-epa"); // slug, verbatim
    expect(html).toContain("EPA453100"); // origin ref, verbatim
  });
});

describe("IndicatorPage — slice 9b: page anatomy repeated for the remaining three slugs (task 9b.1)", () => {
  it("renders header/chart/action-bar/methodology-sheet/related-indicators, in order, for ipc-general, ipc-subyacente and pib", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ["ipc-general", "ipc-subyacente", "pib"]) {
      const doc = docFor(slug, seriesBySlug);
      const content = INDICATOR_CONTENT[slug];
      const methodology = METHODOLOGY_CONTENT[slug];

      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: { doc, content, methodology, relatedCards: [], canonicalPath: `/indicador/${slug}` },
      });

      const order = [
        'data-testid="page-header"',
        'data-testid="page-chart-section"',
        'data-testid="action-bar"',
        'data-testid="methodology-sheet"',
        'data-testid="related-indicators"',
      ];
      const indices = order.map((marker) => html.indexOf(marker));
      for (const index of indices) expect(index, `${slug}: missing ${order[indices.indexOf(index)]}`).toBeGreaterThan(-1);
      for (let i = 1; i < indices.length; i++) expect(indices[i]).toBeGreaterThan(indices[i - 1]);
    }
  });

  it("`pib` additionally shows the quarter-on-quarter variation (series-transformations spec: pib is the one series with BOTH mandatory rates)", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const doc = docFor("pib", seriesBySlug);
    const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
    const html = await container.renderToString(IndicatorPage, {
      props: {
        doc,
        content: INDICATOR_CONTENT["pib"],
        methodology: METHODOLOGY_CONTENT["pib"],
        relatedCards: [],
        canonicalPath: "/indicador/pib",
      },
    });
    expect(html).toContain('data-testid="page-yoy-variation"');
    expect(html).toContain('data-testid="page-intra-annual-variation"');
    // Both rates must actually RESOLVE for `pib`, not merely be present.
    // The comment this line used to carry ("this fixture's own 3-period
    // history has a prior-quarter point") is obsolete: the fixture now
    // carries pib-cvi's full 125-quarter span (1995-Q1..2026-Q1), so the
    // year-on-year rate — which a 3-period history could never produce —
    // resolves here too (verify-report WARNING-6).
    expect(renderedText(html, "page-intra-annual-variation")).toMatch(/^[+-]\d+(\.\d+)?%$/);
    expect(renderedText(html, "page-yoy-variation")).toMatch(/^[+-]\d+(\.\d+)?%$/);
  });

  it("every related card, across all six pages, resolves to a slug with a real built page (closes slice 9a's disclosed narrowing)", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const methodology = METHODOLOGY_CONTENT[slug];
      for (const relatedSlug of methodology.relatedSlugs) {
        expect(METHODOLOGY_CONTENT[relatedSlug], `${slug}'s related card "${relatedSlug}" has no page`).toBeDefined();
        expect(seriesBySlug.has(INDICATOR_CONTENT[relatedSlug]?.artifactSlug ?? relatedSlug)).toBe(true);
      }
    }
  });
});

describe("IndicatorPage — slice 9b: data table / chart association and row-count parity, all six pages (task 9b.3)", () => {
  it("the chart SVG's aria-describedby references the SAME table id the accessible data table renders, on every page", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const doc = docFor(slug, seriesBySlug);
      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: {
          doc,
          content: INDICATOR_CONTENT[slug],
          methodology: METHODOLOGY_CONTENT[slug],
          relatedCards: [],
          canonicalPath: `/indicador/${slug}`,
        },
      });

      const svgMatch = /<svg[^>]*aria-describedby="([^"]+)"/.exec(html);
      expect(svgMatch, `${slug}: chart svg has no aria-describedby`).not.toBeNull();
      const [descriptionId, tableId] = svgMatch![1].split(" ");
      expect(html, `${slug}: no element with id="${descriptionId}"`).toContain(`id="${descriptionId}"`);
      expect(html, `${slug}: no table with id="${tableId}"`).toContain(`id="${tableId}"`);
      expect(html).toContain('data-testid="accessible-data-table"');
    }
  });

  it("the data table has exactly one row per rendered point, on every page (row-count parity)", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const doc = docFor(slug, seriesBySlug);
      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: {
          doc,
          content: INDICATOR_CONTENT[slug],
          methodology: METHODOLOGY_CONTENT[slug],
          relatedCards: [],
          canonicalPath: `/indicador/${slug}`,
        },
      });

      const tableStart = html.indexOf('data-testid="accessible-data-table"');
      const tableEnd = html.indexOf("</table>", tableStart);
      const tableHtml = html.slice(tableStart, tableEnd);
      const rowCount = [...tableHtml.matchAll(/<tr\b/g)].length - 1; // -1 for the header row
      expect(rowCount, `${slug}: table row count`).toBe(doc.points.length);
    }
  });
});

// verify-report WARNING-6 closure: "Page-level acceptance evidence rests on a
// 3-observation-per-series fixture."
//
// The consequences the report reproduced on the real build were that
// `grep -c 'data-testid="range-preset'` returned 0 on all six built pages
// (`availablePresets` correctly offers only `full` for a 3-period series) and
// the header year-on-year figure rendered `—` on all six. The code was right;
// the fixture was the limitation. The fixture now carries each series' REAL
// live length over its REAL span, so these tests assert the production
// behaviour that was previously unreachable — and pin the fixture's shape, so
// a future regeneration cannot silently shrink the acceptance evidence back.
//
// The VALUES in that history are synthesised and documented as such; the
// LENGTHS and SPANS are the real ones. See
// `web/test/fixtures/export/source.txt`.
describe("IndicatorPage — the fixture carries a production-representative history (verify-report WARNING-6)", () => {
  // Real live lengths and spans, recorded in
  // openspec/changes/phase-1-indicator-page/apply-progress.md.
  const REAL_SHAPE: Record<string, { count: number; first: string; last: string }> = {
    "tasa-de-paro-epa": { count: 98, first: "2002-Q1", last: "2026-Q2" },
    "ocupados-epa": { count: 98, first: "2002-Q1", last: "2026-Q2" },
    "ipc-general": { count: 294, first: "2002-01", last: "2026-06" },
    "ipc-subyacente": { count: 294, first: "2002-01", last: "2026-06" },
    "pib-cvi": { count: 125, first: "1995-Q1", last: "2026-Q1" },
    "poblacion-residente": { count: 122, first: "1971-Q1", last: "2026-Q2" },
  };

  it("every series carries its real live observation count over its real span", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const [artifactSlug, shape] of Object.entries(REAL_SHAPE)) {
      const doc = seriesBySlug.get(artifactSlug);
      expect(doc, `no fixture document for ${artifactSlug}`).toBeDefined();
      expect(doc!.points.length, `${artifactSlug}: observation count`).toBe(shape.count);
      expect(doc!.points[0].period, `${artifactSlug}: first period`).toBe(shape.first);
      expect(doc!.points[doc!.points.length - 1].period, `${artifactSlug}: last period`).toBe(shape.last);
    }
  });

  // THE defect the report reproduced by grepping the built pages. All five
  // presets are available for every one of these spans (the shortest starts in
  // 2002, so even `since-2008` has room), and `ChartIsland` only renders the
  // control group at all when more than one preset applies.
  it("all five range-preset controls render on every page — and the SAME page truncated back to three periods renders none, which is exactly the defect", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ALL_SIX_SLUGS) {
      const doc = docFor(slug, seriesBySlug);

      async function render(withDoc: SeriesDoc): Promise<string> {
        const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
        return container.renderToString(IndicatorPage, {
          props: {
            doc: withDoc,
            content: INDICATOR_CONTENT[slug],
            methodology: METHODOLOGY_CONTENT[slug],
            relatedCards: [],
            canonicalPath: `/indicador/${slug}`,
          },
        });
      }

      const html = await render(doc);
      expect(html, `${slug}: no range-preset control group`).toContain('data-testid="chart-range-controls"');
      for (const preset of ["full", "5y", "10y", "since-2008", "since-2018"]) {
        expect(html, `${slug}: no control for the "${preset}" preset`).toContain(`data-testid="range-preset-${preset}"`);
      }

      // The differential. `availablePresets` correctly offers only `full` for
      // a 3-period series, and `ChartIsland` renders no control group for a
      // single preset — so this assertion is what the OLD fixture made
      // unavoidable, and what the new one must not be able to drift back into
      // without failing here.
      const truncated: SeriesDoc = { ...doc, points: doc.points.slice(-3) };
      const truncatedHtml = await render(truncated);
      expect(truncatedHtml, `${slug}: a 3-period series must offer no range controls`).not.toContain(
        'data-testid="chart-range-controls"',
      );
    }
  });

  // Break-band rendering had NO production coverage at all before this
  // fixture: with three periods per series, every configured methodology
  // break fell outside every span, so `breaks` was `[]` on all six documents.
  // The regenerated fixture runs the real `ReconcileEditorialConfig` over the
  // real `config/rupturas.yaml`, and with a full history the EPA-2021 and
  // IPC-base-2021 breaks now fall INSIDE their series' spans.
  it("a real configured methodology break renders a break band on the pages whose span contains it", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    const WITH_BREAKS = ["tasa-de-paro-epa", "ocupados-epa", "ipc-general", "ipc-subyacente"] as const;

    for (const slug of WITH_BREAKS) {
      const doc = docFor(slug, seriesBySlug);
      expect(doc.breaks.length, `${slug}: fixture carries no resolved break`).toBeGreaterThan(0);

      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: {
          doc,
          content: INDICATOR_CONTENT[slug],
          methodology: METHODOLOGY_CONTENT[slug],
          relatedCards: [],
          canonicalPath: `/indicador/${slug}`,
        },
      });

      expect(html, `${slug}: no break band rendered`).toContain('data-testid="chart-breaks"');
      // The band names the break's own period and carries its note — the
      // reader-facing content, not just the container.
      expect(html, `${slug}: break band carries no tooltip`).toContain('data-testid="break-band-tooltip"');
      expect(html, `${slug}: break note absent`).toContain(doc.breaks[0].noteMd.slice(0, 40));
    }
  });

  // The other half of the same story: the two series with no configured break
  // inside their span must render no band. Asserting only the positive case
  // would pass for a component that always renders one.
  it("a page whose span contains no configured break renders no break band", async () => {
    const { seriesBySlug } = await loadFixtureArtifact();
    for (const slug of ["pib", "poblacion-residente"]) {
      const doc = docFor(slug, seriesBySlug);
      expect(doc.breaks, `${slug}: fixture unexpectedly carries a break`).toEqual([]);

      const container = await AstroContainer.create({ renderers: [{ name: "@astrojs/svelte", ssr: svelteServerRenderer }] });
      const html = await container.renderToString(IndicatorPage, {
        props: {
          doc,
          content: INDICATOR_CONTENT[slug],
          methodology: METHODOLOGY_CONTENT[slug],
          relatedCards: [],
          canonicalPath: `/indicador/${slug}`,
        },
      });
      expect(html, `${slug}: unexpected break band`).not.toContain('data-testid="chart-breaks"');
    }
  });
});
