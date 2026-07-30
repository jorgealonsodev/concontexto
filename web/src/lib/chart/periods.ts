// Period-label arithmetic shared by the chart geometry module and the
// transform functions (design.md D-5: "one shared geometry module"). Mirrors
// `app/internal/indicators/period.go`'s own String()/Next()/Previous()
// contract so a TypeScript period label round-trips identically to the Go
// domain type that produced it (`Point.period` in the export artifact) —
// deliberately re-implemented here rather than imported, since the Go
// package cannot cross the process boundary into a build-time TS module.
//
// Every configured series uses exactly one of these three label shapes:
// quarterly "YYYY-Qn" (tasa-de-paro-epa, ocupados-epa, pib, poblacion-
// residente), monthly "YYYY-MM" (ipc-general, ipc-subyacente), or annual
// "YYYY" (unused by the six milestone-1.2 series today, supported because
// the Go domain already models it).
export type Frequency = "M" | "Q" | "A";

export function stepsPerYear(frequency: Frequency): number {
  switch (frequency) {
    case "M":
      return 12;
    case "Q":
      return 4;
    case "A":
      return 1;
  }
}

export interface ParsedPeriod {
  year: number;
  ordinal: number;
}

const QUARTER_RE = /^(\d{4})-Q([1-4])$/;
const MONTH_RE = /^(\d{4})-(\d{2})$/;
const YEAR_RE = /^(\d{4})$/;

/** Parses a period label under an explicit frequency. Throws on a label
 * that does not match that frequency's own shape — a mismatch is a real
 * data-contract defect, never silently coerced (P4-style fail-closed). */
export function parsePeriod(label: string, frequency: Frequency): ParsedPeriod {
  switch (frequency) {
    case "Q": {
      const m = QUARTER_RE.exec(label);
      if (!m) throw new Error(`parsePeriod: "${label}" is not a valid quarterly period label`);
      return { year: Number(m[1]), ordinal: Number(m[2]) };
    }
    case "M": {
      const m = MONTH_RE.exec(label);
      if (!m) throw new Error(`parsePeriod: "${label}" is not a valid monthly period label`);
      return { year: Number(m[1]), ordinal: Number(m[2]) };
    }
    case "A": {
      const m = YEAR_RE.exec(label);
      if (!m) throw new Error(`parsePeriod: "${label}" is not a valid annual period label`);
      return { year: Number(m[1]), ordinal: 1 };
    }
  }
}

export function formatPeriod(year: number, ordinal: number, frequency: Frequency): string {
  switch (frequency) {
    case "Q":
      return `${year}-Q${ordinal}`;
    case "M":
      return `${year}-${String(ordinal).padStart(2, "0")}`;
    case "A":
      return `${year}`;
  }
}

/** A globally comparable integer ordinal — monotonically increasing with
 * time, safe to subtract for distance/ordering, never used as a display
 * value. */
export function periodOrdinalIndex(label: string, frequency: Frequency): number {
  const { year, ordinal } = parsePeriod(label, frequency);
  return year * stepsPerYear(frequency) + (ordinal - 1);
}

/** The label exactly one calendar year before `label` (used by year-on-year:
 * series-transformations spec, "the divisor is the population value at P").
 * Negative `deltaYears` moves backward. */
export function addYears(label: string, frequency: Frequency, deltaYears: number): string {
  const { year, ordinal } = parsePeriod(label, frequency);
  return formatPeriod(year + deltaYears, ordinal, frequency);
}

/** The immediately preceding period label (intra-annual / quarter-on-quarter
 * rate: PRD §7 #14, `pib`'s mandatory QoQ). */
export function previousPeriod(label: string, frequency: Frequency): string {
  const { year, ordinal } = parsePeriod(label, frequency);
  const steps = stepsPerYear(frequency);
  if (ordinal <= 1) return formatPeriod(year - 1, steps, frequency);
  return formatPeriod(year, ordinal - 1, frequency);
}

/** Maps a calendar date (break/event `date`/`dateStart`, "YYYY-MM-DD") onto
 * the series' own period label shape — used only to POSITION a break band
 * or annotation against the plotted period axis, never to invent a
 * denominator or observation. */
export function periodFromCalendarDate(dateISO: string, frequency: Frequency): string {
  const [yearStr, monthStr] = dateISO.split("-");
  const year = Number(yearStr);
  const month = Number(monthStr ?? "1");
  switch (frequency) {
    case "M":
      return formatPeriod(year, month, frequency);
    case "Q":
      return formatPeriod(year, Math.ceil(month / 3), frequency);
    case "A":
      return formatPeriod(year, 1, frequency);
  }
}

/** The index into `periods` whose ordinal distance to `targetLabel` is
 * smallest — used to snap a break/event calendar date onto the nearest
 * actually-plotted period when the series' own cadence (e.g.
 * poblacion-residente's historical semiannual segment) has no observation
 * at the exact calendar period. Ties resolve to the earlier index. Returns
 * -1 for an empty `periods` list. */
export function nearestPeriodIndex(periods: string[], frequency: Frequency, targetLabel: string): number {
  if (periods.length === 0) return -1;
  const targetOrdinal = periodOrdinalIndex(targetLabel, frequency);
  let bestIndex = 0;
  let bestDistance = Infinity;
  periods.forEach((period, index) => {
    const distance = Math.abs(periodOrdinalIndex(period, frequency) - targetOrdinal);
    if (distance < bestDistance) {
      bestDistance = distance;
      bestIndex = index;
    }
  });
  return bestIndex;
}
