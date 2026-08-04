// The homepage (milestone 1.2). Renders the REAL `src/pages/index.astro`
// through `experimental_AstroContainer`, not a stand-in: the page's whole
// reason to exist is that it reaches the export artifact at build time and
// turns it into six links, and a test that rendered only the presentation
// template would prove none of that wiring.
//
// The env stub is what makes that possible. `resolveLoadOptionsFromEnv`
// deliberately has NO default artifact source (verify-report CRITICAL-15 —
// the default used to be the synthetic fixture, and the Dockerfile's
// production build reached it), so a test that wants the fixture has to say
// so in the same words a developer would. `BUILD_WITH_SYNTHETIC_FIXTURE=1`
// resolves to `test/fixtures/export` relative to the vitest cwd (`web/`),
// which is the same directory the explicit-`dir` tests in this suite load.
import { afterEach, describe, expect, it, vi } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import Home from "../../src/pages/index.astro";
import HomePage from "../../src/templates/HomePage.astro";
import { FROZEN_INDICATOR_SLUGS } from "../../src/lib/indicator/routes";
import { INDICATOR_CONTENT } from "../../src/content/indicators";
import { es } from "../../src/i18n/es";

async function renderHome(): Promise<string> {
  vi.stubEnv("BUILD_WITH_SYNTHETIC_FIXTURE", "1");
  const container = await AstroContainer.create();
  return container.renderToString(Home);
}

/** Every `/indicador/...` href carried by an indicator card, in document
 * order. Reading the hrefs rather than counting cards is deliberate: six
 * cards that all link to the same page would satisfy a count. */
function cardHrefs(html: string): string[] {
  return [...html.matchAll(/<a[^>]*href="(\/indicador\/[^"]+)"[^>]*data-testid="indicator-card"/g)].map((m) => m[1]);
}

/** The page's heading levels in document order — the shape a screen-reader
 * user navigates by, and the one a recent defect broke by jumping h1 → h4. */
function headingLevels(html: string): number[] {
  return [...html.matchAll(/<h([1-6])[\s>]/g)].map((m) => Number(m[1]));
}

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("Home page — the six frozen indicators are reachable from `/`", () => {
  it("links to every frozen indicator route, once each, in the frozen order", async () => {
    const html = await renderHome();

    expect(cardHrefs(html)).toEqual(FROZEN_INDICATOR_SLUGS.map((slug) => `/indicador/${slug}`));
  });

  it("names each indicator with its editorial name, so the link text is not a slug", async () => {
    const html = await renderHome();

    for (const slug of FROZEN_INDICATOR_SLUGS) {
      expect(html, `missing the editorial name for "${slug}"`).toContain(INDICATOR_CONTENT[slug].name);
    }
  });

  it("shows each indicator's latest published value with its period and unit", async () => {
    const html = await renderHome();

    // Real published INE figures from the fixture's newest observations
    // (web/test/fixtures/export/source.txt), formatted to each indicator's
    // own configured decimals — 2 for the unemployment rate, 4 for PIB —
    // and punctuated the way a Spanish reader reads them: decimal comma,
    // grouping point. The homepage shipped `49687120` and `22779` before
    // this, which is the same digits and a different reading task.
    expect(html).toContain("9,87");
    // The period, in the same reader's vocabulary as the value beside it: a
    // card is a labelled field, so it takes the prose register — which for a
    // quarter is the T form INE itself publishes.
    expect(html).toContain("T2 2026");
    expect(html).not.toContain("2026-Q2");
    expect(html).toContain("121,9959");
    expect(html).toContain("T1 2026");
    expect(html).toContain(es.indicatorCard.periodLabel);
  });

  it("groups every thousands separator, leaving no bare digit run for a reader to count", async () => {
    const html = await renderHome();

    // The two figures the defect was found on, in the form a reader can
    // take in at a glance.
    expect(html).toContain("49.687.120");
    expect(html).toContain("22.779");
    // And the negative, which is what actually pins the fix: the raw runs
    // must be gone from the page entirely, not merely accompanied by a
    // formatted copy somewhere else in the markup.
    expect(html).not.toContain("49687120");
    expect(html).not.toContain("22779");
  });

  it("labels `ocupados-epa` in the thousands the pipeline measured it in", async () => {
    const html = await renderHome();

    // The card read `22779 personas` — 22.8 million people published as a
    // headcount, because the editorial unit had drifted from the artifact's
    // `miles de personas`. Asserted on the rendered page, not only in the
    // catalog, because the card is where a reader met the wrong figure.
    expect(html).toContain("miles de personas");
    expect(html).not.toMatch(/22\.779\s*<span[^>]*>personas</);
  });

  it("reports each indicator's freshness, so a stale source is visible before the reader clicks", async () => {
    const html = await renderHome();

    // Every fixture series is fresh, so this asserts the semaphore is
    // present and wired; the source-pending rendering is exercised below
    // against the template, which is the only way to get a state this
    // artifact does not contain without inventing artifact data.
    expect(html).toContain('data-testid="freshness-fresh"');
  });

  it("never skips a heading level: h1, then the section, then one heading per indicator", async () => {
    const html = await renderHome();

    const levels = headingLevels(html);
    expect(levels[0]).toBe(1);
    for (let i = 1; i < levels.length; i++) {
      expect(levels[i] - levels[i - 1], `heading level jumped from h${levels[i - 1]} to h${levels[i]}`).toBeLessThanOrEqual(1);
    }
    // The six indicator names really are headings, not decorative text —
    // otherwise the assertion above would hold vacuously over [1, 2].
    expect(levels.filter((level) => level === 3)).toHaveLength(FROZEN_INDICATOR_SLUGS.length);
  });

  it("ships no runtime JavaScript — it is a static list, not an island", async () => {
    const html = await renderHome();

    expect(html.toLowerCase()).not.toContain("<script");
  });
});

describe("HomePage template — states the artifact does not currently contain", () => {
  it("renders a source-pending semaphore for an indicator whose source has not published yet", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(HomePage, {
      props: {
        indicators: [
          {
            slug: "tasa-de-paro-epa",
            name: "Tasa de paro",
            unit: "% población activa",
            latestValue: 9.87,
            latestPeriod: "2026-Q2",
            freshness: "source-pending" as const,
            decimals: 2,
          },
        ],
      },
    });

    expect(html).toContain('data-testid="freshness-source-pending"');
    expect(html).toContain(es.freshness.sourcePending);
  });
});

describe("IndicatorCard — the value and its unit are two words, not one", () => {
  // Found by reading the BUILT homepage's visible text rather than its
  // markup: `22779personas`. Astro drops the whitespace around a lone
  // expression, so `{formattedValue}` followed by the unit span on the next
  // source line emits no text node between them, and the two render welded
  // together. Every assertion covering this component looked for the number
  // and the unit separately, so all of them passed on a string no reader
  // would accept.
  //
  // Same component, so the "related indicators" strip on all six indicator
  // pages has been rendering it this way too.
  it("separates the latest value from its unit in the rendered text", async () => {
    const html = await renderHome();

    expect(html).toContain(">22.779 <span");
    expect(html).not.toContain("22.779<span");
  });
});
