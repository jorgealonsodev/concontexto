// Pure scale/geometry functions (design.md D-5: "one shared geometry module
// feeds both the build-time SVG and the island" — data in, geometry/points
// out, no DOM). `IndicatorChart.astro` (slice 7) and `ChartIsland.svelte`
// (slice 8) both call these; a Vitest golden test (svg.test.ts) is the
// anti-divergence device D-5 commits to.
import { type Frequency, nearestPeriodIndex, periodFromCalendarDate } from "./periods";
// The y-axis labels are read by a person, so they are formatted like every
// other reader-facing figure on the page. Without this the same number appeared
// two ways on one screen: the accessible data table read "49.687.120" while the
// axis beside it read "49687120". Geometry -- the `x`/`y` coordinates and the
// path `d` attribute above -- keeps `toFixed`, because those are SVG machine
// values where a grouping separator would be a syntax error.
import { formatNumber } from "../format/number";
// The x-axis labels are read by a person too, and by the same argument: the
// axis said "2026-Q2" — the database's storage format, carrying the English
// abbreviation for *quarter* — beside a source that publishes "T2". The
// COMPACT register is the right one here and not the prose one: a tick has a
// width, and this module's own margins are derived from it.
import { formatPeriodCompact } from "../format/period";

export type ObservationStatus = "P" | "D";

export interface ChartPoint {
  period: string;
  value: number | null;
  status: ObservationStatus;
}

export interface ChartDimensions {
  width: number;
  height: number;
  marginTop: number;
  marginRight: number;
  marginBottom: number;
  marginLeft: number;
}

/** Tick type size for the wide variant, in user units.
 *
 * Exported rather than left as a literal in `svg.ts` because the margins the
 * wide box reserves are DERIVED from the width of the labels drawn at this
 * size (see `wideDimensions`). If the renderer drew ticks at one size while
 * the geometry sized the gutters for another, the labels would be clipped
 * again — the exact defect the derivation exists to end. One constant, so the
 * two cannot disagree. */
export const WIDE_TICK_FONT_SIZE = 10;

/** The wide box: the four numbers that are the same for every series.
 *
 * `width`/`height`/`marginTop`/`marginBottom` are the box itself. The two
 * horizontal margins here are NOT what production renders with —
 * `wideDimensions` replaces both from the series' own axis labels, keeping 56
 * only as a floor (`WIDE_MIN_MARGIN_LEFT`) and deriving the right one
 * outright. Reach for this constant where a box is needed and the labels are
 * irrelevant (pure scale tests); anything that draws or hit-tests a real
 * series must derive its own, because a constant gutter here is precisely
 * what clipped five labels on /indicador/poblacion-residente. */
export const DEFAULT_DIMENSIONS: ChartDimensions = {
  width: 960,
  height: 360,
  marginTop: 16,
  marginRight: 16,
  marginBottom: 32,
  marginLeft: 56,
};

export interface PlotArea {
  x0: number;
  y0: number;
  x1: number;
  y1: number;
  width: number;
  height: number;
}

export function plotArea(dims: ChartDimensions): PlotArea {
  const x0 = dims.marginLeft;
  const y0 = dims.marginTop;
  const x1 = dims.width - dims.marginRight;
  const y1 = dims.height - dims.marginBottom;
  return { x0, y0, x1, y1, width: x1 - x0, height: y1 - y0 };
}

/** Ordinal x position: evenly spaced by data-point RANK, not by real
 * elapsed calendar time — tolerant of an irregular cadence (e.g.
 * poblacion-residente mixes semiannual and quarterly spacing in one
 * series; an evenly-spaced ordinal axis never has to invent a position for
 * a period that was never observed). A single point centers in the plot
 * area. */
export function xForIndex(index: number, count: number, dims: ChartDimensions): number {
  const area = plotArea(dims);
  if (count <= 1) return area.x0 + area.width / 2;
  return area.x0 + (index / (count - 1)) * area.width;
}

/** The padded value domain `[min, max]` across every non-null value. A
 * flat series (min === max) still gets a usable non-zero-height domain.
 * Empty input yields `[0, 1]` — an arbitrary but harmless default; callers
 * never project points against it when there are no points to project. */
export function valueDomain(points: ChartPoint[]): [number, number] {
  const values = points.map((p) => p.value).filter((v): v is number => v !== null);
  if (values.length === 0) return [0, 1];
  let min = Math.min(...values);
  let max = Math.max(...values);
  if (min === max) {
    min -= 1;
    max += 1;
  }
  const pad = (max - min) * 0.1;
  return [min - pad, max + pad];
}

/** SVG y grows downward, so the domain's low end maps to the plot area's
 * bottom edge. */
export function yForValue(value: number, domain: [number, number], dims: ChartDimensions): number {
  const area = plotArea(dims);
  const [lo, hi] = domain;
  const t = (value - lo) / (hi - lo);
  return area.y1 - t * area.height;
}

export interface ProjectedPoint extends ChartPoint {
  x: number;
  y: number | null;
}

export function projectPoints(points: ChartPoint[], dims: ChartDimensions): ProjectedPoint[] {
  const domain = valueDomain(points);
  return points.map((p, i) => ({
    ...p,
    x: xForIndex(i, points.length, dims),
    y: p.value === null ? null : yForValue(p.value, domain, dims),
  }));
}

/** Splits the projected series into contiguous runs, breaking at every null
 * value — series-transformations spec: "rendered as absent, not as zero".
 * A straight line MUST NOT be drawn across a gap. */
export function buildLineSegments(projected: ProjectedPoint[]): ProjectedPoint[][] {
  const segments: ProjectedPoint[][] = [];
  let current: ProjectedPoint[] = [];
  for (const p of projected) {
    if (p.value === null || p.y === null) {
      if (current.length) segments.push(current);
      current = [];
      continue;
    }
    current.push(p);
  }
  if (current.length) segments.push(current);
  return segments;
}

export function segmentPath(segment: ProjectedPoint[]): string {
  return segment
    .map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(2)},${(p.y as number).toFixed(2)}`)
    .join(" ");
}

export interface BreakBandInput {
  key: string;
  date: string;
}

export interface BreakBandGeometry {
  key: string;
  x: number;
  width: number;
}

/** Positions one shaded vertical band per break, centered on the plotted
 * period nearest the break's calendar date (indicator-page spec: "two
 * resolved breaks render two bands at the correct periods"). Band width is
 * one inter-period step wide (half a step on each side) so adjoining bands
 * never overlap for evenly-spaced data. */
export function buildBreakBands(
  breaks: BreakBandInput[],
  periods: string[],
  frequency: Frequency,
  dims: ChartDimensions,
): BreakBandGeometry[] {
  if (periods.length === 0) return [];
  const area = plotArea(dims);
  const step = periods.length > 1 ? area.width / (periods.length - 1) : area.width;
  const halfWidth = step / 2;
  return breaks.map((b) => {
    const targetLabel = periodFromCalendarDate(b.date, frequency);
    const index = nearestPeriodIndex(periods, frequency, targetLabel);
    const cx = xForIndex(index, periods.length, dims);
    return { key: b.key, x: cx - halfWidth, width: halfWidth * 2 };
  });
}

export interface AxisTick {
  x: number;
  label: string;
}

/** A small, readable set of x-axis ticks (never every period — that would
 * be illegible for a 294-point monthly series) always including the first
 * and last plotted period.
 *
 * `periods` arrives, and is indexed, in the CANONICAL storage form — that is
 * what every caller holds and what `buildBreakBands` positions against. Only
 * the `label` is formatted, at the last possible moment, which is what keeps
 * the display decision from leaking into the axis arithmetic. The
 * duplicate-tick check below compares INDICES rather than labels for exactly
 * that reason: two distinct periods can never share an index, whereas
 * comparing rendered text would be comparing the wrong thing. */
export function buildXTicks(periods: string[], dims: ChartDimensions, maxTicks = 6): AxisTick[] {
  if (periods.length === 0) return [];
  const step = Math.max(1, Math.ceil((periods.length - 1) / Math.max(1, maxTicks - 1)));
  const indices: number[] = [];
  for (let i = 0; i < periods.length; i += step) indices.push(i);
  const lastIndex = periods.length - 1;
  if (indices[indices.length - 1] !== lastIndex) indices.push(lastIndex);
  return indices.map((i) => ({
    x: xForIndex(i, periods.length, dims),
    label: formatPeriodCompact(periods[i]),
  }));
}

export interface ValueTick {
  y: number;
  label: string;
}

export function buildYTicks(
  domain: [number, number],
  dims: ChartDimensions,
  decimals: number,
  count = 4,
): ValueTick[] {
  return yTickLabels(domain, decimals, count).map((label, i) => {
    const [lo, hi] = domain;
    const value = lo + ((hi - lo) * i) / (count - 1);
    return { y: yForValue(value, domain, dims), label };
  });
}

/** The y-axis tick LABELS alone, with no geometry attached.
 *
 * Extracted from `buildYTicks` because the narrow layout has a
 * chicken-and-egg problem the wide one does not: its left margin has to be
 * wide enough for the labels, and the labels are what `buildYTicks` needs a
 * `ChartDimensions` to produce. The label text depends only on the domain
 * and the series' decimal count, never on the box — so it can be asked for
 * first, and `buildYTicks` keeps producing exactly what it always did by
 * calling through here. */
export function yTickLabels(domain: [number, number], decimals: number, count = 4): string[] {
  const [lo, hi] = domain;
  const labels: string[] = [];
  for (let i = 0; i < count; i++) {
    labels.push(formatNumber(lo + ((hi - lo) * i) / (count - 1), decimals));
  }
  return labels;
}

// ---------------------------------------------------------------------------
// Per-variant geometry: the narrow box (phones) and the wide one, both sized
// by the SAME derivation (`derivedMargins`, below).
//
// THE PROBLEM THE NARROW BOX SOLVES, measured in a real browser rather than
// estimated. At a 375 px viewport an indicator page gives the chart a 312.3 px
// column, so the 960-unit viewBox above is drawn at a scale of 0.325 and its
// `font-size="10"` tick labels land at 3.25 CSS px. Everything in the drawing
// shrinks by that same factor, which is why this is a geometry problem and not
// a font-size one.
//
// WHY A SECOND SET OF DIMENSIONS RATHER THAN A BIGGER FONT IN THE WIDE ONE.
// The same browser measurement rules the simple fix out. `poblacion-residente`
// renders y-axis labels like "49.477.903" about 54 units wide, into the 48
// units a 56-unit left margin leaves between the viewBox edge and the label
// anchor: they were clipped at every viewport, at font-size 10, until
// `wideDimensions` below started deriving that margin too. Type and margins
// are one decision, not two, and the margins live here.
//
// WHY THE MARGINS ARE DERIVED AND NOT CONSTANTS. Two reasons, one per box.
// In the narrow box, sizing every page's left gutter for
// `poblacion-residente`'s ten-glyph labels would spend a fifth of a phone's
// screen width on empty space for the five series whose labels are three or
// four glyphs long. In the wide box the pressure is the opposite one: a
// constant gutter chosen once, for no series in particular, silently clipped
// the widest series' labels and every page's final x tick. A gutter computed
// from the labels the series really produces is the only version that is
// right for both.
//
// WHAT THIS COSTS, stated plainly. One viewBox cannot serve a 312 px column
// and an 848 px one: the ratio between them is 2.7, so anything legible in
// the first is oversized in the second, and vice versa. A second geometry is
// the price of that arithmetic — the chart is rendered twice into the page
// and CSS shows exactly one (see `IndicatorChart.astro`). Measured on the
// heaviest page (`poblacion-residente`, 294 points), the second copy adds
// 3.7 KB gzipped to a 17.4 KB document, against a 300 KB budget. The residual
// imprecision is at the WIDE end of this band: the same viewBox is scaled up
// as the viewport approaches 768 px, so tick labels there render larger than
// they strictly need to be. Generous is a defect an order of magnitude
// smaller than illegible, and it is the one this trade buys.

/** Advance width of one glyph, as a fraction of the font size, for the two
 * alphabets the axes actually print: digits with `.` and `,` separators, and
 * period labels like `2026-Q2`.
 *
 * MEASURED, not guessed: at font-size 10 in the shipped typeface, Chromium
 * reports 55.3 units for the ten-glyph "49.477.903" (0.553/glyph) and 44.8
 * units for the seven-glyph "2025-Q4" (0.640/glyph). The larger of the two is
 * used for both, so the margins this drives are conservative for numerals
 * rather than tight for letters. A ratio is enough here because the only
 * thing it protects is a margin: over-reserving costs a few units of gutter,
 * under-reserving clips a label. */
export const GLYPH_ADVANCE_RATIO = 0.64;

/** Tick type size for the narrow variant, in user units.
 *
 * Chosen from the column a phone really gives the chart, not from taste:
 * 20 units in a 560-unit viewBox drawn into 312.3 px renders at 11.2 CSS px,
 * which is ordinary small-print size. The wide variant's 10 units renders at
 * 3.25 px in that same column. */
export const NARROW_TICK_FONT_SIZE = 20;

/** Distance from the plot area's bottom edge to the x-label baseline. Scaled
 * with the type (the wide variant uses 16 for a 10-unit font) so the labels
 * clear the axis line rather than sitting on it. */
export const NARROW_X_TICK_LABEL_OFFSET = 28;

/** Three x-axis ticks, not six. Six seven-glyph labels at the narrow tick
 * size need more width than the narrow plot area has, so they would overlap
 * into an unreadable bar; `buildXTicks` always keeps the first and the last
 * period, which are the two a reader needs to know what span they are
 * looking at. */
export const NARROW_MAX_X_TICKS = 3;

/** The viewport below which the narrow geometry applies.
 *
 * This MUST stay equal to Tailwind's own `md` breakpoint (48rem), because
 * the two rendered variants are shown and hidden with `md:hidden` /
 * `hidden md:block`, and `ChartIsland.svelte` reads this same string through
 * `matchMedia` to decide which geometry its interactive overlay must align
 * to. One constant, so the CSS boundary and the JS boundary cannot drift and
 * leave the hit targets sitting where the chart is not.
 *
 * `md` rather than `sm` deliberately: at a 767 px viewport the wide geometry
 * still renders its ticks at about 7 CSS px, so stopping the narrow variant
 * at 640 px would leave a band of tablet widths unreadable. */
export const NARROW_VIEWPORT_QUERY = "(max-width: 47.999rem)";

const NARROW_WIDTH = 560;
/** 560x420 is exactly 4:3, against the wide variant's 2.67:1. At 312 px wide
 * that is a 234 px-tall chart instead of a 117 px one — the difference
 * between a series whose movement you can read and a horizontal smear. A
 * first draft used 400 (1.4:1) and was pulled back to 4:3 by this module's
 * own test, which fixes that ceiling: vertical resolution is the whole point
 * of reshaping the chart, and 4:3 is where a time series stops reading as a
 * strip. */
const NARROW_HEIGHT = 420;
const NARROW_MARGIN_TOP = 20;
/** `svg.ts` anchors each y label's right edge this far left of the plot
 * area, so the gutter must hold the label PLUS this gap. */
const Y_LABEL_ANCHOR_GAP = 8;
/** A little air beyond the computed label width, so a glyph slightly wider
 * than the average ratio still lands inside the box. */
const LABEL_SAFETY_MARGIN = 4;

function widestLabelWidth(labels: string[], tickFontSize: number): number {
  const glyphs = labels.reduce((max, label) => Math.max(max, label.length), 0);
  return glyphs * GLYPH_ADVANCE_RATIO * tickFontSize;
}

/** The horizontal gutters one series needs so that no axis label is drawn
 * outside the box — the ONE derivation both variants use.
 *
 * There is a single rule here because there is a single failure: `svg.ts`
 * anchors a y label's right edge at `marginLeft - 8` and centres an x label
 * on its own tick, so three quantities have to fit and the margins are what
 * pays for them.
 *
 *   LEFT   the widest y label plus its anchor gap; and, because the FIRST x
 *          tick is centred on the plot area's left edge, half the widest
 *          period label. Whichever is larger wins.
 *   RIGHT  half the widest period label, because the LAST x tick is centred
 *          on the plot area's right edge.
 *
 * `minMarginLeft` is a floor, not a target: a variant may insist on a gutter
 * wider than its labels strictly need (see `WIDE_MIN_MARGIN_LEFT`).
 *
 * The widths are ESTIMATED from `GLYPH_ADVANCE_RATIO`, deliberately using the
 * wider of the two measured alphabets, so the estimate errs towards a gutter
 * a few units too wide rather than a label a few units clipped. The browser
 * gate in `tests/e2e/indicator/indicator-pages.spec.ts` measures the real
 * `getBBox()` of every tick on all six built pages, so an estimate that ever
 * stops being conservative fails there rather than shipping. */
function derivedMargins(input: {
  periods: string[];
  yLabels: string[];
  tickFontSize: number;
  minMarginLeft?: number;
}): { marginLeft: number; marginRight: number } {
  // MEASURED ON WHAT IS DRAWN, not on what is stored. `buildXTicks` renders
  // each period through `formatPeriodCompact`, and a gutter sized from the
  // canonical label would be measuring a different alphabet: "sept 2026" is
  // nine glyphs where "2026-09" was seven, so a monthly page's final tick
  // would have hung outside the box — the exact defect this derivation was
  // introduced to end. Quarters are unaffected ("T2 2026" is seven glyphs,
  // like "2026-Q2"), which is why no quarterly page's coordinates move.
  const halfPeriodLabel = widestLabelWidth(input.periods.map(formatPeriodCompact), input.tickFontSize) / 2;
  const yLabelGutter =
    Math.ceil(widestLabelWidth(input.yLabels, input.tickFontSize)) +
    Y_LABEL_ANCHOR_GAP +
    LABEL_SAFETY_MARGIN;
  const marginRight = Math.ceil(halfPeriodLabel) + LABEL_SAFETY_MARGIN;
  return {
    marginLeft: Math.max(input.minMarginLeft ?? 0, yLabelGutter, marginRight),
    marginRight,
  };
}

/**
 * The narrow-viewport box for one series, with its margins sized from the
 * labels that series really prints.
 *
 * `periods` sizes the RIGHT margin: the last x tick is centred on the plot
 * area's right edge, so half a label has to fit beyond it or the final period
 * — the one a reader looks for first — is clipped.
 *
 * `points` and `decimals` size the LEFT margin, through the real y-tick
 * labels rather than through an assumption about magnitude.
 *
 * No floor: at 560 units wide the gutter is a real share of the drawing, and
 * spending a ten-glyph series' gutter on a three-glyph one would cost a phone
 * reader plot area they can see the loss of.
 */
export function narrowDimensions(
  periods: string[],
  points: ChartPoint[],
  decimals: number,
): ChartDimensions {
  const { marginLeft, marginRight } = derivedMargins({
    periods,
    yLabels: yTickLabels(valueDomain(points), decimals),
    tickFontSize: NARROW_TICK_FONT_SIZE,
  });
  // Baseline offset, plus a full em for the type itself, plus air: enough
  // that a descender never reaches the viewBox edge and gets clipped.
  const marginBottom = NARROW_X_TICK_LABEL_OFFSET + NARROW_TICK_FONT_SIZE + 8;
  return {
    width: NARROW_WIDTH,
    height: NARROW_HEIGHT,
    marginTop: NARROW_MARGIN_TOP,
    marginRight,
    marginBottom,
    marginLeft,
  };
}

/** The floor under the wide variant's left gutter.
 *
 * 56 is what `DEFAULT_DIMENSIONS` has always reserved, and it is kept as a
 * MINIMUM rather than dropped, for a reason the narrow box cannot claim: at
 * 960 units wide, the ~18 units a four-glyph series would win back are 1.9%
 * of the drawing — invisible to a reader — while the y axis is where the eye
 * enters the chart and a label pressed against the left edge of the box reads
 * as cramped. Nothing is bought by shrinking it, and the coordinates of five
 * of the six pages stay where they were.
 *
 * The floor is a minimum in the strict sense: `poblacion-residente`'s
 * ten-glyph labels need 76 and get 76. */
export const WIDE_MIN_MARGIN_LEFT = 56;

/**
 * The wide box for one series, with the same margins derivation the narrow
 * box uses — the fix for a defect the narrow variant never had because it was
 * derived from the start.
 *
 * Measured before this existed, on the built /indicador/poblacion-residente:
 * every y label ("49.477.903" and its three siblings) started between 3.6 and
 * 6.5 units LEFT of the viewBox origin, and the last x tick ("2026-Q2") ended
 * at 965.5 against a 960-unit box. Five clipped labels on one published
 * chart, and one clipped x tick on all six pages.
 */
export function wideDimensions(
  periods: string[],
  points: ChartPoint[],
  decimals: number,
): ChartDimensions {
  const { marginLeft, marginRight } = derivedMargins({
    periods,
    yLabels: yTickLabels(valueDomain(points), decimals),
    tickFontSize: WIDE_TICK_FONT_SIZE,
    minMarginLeft: WIDE_MIN_MARGIN_LEFT,
  });
  return { ...DEFAULT_DIMENSIONS, marginLeft, marginRight };
}
