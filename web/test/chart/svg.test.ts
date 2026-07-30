// design.md D-5: "one shared geometry module feeds both the build-time SVG
// and the island" — `renderChartSVG` is the ONE function both
// `IndicatorChart.astro` (this slice) and `ChartIsland.svelte` (slice 8)
// call. This file's golden test is the anti-divergence device D-5 commits
// to: slice 8's own island-parity test (task 8.12) re-asserts the SAME
// golden fixture against the island's initial client-side render.
import { describe, expect, it } from "vitest";
import { narrowChartVariant, renderChartSVG } from "../../src/lib/chart/svg";
import {
  GLYPH_ADVANCE_RATIO,
  NARROW_MAX_X_TICKS,
  NARROW_TICK_FONT_SIZE,
  narrowDimensions,
  type ChartPoint,
} from "../../src/lib/chart/geometry";

const GOLDEN_POINTS: ChartPoint[] = [
  { period: "2019-Q1", value: 10.2, status: "D" },
  { period: "2019-Q2", value: 10.5, status: "D" },
  { period: "2019-Q3", value: 10.1, status: "D" },
  { period: "2019-Q4", value: 9.8, status: "D" },
  { period: "2020-Q1", value: 14.4, status: "D" },
  { period: "2020-Q2", value: 15.3, status: "D" },
  { period: "2020-Q3", value: 16.3, status: "P" },
];

const GOLDEN_BREAKS = [{ key: "covid-2020", date: "2020-04-01" }];

const GOLDEN_INPUT = {
  points: GOLDEN_POINTS,
  breaks: GOLDEN_BREAKS,
  frequency: "Q" as const,
  decimals: 1,
  unit: "% población activa",
  titleId: "golden-title",
  descriptionId: "golden-description",
  tableId: "golden-table",
};

describe("renderChartSVG", () => {
  it("matches the committed golden fixture (build-time/island anti-divergence device)", async () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    await expect(svg).toMatchFileSnapshot("../fixtures/chart/golden-indicator-chart.svg");
  });

  it("renders exactly one break band per resolved break", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    const matches = svg.match(/data-testid="chart-break-band"/g) ?? [];
    expect(matches).toHaveLength(1);
    expect(svg).toContain('data-break-key="covid-2020"');
  });

  it("renders two break bands for two resolved breaks (indicator-page spec scenario)", () => {
    const svg = renderChartSVG({
      ...GOLDEN_INPUT,
      breaks: [
        { key: "b1", date: "2019-06-01" },
        { key: "b2", date: "2020-04-01" },
      ],
    });
    const matches = svg.match(/data-testid="chart-break-band"/g) ?? [];
    expect(matches).toHaveLength(2);
  });

  it("draws no break band at all for a series with no resolved breaks", () => {
    const svg = renderChartSVG({ ...GOLDEN_INPUT, breaks: [] });
    expect(svg).not.toContain("chart-break-band");
  });

  it("renders a provisional marker with a distinct shape (not just colour)", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain('data-testid="chart-marker-provisional"');
    expect(svg).toContain('data-testid="chart-marker-definitive"');
    // Different SVG element tags -- shape, not only fill colour, carries the
    // distinction (indicator-page spec, §12.5: "colour is never the sole
    // channel").
    expect(svg).toMatch(/<rect class="chart-marker chart-marker--provisional"/);
    expect(svg).toMatch(/<circle class="chart-marker chart-marker--definitive"/);
  });

  it("dashes the line segment ending at a provisional point", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain("stroke-dasharray");
  });

  it("consumes design tokens via var(--color-...) on SVG presentational attributes, never a hardcoded hex", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain("var(--color-accent)");
    expect(svg).toContain("var(--color-provisional)");
    expect(svg).toContain("var(--color-break-band)");
    expect(svg).not.toMatch(/#[0-9a-fA-F]{6}/);
  });

  it("is aria-labelled and aria-described, referencing the caller-supplied ids", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain('aria-labelledby="golden-title"');
    expect(svg).toContain('aria-describedby="golden-description golden-table"');
    expect(svg).toContain('role="img"');
  });

  it("never draws a line across a null-valued gap", () => {
    const svg = renderChartSVG({
      ...GOLDEN_INPUT,
      points: [
        { period: "2020-Q1", value: 1, status: "D" },
        { period: "2020-Q2", value: null, status: "D" },
        { period: "2020-Q3", value: 3, status: "D" },
      ],
      breaks: [],
    });
    const segments = svg.match(/data-testid="chart-line-segment-\d+"/g) ?? [];
    expect(segments).toHaveLength(2);
  });
});

// ---------------------------------------------------------------------------
// The narrow-viewport variant.
//
// One renderer, two boxes. `renderChartSVG` gains four optional inputs, all
// defaulted to exactly what it emitted before — which is what the golden
// fixture above is now ALSO proving: if any default drifted, that snapshot
// moves. The narrow variant is that same function called with the narrow
// geometry (see `geometry.ts` for why the phone needs its own box at all).
describe("renderChartSVG — narrow-viewport variant", () => {
  const NARROW_INPUT = { ...GOLDEN_INPUT, titleId: "golden-title-narrow", ...narrowChartVariant(GOLDEN_POINTS, 1) };

  it("leaves the wide variant byte-identical when the new inputs are omitted", () => {
    // Belt and braces alongside the golden snapshot: passing the defaults
    // explicitly must produce the same string as passing nothing, so a
    // future change to a default value cannot hide behind a call site that
    // happens to pass it.
    expect(
      renderChartSVG({ ...GOLDEN_INPUT, tickFontSize: 10, xTickLabelOffset: 16, maxXTicks: 6 }),
    ).toBe(renderChartSVG(GOLDEN_INPUT));
  });

  it("draws its tick labels at the narrow type size", () => {
    expect(renderChartSVG(NARROW_INPUT)).toContain(`font-size="${NARROW_TICK_FONT_SIZE}"`);
    expect(renderChartSVG(NARROW_INPUT)).not.toContain('font-size="10"');
  });

  it("carries the narrow viewBox, so the two variants are genuinely different drawings", () => {
    const narrowDims = narrowDimensions(
      GOLDEN_POINTS.map((p) => p.period),
      GOLDEN_POINTS,
      1,
    );
    expect(renderChartSVG(NARROW_INPUT)).toContain(`viewBox="0 0 ${narrowDims.width} ${narrowDims.height}"`);
    expect(renderChartSVG(GOLDEN_INPUT)).toContain('viewBox="0 0 960 360"');
  });

  it("thins the x axis to the narrow tick count", () => {
    const narrowTicks = renderChartSVG(NARROW_INPUT).match(/class="chart-tick chart-tick--x"/g) ?? [];
    const wideTicks = renderChartSVG(GOLDEN_INPUT).match(/class="chart-tick chart-tick--x"/g) ?? [];
    expect(narrowTicks.length).toBeLessThan(wideTicks.length);
    // `buildXTicks` always appends the final period if the stride missed it,
    // so the count can be the cap plus one — never more.
    expect(narrowTicks.length).toBeLessThanOrEqual(NARROW_MAX_X_TICKS + 1);
  });

  it("suffixes EVERY test id, so the two variants never collide in one document", () => {
    // Both variants are rendered into the same page and CSS shows one. If
    // they shared test ids, every existing `[data-testid=...]` locator and
    // every `toHaveCount(1)` in this suite would start matching two nodes —
    // a strict-mode failure at best, a silently doubled count at worst. The
    // suffix is what keeps this change invisible to locators that predate it.
    const narrow = renderChartSVG(NARROW_INPUT);
    const wide = renderChartSVG(GOLDEN_INPUT);
    for (const id of wide.match(/data-testid="([^"]+)"/g) ?? []) {
      const value = id.slice('data-testid="'.length, -1);
      expect(narrow, `${value} is not suffixed in the narrow variant`).not.toContain(`data-testid="${value}"`);
      expect(narrow).toContain(`data-testid="${value}-narrow"`);
    }
  });

  it("keeps its own class, so stylesheets can target one variant without the other", () => {
    expect(renderChartSVG(NARROW_INPUT)).toContain('class="indicator-chart-svg indicator-chart-svg--narrow"');
    expect(renderChartSVG(GOLDEN_INPUT)).toContain('class="indicator-chart-svg"');
  });

  it("draws every tick label INSIDE the viewBox, which is the whole point of the narrow margins", () => {
    // The assertion that would have caught the pre-existing clipping: no
    // y label may start left of x=0, and no x label may end right of the
    // viewBox width. Widths are estimated with the same measured glyph
    // ratio `geometry.ts` sizes the margins from, so this test and that
    // code disagree only if one of them is wrong.
    const svg = renderChartSVG(NARROW_INPUT);
    const dims = narrowDimensions(
      GOLDEN_POINTS.map((p) => p.period),
      GOLDEN_POINTS,
      1,
    );

    for (const [, x, label] of svg.matchAll(
      /class="chart-tick chart-tick--y"[^>]*x="([\d.]+)"[^>]*>([^<]+)</g,
    )) {
      const width = label.length * GLYPH_ADVANCE_RATIO * NARROW_TICK_FONT_SIZE;
      expect(Number(x) - width, `y label "${label}" starts left of the viewBox`).toBeGreaterThan(0);
    }

    for (const [, x, label] of svg.matchAll(
      /class="chart-tick chart-tick--x"[^>]*x="([\d.]+)"[^>]*>([^<]+)</g,
    )) {
      const half = (label.length * GLYPH_ADVANCE_RATIO * NARROW_TICK_FONT_SIZE) / 2;
      expect(Number(x) - half, `x label "${label}" starts left of the viewBox`).toBeGreaterThanOrEqual(0);
      expect(Number(x) + half, `x label "${label}" runs past the viewBox`).toBeLessThanOrEqual(dims.width);
    }
  });

  it("still consumes design tokens and never a hardcoded hex, exactly like the wide variant", () => {
    const svg = renderChartSVG(NARROW_INPUT);
    expect(svg).toContain("var(--color-accent)");
    expect(svg).not.toMatch(/#[0-9a-fA-F]{6}/);
  });

  it("still bakes in the break bands, so P4 holds in the variant a phone actually sees", () => {
    // indicator-page spec, "Series breaks are always visible and never
    // dismissible". A second variant is a second chance to lose them.
    const svg = renderChartSVG(NARROW_INPUT);
    expect(svg.match(/data-testid="chart-break-band-narrow"/g) ?? []).toHaveLength(1);
    expect(svg).toContain('data-break-key="covid-2020"');
  });
});
