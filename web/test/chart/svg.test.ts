// design.md D-5: "one shared geometry module feeds both the build-time SVG
// and the island" — `renderChartSVG` is the ONE function both
// `IndicatorChart.astro` (this slice) and `ChartIsland.svelte` (slice 8)
// call. This file's golden test is the anti-divergence device D-5 commits
// to: slice 8's own island-parity test (task 8.12) re-asserts the SAME
// golden fixture against the island's initial client-side render.
import { describe, expect, it } from "vitest";
import { renderChartSVG } from "../../src/lib/chart/svg";
import type { ChartPoint } from "../../src/lib/chart/geometry";

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
