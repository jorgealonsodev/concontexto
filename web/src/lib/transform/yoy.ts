// Year-on-year and intra-annual (period-on-period) rate transforms
// (series-transformations spec, "Transformations for the six milestone-1.2
// series" — MANDATORY year-on-year for `ipc-general`, `ipc-subyacente` and
// `pib`; `pib` additionally requires the quarter-on-quarter rate, PRD §7
// #14). Pure functions, no DOM — slice 7 computes them, slice 8 lets the
// reader toggle them (design.md D-5).
import { addYears, previousPeriod, type Frequency } from "../chart/periods";

export interface RateTransformPoint {
  period: string;
  value: number | null;
}

export interface RatePoint {
  period: string;
  /** Percentage points, e.g. `2.3` means +2.3%. */
  value: number;
}

function computeRate(
  points: RateTransformPoint[],
  frequency: Frequency,
  priorLabel: (period: string) => string,
): RatePoint[] {
  const byPeriod = new Map<string, number>();
  for (const p of points) {
    if (p.value !== null) byPeriod.set(p.period, p.value);
  }
  const out: RatePoint[] = [];
  for (const p of points) {
    if (p.value === null) continue;
    const prior = byPeriod.get(priorLabel(p.period));
    // series-transformations spec, "Transformations never invent data": a
    // rate is NEVER emitted for a period with no same-period-prior-year
    // (or no immediately-prior-period) observation — absent, not zero.
    if (prior === undefined || prior === 0) continue;
    out.push({ period: p.period, value: (p.value / prior - 1) * 100 });
  }
  return out;
}

/** Year-on-year rate: each point compares against the same period exactly
 * one year earlier. */
export function computeYoY(points: RateTransformPoint[], frequency: Frequency): RatePoint[] {
  return computeRate(points, frequency, (period) => addYears(period, frequency, -1));
}

/** Intra-annual (period-on-period) rate — quarter-on-quarter for `pib`
 * (PRD §7 #14's second mandatory rate), month-on-month for a monthly
 * series. Named generically because the same arithmetic applies regardless
 * of frequency; each configured series picks its own applicable set
 * (series-transformations spec's applicability table). */
export function computeIntraPeriodRate(points: RateTransformPoint[], frequency: Frequency): RatePoint[] {
  return computeRate(points, frequency, (period) => previousPeriod(period, frequency));
}
