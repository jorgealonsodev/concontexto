// Pure scale/geometry functions (design.md D-5: "one shared geometry module
// feeds both the build-time SVG and the island" — data in, geometry/points
// out, no DOM). `IndicatorChart.astro` (slice 7) and `ChartIsland.svelte`
// (slice 8) both call these; a Vitest golden test (svg.test.ts) is the
// anti-divergence device D-5 commits to.
import { type Frequency, nearestPeriodIndex, periodFromCalendarDate } from "./periods";

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
 * and last plotted period. */
export function buildXTicks(periods: string[], dims: ChartDimensions, maxTicks = 6): AxisTick[] {
  if (periods.length === 0) return [];
  const step = Math.max(1, Math.ceil((periods.length - 1) / Math.max(1, maxTicks - 1)));
  const ticks: AxisTick[] = [];
  for (let i = 0; i < periods.length; i += step) {
    ticks.push({ x: xForIndex(i, periods.length, dims), label: periods[i] });
  }
  const lastIndex = periods.length - 1;
  if (ticks[ticks.length - 1]?.label !== periods[lastIndex]) {
    ticks.push({ x: xForIndex(lastIndex, periods.length, dims), label: periods[lastIndex] });
  }
  return ticks;
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
  const [lo, hi] = domain;
  const ticks: ValueTick[] = [];
  for (let i = 0; i < count; i++) {
    const value = lo + ((hi - lo) * i) / (count - 1);
    ticks.push({ y: yForValue(value, domain, dims), label: value.toFixed(decimals) });
  }
  return ticks;
}
