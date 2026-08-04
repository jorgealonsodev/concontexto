// Build-time generated textual description of a chart's main pattern
// (web-accessibility-gates spec, "Every chart has a textual description of
// its main pattern"; PRD §12.5's own worked example: "el paro sube de X a
// Y entre A y B, luego desciende…"). A pure function of the series' points
// — no DOM, no network — so it belongs under red-first testing per this
// project's own Strict-TDD convention for pure render/transform functions.
import { es } from "../../i18n/es";
import { formatNumber } from "../format/number";
import { formatPeriodProse } from "../format/period";
import type { ChartPoint } from "./geometry";

export interface DescribeSeriesInput {
  points: ChartPoint[];
  unit: string;
  decimals: number;
}

/** The description is Spanish prose a reader hears or reads, so its numbers
 * are formatted the same way every other reader-facing numeral on the site
 * is — "sube de 22.779 miles de personas", never "22779". */
function formatValue(value: number, decimals: number, unit: string): string {
  return `${formatNumber(value, decimals)} ${unit}`;
}

/** And the same argument one field over, for the dates in the same sentence.
 * This description is prose — PRD §12.5's own worked example is a sentence —
 * so it takes the PROSE register: "…sube de 71,8 índice en enero de 2002 a
 * …", never "en 2002-01", which is how the storage format was reaching a
 * screen reader. */
function formatPeriod(period: string): string {
  return formatPeriodProse(period);
}

function directionVerb(from: number, to: number): string {
  if (to > from) return es.chart.description.verbRise;
  if (to < from) return es.chart.description.verbFall;
  return es.chart.description.verbStable;
}

/**
 * Describes the series' principal pattern in one or two Spanish sentences:
 * start value/period, end value/period, and — when a genuine reversal
 * exists strictly between them — the turning point that produces it (PRD
 * §12.5's exact shape: "sube de X a Y entre A y B, y después desciende").
 *
 * The "turning point" is the interior global extremum (max or min) whose
 * combined deviation from both the start and end values is largest — i.e.
 * the point that would be LEAST well explained by a straight line from
 * start to end. When the direction before and after that point agrees (no
 * real reversal — e.g. a single interior wobble on an otherwise monotonic
 * climb), the description falls back to the simple two-point form, because
 * describing a non-reversal as one would be inventing structure the data
 * does not show.
 */
export function describeSeries(input: DescribeSeriesInput): string {
  const { points, unit, decimals } = input;
  const valid = points.filter((p): p is ChartPoint & { value: number } => p.value !== null);

  if (valid.length === 0) return es.chart.description.noData;
  if (valid.length === 1) {
    return es.chart.description.singlePoint(formatValue(valid[0].value, decimals, unit), formatPeriod(valid[0].period));
  }

  const start = valid[0];
  const end = valid[valid.length - 1];

  let maxPoint = valid[0];
  let minPoint = valid[0];
  for (const p of valid) {
    if (p.value > maxPoint.value) maxPoint = p;
    if (p.value < minPoint.value) minPoint = p;
  }

  const isInterior = (p: ChartPoint) => p.period !== start.period && p.period !== end.period;
  const candidates = [maxPoint, minPoint].filter(isInterior);

  if (candidates.length > 0) {
    const deviation = (p: ChartPoint & { value: number }) =>
      Math.abs(p.value - start.value) + Math.abs(p.value - end.value);
    const turning = candidates.reduce((best, candidate) => (deviation(candidate) > deviation(best) ? candidate : best));
    const turningValue = turning.value as number;

    const firstVerb = directionVerb(start.value, turningValue);
    const secondVerb = directionVerb(turningValue, end.value);

    const isRealReversal =
      firstVerb !== secondVerb &&
      firstVerb !== es.chart.description.verbStable &&
      secondVerb !== es.chart.description.verbStable;

    if (isRealReversal) {
      return es.chart.description.withTurningPoint(
        firstVerb,
        formatValue(start.value, decimals, unit),
        formatPeriod(start.period),
        formatValue(turningValue, decimals, unit),
        formatPeriod(turning.period),
        secondVerb,
        formatValue(end.value, decimals, unit),
        formatPeriod(end.period),
      );
    }
  }

  const overall = directionVerb(start.value, end.value);
  return es.chart.description.simple(
    overall,
    formatValue(start.value, decimals, unit),
    formatPeriod(start.period),
    formatValue(end.value, decimals, unit),
    formatPeriod(end.period),
  );
}
