import { describe, expect, it } from "vitest";
import {
  DEFAULT_DIMENSIONS,
  GLYPH_ADVANCE_RATIO,
  NARROW_MAX_X_TICKS,
  NARROW_TICK_FONT_SIZE,
  NARROW_VIEWPORT_QUERY,
  NARROW_X_TICK_LABEL_OFFSET,
  WIDE_MIN_MARGIN_LEFT,
  WIDE_TICK_FONT_SIZE,
  narrowDimensions,
  wideDimensions,
  yTickLabels,
  buildBreakBands,
  buildLineSegments,
  buildXTicks,
  buildYTicks,
  plotArea,
  projectPoints,
  segmentPath,
  valueDomain,
  xForIndex,
  yForValue,
  type ChartDimensions,
  type ChartPoint,
} from "../../src/lib/chart/geometry";
import { formatPeriodCompact } from "../../src/lib/format/period";

const DIMS: ChartDimensions = DEFAULT_DIMENSIONS;

describe("plotArea", () => {
  it("subtracts every margin from the full canvas", () => {
    const area = plotArea(DIMS);
    expect(area.width).toBe(DIMS.width - DIMS.marginLeft - DIMS.marginRight);
    expect(area.height).toBe(DIMS.height - DIMS.marginTop - DIMS.marginBottom);
  });
});

describe("xForIndex", () => {
  it("centers a single point", () => {
    const area = plotArea(DIMS);
    expect(xForIndex(0, 1, DIMS)).toBeCloseTo(area.x0 + area.width / 2, 5);
  });

  it("places the first and last of several points at the plot area's edges", () => {
    const area = plotArea(DIMS);
    expect(xForIndex(0, 5, DIMS)).toBeCloseTo(area.x0, 5);
    expect(xForIndex(4, 5, DIMS)).toBeCloseTo(area.x1, 5);
  });

  it("spaces points evenly by RANK, not by calendar distance", () => {
    // Same guarantee poblacion-residente's mixed semiannual/quarterly
    // history relies on: index spacing never depends on the period labels.
    const x0 = xForIndex(0, 3, DIMS);
    const x1 = xForIndex(1, 3, DIMS);
    const x2 = xForIndex(2, 3, DIMS);
    expect(x1 - x0).toBeCloseTo(x2 - x1, 5);
  });
});

describe("valueDomain", () => {
  it("pads above and below the observed min/max", () => {
    const points: ChartPoint[] = [
      { period: "2020-Q1", value: 10, status: "D" },
      { period: "2020-Q2", value: 20, status: "D" },
    ];
    const [lo, hi] = valueDomain(points);
    expect(lo).toBeLessThan(10);
    expect(hi).toBeGreaterThan(20);
  });

  it("ignores null values", () => {
    const points: ChartPoint[] = [
      { period: "2020-Q1", value: null, status: "D" },
      { period: "2020-Q2", value: 5, status: "D" },
    ];
    const [lo, hi] = valueDomain(points);
    expect(lo).toBeLessThanOrEqual(5);
    expect(hi).toBeGreaterThanOrEqual(5);
  });

  it("produces a non-degenerate domain for a flat series", () => {
    const points: ChartPoint[] = [
      { period: "2020-Q1", value: 7, status: "D" },
      { period: "2020-Q2", value: 7, status: "D" },
    ];
    const [lo, hi] = valueDomain(points);
    expect(hi).toBeGreaterThan(lo);
  });
});

describe("yForValue", () => {
  it("maps the domain's low end to the plot area's bottom (SVG y grows downward)", () => {
    const area = plotArea(DIMS);
    expect(yForValue(0, [0, 10], DIMS)).toBeCloseTo(area.y1, 5);
    expect(yForValue(10, [0, 10], DIMS)).toBeCloseTo(area.y0, 5);
  });
});

describe("projectPoints / buildLineSegments / segmentPath", () => {
  it("breaks the line at a null value instead of joining across the gap", () => {
    const points: ChartPoint[] = [
      { period: "2020-Q1", value: 1, status: "D" },
      { period: "2020-Q2", value: null, status: "D" },
      { period: "2020-Q3", value: 3, status: "D" },
    ];
    const projected = projectPoints(points, DIMS);
    const segments = buildLineSegments(projected);
    expect(segments).toHaveLength(2);
    expect(segments[0]).toHaveLength(1);
    expect(segments[1]).toHaveLength(1);
  });

  it("produces one continuous segment for a fully populated series", () => {
    const points: ChartPoint[] = [
      { period: "2020-Q1", value: 1, status: "D" },
      { period: "2020-Q2", value: 2, status: "D" },
      { period: "2020-Q3", value: 3, status: "D" },
    ];
    const segments = buildLineSegments(projectPoints(points, DIMS));
    expect(segments).toHaveLength(1);
    expect(segments[0]).toHaveLength(3);
  });

  it("segmentPath starts with M and continues with L commands", () => {
    const points: ChartPoint[] = [
      { period: "2020-Q1", value: 1, status: "D" },
      { period: "2020-Q2", value: 2, status: "D" },
    ];
    const segments = buildLineSegments(projectPoints(points, DIMS));
    const d = segmentPath(segments[0]);
    expect(d.startsWith("M")).toBe(true);
    expect(d).toContain("L");
  });
});

describe("buildBreakBands", () => {
  it("positions a band at the period nearest the break's calendar date", () => {
    const periods = ["2020-Q1", "2020-Q2", "2020-Q3", "2020-Q4"];
    const bands = buildBreakBands([{ key: "b1", date: "2020-06-15" }], periods, "Q", DIMS);
    expect(bands).toHaveLength(1);
    const expectedCx = xForIndex(1, periods.length, DIMS); // 2020-Q2
    expect(bands[0].x + bands[0].width / 2).toBeCloseTo(expectedCx, 5);
  });

  it("renders one band per resolved break, at the correct periods", () => {
    const periods = ["2019-Q1", "2019-Q2", "2019-Q3", "2019-Q4", "2020-Q1"];
    const bands = buildBreakBands(
      [
        { key: "b1", date: "2019-03-01" }, // Q1
        { key: "b2", date: "2020-02-01" }, // 2020-Q1
      ],
      periods,
      "Q",
      DIMS,
    );
    expect(bands.map((b) => b.key)).toEqual(["b1", "b2"]);
  });

  it("returns no bands for an empty period list", () => {
    expect(buildBreakBands([{ key: "b1", date: "2020-01-01" }], [], "Q", DIMS)).toEqual([]);
  });
});

describe("buildXTicks / buildYTicks", () => {
  it("always includes the first and last period", () => {
    const periods = Array.from({ length: 40 }, (_, i) => `${2000 + Math.floor(i / 4)}-Q${(i % 4) + 1}`);
    const ticks = buildXTicks(periods, DIMS);
    // The FIRST and LAST period, in the register the axis draws them in — the
    // ticks are reader-facing text, and only their text changed here.
    expect(ticks[0].label).toBe(formatPeriodCompact(periods[0]));
    expect(ticks[ticks.length - 1].label).toBe(formatPeriodCompact(periods[periods.length - 1]));
  });

  it("never exceeds a small, legible tick count for a long series", () => {
    const periods = Array.from({ length: 294 }, (_, i) => `${1980 + Math.floor(i / 12)}-${String((i % 12) + 1).padStart(2, "0")}`);
    const ticks = buildXTicks(periods, DIMS, 6);
    expect(ticks.length).toBeLessThanOrEqual(7);
  });

  it("buildYTicks produces the requested count, ascending", () => {
    const ticks = buildYTicks([0, 100], DIMS, 1, 4);
    expect(ticks).toHaveLength(4);
    const values = ticks.map((t) => Number(t.label));
    expect(values).toEqual([...values].sort((a, b) => a - b));
  });
});

// ---------------------------------------------------------------------------
// Narrow-viewport geometry.
//
// THE DEFECT THIS DRIVES, measured in a real browser at a 375 px viewport on
// /indicador/tasa-de-paro-epa:
//
//   svg rendered box   312.3 x 117.1 CSS px
//   scale factor       312.3 / 960 = 0.325
//   tick font-size     10 user units -> 3.25 CSS px
//
// 3.25 CSS px is not small type, it is a grey smudge. The cause is not the
// font size: it is that a 960-unit-wide viewBox scaled into a 312 px column
// shrinks EVERYTHING by the same 0.325, and the chart is the centrepiece of
// every indicator page.
//
// WHY THE FIX IS A SECOND GEOMETRY AND NOT A BIGGER NUMBER. Raising
// `font-size` inside the 960x360 viewBox cannot work, and the same browser
// measurement says why: `poblacion-residente`'s y-axis label "49.477.903"
// renders about 54 units wide against the 48 units a 56-unit `marginLeft`
// leaves it, so it was clipped at the left edge at font-size 10, at every
// viewport — see the wide-viewport block at the end of this file for the
// measurements and the fix. Every unit added to the font makes that worse.
// The margins have to grow with the type, and the margins live in the
// dimensions — so each viewport gets its own dimensions, sized from the
// labels the series really has rather than from a guess.
describe("narrow-viewport geometry — the chart at a phone width", () => {
  const QUARTERLY_PERIODS = ["2024-Q1", "2024-Q2", "2024-Q3", "2024-Q4", "2025-Q1"];
  const SMALL_VALUE_POINTS: ChartPoint[] = QUARTERLY_PERIODS.map((period, i) => ({
    period,
    value: 10 + i * 0.5,
    status: "D",
  }));
  // The real shape of `poblacion-residente`: ten-glyph y-axis labels, which
  // is the case that already overflows today's fixed left margin.
  const POPULATION_POINTS: ChartPoint[] = QUARTERLY_PERIODS.map((period, i) => ({
    period,
    value: 49_477_903 + i * 76_079,
    status: "D",
  }));

  it("reserves enough left margin for the WIDEST y-axis label the series really produces", () => {
    const dims = narrowDimensions(QUARTERLY_PERIODS, POPULATION_POINTS, 0);
    const labels = yTickLabels(valueDomain(POPULATION_POINTS), 0);
    const widest = Math.max(...labels.map((l) => l.length));

    // `svg.ts` anchors a y label's RIGHT edge at `marginLeft - 8`, so the
    // label runs leftwards from there and must still start at a positive x.
    const widestLabelWidth = widest * GLYPH_ADVANCE_RATIO * NARROW_TICK_FONT_SIZE;
    expect(dims.marginLeft - 8 - widestLabelWidth).toBeGreaterThan(0);
  });

  it("does NOT spend that margin on a series whose labels are short", () => {
    // The cost of sizing for the worst case unconditionally would be a fat
    // empty gutter on five of the six pages. The margin is derived, so a
    // three-glyph label buys back the space a ten-glyph one needs.
    const wide = narrowDimensions(QUARTERLY_PERIODS, POPULATION_POINTS, 0);
    const narrow = narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    expect(narrow.marginLeft).toBeLessThan(wide.marginLeft);
    expect(plotArea(narrow).width).toBeGreaterThan(plotArea(wide).width);
  });

  it("keeps the LAST x-axis label inside the viewBox", () => {
    // Measured on the live page before the wide box derived its own margins:
    // the final tick "2026-Q2" was centred on x=944 in a 960-wide viewBox and
    // ran to 965.5 — clipped. Both boxes now reserve half a label on the
    // right, so the last tick cannot overhang either of them.
    const dims = narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const widestPeriod = Math.max(...QUARTERLY_PERIODS.map((p) => p.length));
    const halfLabel = (widestPeriod * GLYPH_ADVANCE_RATIO * NARROW_TICK_FONT_SIZE) / 2;
    expect(plotArea(dims).x1 + halfLabel).toBeLessThanOrEqual(dims.width);
  });

  it("leaves room below the axis for x labels drawn at the narrow tick size", () => {
    const dims = narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const baseline = dims.height - dims.marginBottom + NARROW_X_TICK_LABEL_OFFSET;
    // Baseline plus a descender must stay inside the box, or the labels are
    // clipped by the SVG's own overflow.
    expect(baseline + NARROW_TICK_FONT_SIZE * 0.25).toBeLessThanOrEqual(dims.height);
    // ...and must clear the axis line rather than sitting on it.
    expect(baseline).toBeGreaterThan(plotArea(dims).y1 + NARROW_TICK_FONT_SIZE);
  });

  it("renders tick labels at a legible size in the column a 375 px phone really gives it", () => {
    // The arithmetic the defect report is made of, run forwards. 312.3 px is
    // the measured content width of `main.max-w-4xl.p-6` at a 375 px
    // viewport (48 px of page padding, 15 px of scrollbar).
    const MEASURED_COLUMN_AT_375 = 312.3;
    const dims = narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const rendered = (MEASURED_COLUMN_AT_375 / dims.width) * NARROW_TICK_FONT_SIZE;

    expect(rendered).toBeGreaterThanOrEqual(10);
    // And the same arithmetic against the wide geometry, kept here so the
    // number this replaces is on the record next to the number it replaces.
    expect((MEASURED_COLUMN_AT_375 / DEFAULT_DIMENSIONS.width) * 10).toBeLessThan(4);
  });

  it("gives the chart a shape a phone can hold, instead of a 2.67:1 letterbox", () => {
    const dims = narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const wideAspect = DEFAULT_DIMENSIONS.width / DEFAULT_DIMENSIONS.height;
    const narrowAspect = dims.width / dims.height;
    expect(narrowAspect).toBeLessThan(wideAspect);
    // At 312 px wide the wide geometry gives a 117 px-tall plot; anything
    // squatter than 4:3 keeps the series' vertical movement unreadable.
    expect(narrowAspect).toBeLessThanOrEqual(4 / 3);
  });

  it("asks for fewer x-axis ticks, because a narrow axis cannot hold six labels", () => {
    // Six labels of seven glyphs at the narrow tick size need more width
    // than the plot area has; they would overlap into a solid bar.
    const dims = narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const labelWidth = 7 * GLYPH_ADVANCE_RATIO * NARROW_TICK_FONT_SIZE;
    expect(NARROW_MAX_X_TICKS * labelWidth).toBeLessThan(plotArea(dims).width);
  });

  it("keeps the FIRST x tick inside the viewBox too, since it is centred on the left edge", () => {
    // The mirror of the rule above, and the reason the left gutter is not
    // sized from the y labels alone: `xForIndex(0, …)` puts the first period
    // exactly on `plotArea().x0`, so half its label reaches into the gutter.
    const dims = narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const halfLabel = (7 * GLYPH_ADVANCE_RATIO * NARROW_TICK_FONT_SIZE) / 2;
    expect(plotArea(dims).x0 - halfLabel).toBeGreaterThanOrEqual(0);
  });

  it("declares a media query that matches the breakpoint the markup toggles on", () => {
    // The two variants are shown and hidden by Tailwind's `md:` utilities,
    // and the island reads the SAME boundary through `matchMedia` to know
    // which geometry its interactive overlay must line up with. If these two
    // ever disagreed, the hit targets would sit somewhere the chart is not —
    // which is exactly the kind of drift a shared constant prevents.
    expect(NARROW_VIEWPORT_QUERY).toBe("(max-width: 47.999rem)");
  });
});

// ---------------------------------------------------------------------------
// Wide-viewport geometry.
//
// THE DEFECT, measured with `getBBox()` in Chromium on the built
// /indicador/poblacion-residente at a 1280 px viewport, inside the wide
// drawing's own `viewBox="0 0 960 360"`:
//
//   49.477.903  left edge  -5.46      2026-Q2  right edge  965.50
//   49.553.982  left edge  -6.49
//   49.630.061  left edge  -4.60
//   49.706.140  left edge  -3.61
//
// Five clipped labels on one published chart. The same sweep across all six
// built pages found the last x tick clipped on every one of them.
//
// THE CAUSE was not the grouping separators that made it visible: it was that
// `marginLeft: 56` and `marginRight: 16` were constants nothing had ever
// derived. The narrow box had already solved this by computing its gutters
// from the labels the series really prints; `wideDimensions` is that same
// derivation applied to the box it was extracted from.
describe("wide-viewport geometry — the published chart's own margins", () => {
  const QUARTERLY_PERIODS = ["2025-Q3", "2025-Q4", "2026-Q1", "2026-Q2"];
  /** The real shape of `poblacion-residente`, the series that clipped. */
  const POPULATION_POINTS: ChartPoint[] = QUARTERLY_PERIODS.map((period, i) => ({
    period,
    value: 49_477_903 + i * 76_079,
    status: "D",
  }));
  /** The other five pages: three- and four-glyph y labels. */
  const SMALL_VALUE_POINTS: ChartPoint[] = QUARTERLY_PERIODS.map((period, i) => ({
    period,
    value: 10 + i * 0.5,
    status: "D",
  }));

  it("reserves enough left margin for the WIDEST y-axis label the series really produces", () => {
    const dims = wideDimensions(QUARTERLY_PERIODS, POPULATION_POINTS, 0);
    const labels = yTickLabels(valueDomain(POPULATION_POINTS), 0);
    const widest = Math.max(...labels.map((l) => l.length));
    const widestLabelWidth = widest * GLYPH_ADVANCE_RATIO * WIDE_TICK_FONT_SIZE;

    // `svg.ts` anchors a y label's RIGHT edge at `marginLeft - 8`, so the
    // label runs leftwards from there and must still start at a positive x.
    // With the old constant this was 48 - 64 = -16.
    expect(dims.marginLeft - 8 - widestLabelWidth).toBeGreaterThan(0);
  });

  it("keeps the LAST x-axis label inside the viewBox, which the constant 16 could not", () => {
    // The last tick is centred on the final data point, which sits exactly on
    // `plotArea().x1` — so half a label always overhangs unless the right
    // margin pays for it. Measured on the live page: "2026-Q2" centred on
    // x=944 ran to 965.5 in a 960-unit box.
    const dims = wideDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const widestPeriod = Math.max(...QUARTERLY_PERIODS.map((p) => p.length));
    const halfLabel = (widestPeriod * GLYPH_ADVANCE_RATIO * WIDE_TICK_FONT_SIZE) / 2;
    expect(plotArea(dims).x1 + halfLabel).toBeLessThanOrEqual(dims.width);
    expect(plotArea(dims).x0 - halfLabel).toBeGreaterThanOrEqual(0);
  });

  it("holds a FLOOR under the left gutter, unlike the narrow box which has none", () => {
    // The two boxes make opposite trades because their widths are opposite.
    // At 560 units a derived-down gutter buys a phone reader plot area they
    // can see; at 960 units the same 18 units is 1.9% of the drawing, and
    // spending it keeps the y axis off the edge of the box. So the wide
    // variant floors at what it has always reserved and only ever grows.
    const short = wideDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1);
    const wide = wideDimensions(QUARTERLY_PERIODS, POPULATION_POINTS, 0);
    expect(short.marginLeft).toBe(WIDE_MIN_MARGIN_LEFT);
    expect(wide.marginLeft).toBeGreaterThan(WIDE_MIN_MARGIN_LEFT);
    expect(narrowDimensions(QUARTERLY_PERIODS, SMALL_VALUE_POINTS, 1).marginLeft).toBeLessThan(
      narrowDimensions(QUARTERLY_PERIODS, POPULATION_POINTS, 0).marginLeft,
    );
  });

  it("changes only the margins, never the box the page reserves space for", () => {
    // `ChartIsland.svelte`'s `<style>` hard-codes `aspect-ratio: 960 / 360`
    // to stop the page reflowing when the island hydrates. A derivation that
    // touched width or height would silently break that promise for the one
    // series whose labels are widest.
    for (const points of [POPULATION_POINTS, SMALL_VALUE_POINTS]) {
      const dims = wideDimensions(QUARTERLY_PERIODS, points, 0);
      expect(dims.width).toBe(DEFAULT_DIMENSIONS.width);
      expect(dims.height).toBe(DEFAULT_DIMENSIONS.height);
      expect(dims.marginTop).toBe(DEFAULT_DIMENSIONS.marginTop);
      expect(dims.marginBottom).toBe(DEFAULT_DIMENSIONS.marginBottom);
    }
  });
});

// ---------------------------------------------------------------------------
// The x axis speaks to a reader, so it is drawn in the reader's own period
// vocabulary ("T2 2026", "jun 2026") rather than in the database's storage
// format ("2026-Q2", "2026-06") — see `src/lib/format/period.ts`.
//
// The load-bearing part is not the text: it is that the DERIVED MARGINS are
// sized from the labels that are really drawn. Those margins were derived
// (rather than fixed at a constant) precisely to stop the last x tick being
// clipped, and measuring one alphabet while drawing another would reopen that
// defect from the other side.
describe("x-axis tick labels are reader-facing, and the margins are sized from what is drawn", () => {
  const QUARTERS = ["2019-Q1", "2019-Q2", "2019-Q3", "2019-Q4", "2020-Q1"];
  const MONTHS = ["2026-01", "2026-06", "2026-09", "2026-12"];
  const flatPoints = (periods: string[]): ChartPoint[] =>
    periods.map((period) => ({ period, value: 10, status: "D" as const }));

  it("labels quarters as INE's own T, first tick and last", () => {
    const ticks = buildXTicks(QUARTERS, DIMS);
    expect(ticks[0].label).toBe("T1 2019");
    expect(ticks[ticks.length - 1].label).toBe("T1 2020");
  });

  it("labels months with the abbreviated Spanish month name", () => {
    const ticks = buildXTicks(MONTHS, DIMS);
    expect(ticks.map((t) => t.label)).toEqual(["ene 2026", "jun 2026", "sept 2026", "dic 2026"]);
  });

  it("leaves a quarterly series' derived margins exactly where they were", () => {
    // "T2 2026" is seven glyphs, like the "2026-Q2" it replaces — so every
    // quarterly page's geometry is unchanged, and the golden SVG fixture moves
    // only in its tick text.
    const dims = wideDimensions(QUARTERS, flatPoints(QUARTERS), 1);
    expect(dims.marginRight).toBe(27);
    expect(dims.marginLeft).toBe(WIDE_MIN_MARGIN_LEFT);
  });

  it("widens a monthly series' right gutter to hold the longest month name it will draw", () => {
    // "sept 2026" is nine glyphs against "2026-09"'s seven. The last tick is
    // centred on the plot area's right edge, so half of it has to fit beyond
    // that edge — which is exactly what deriving the gutter buys.
    const dims = wideDimensions(MONTHS, flatPoints(MONTHS), 1);
    const halfWidest = ("sept 2026".length * GLYPH_ADVANCE_RATIO * WIDE_TICK_FONT_SIZE) / 2;
    expect(dims.marginRight).toBeGreaterThanOrEqual(halfWidest);
  });

  it("keeps every drawn tick inside the box for a monthly series, which the storage format never had to prove", () => {
    const dims = wideDimensions(MONTHS, flatPoints(MONTHS), 1);
    const ticks = buildXTicks(MONTHS, dims);
    for (const tick of ticks) {
      const half = (tick.label.length * GLYPH_ADVANCE_RATIO * WIDE_TICK_FONT_SIZE) / 2;
      expect(tick.x - half, `"${tick.label}" runs off the left edge`).toBeGreaterThanOrEqual(0);
      expect(tick.x + half, `"${tick.label}" runs off the right edge`).toBeLessThanOrEqual(dims.width);
    }
  });
});
