// Attaches each derived (YoY/QoQ/per-capita) point's provisional/definitive
// status by looking it up from the RAW observation at the same period
// (indicator-page spec, "Every point discloses provisional or definitive"
// — this MUST still hold once a transformation is active; series-
// transformations spec, "two encodings... distinguished by a legend and a
// non-colour channel"). `RatePoint`/`PerCapitaPoint` (transform/yoy.ts,
// transform/perCapita.ts) deliberately carry no status of their own — a
// derived value is not itself "provisional" or "definitive", the
// OBSERVATION it is derived from is — so the chart borrows the raw point's
// status for its marker/dash encoding, never inventing a third state.
import type { ChartPoint, ObservationStatus } from "./geometry";

export interface DerivedTransformPoint {
  period: string;
  value: number;
}

/** A period absent from `statusByPeriod` defaults to definitive rather than
 * throwing — this should never happen (every derived point's period is
 * drawn from the raw series' own periods), but a missing status must never
 * block rendering. */
export function toChartPoints(
  points: DerivedTransformPoint[],
  statusByPeriod: Map<string, ObservationStatus>,
): ChartPoint[] {
  return points.map((p) => ({
    period: p.period,
    value: p.value,
    status: statusByPeriod.get(p.period) ?? "D",
  }));
}
