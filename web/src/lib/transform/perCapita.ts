// Per-capita transform (series-transformations spec, "Per capita uses the
// resident population at each observation's own date" / "Per capita exists
// only where the denominator genuinely exists"; PRD §6.1.2, principle P2).
// Pure function, no DOM.
export interface PerCapitaTransformPoint {
  period: string;
  value: number | null;
}

export interface PerCapitaPoint {
  period: string;
  value: number;
}

export interface PerCapitaCoverage {
  from: string;
  to: string;
}

export interface PerCapitaResult {
  /** `null` when no non-empty covered sub-span exists — the per-capita
   * control does not exist at all in that case (spec: "absent, not
   * disabled"), never rendered with an empty result. */
  coverage: PerCapitaCoverage | null;
  points: PerCapitaPoint[];
}

/**
 * Divides `points` by `population` at each observation's OWN period —
 * never the latest population value, never interpolated or carried
 * forward (spec: "It MUST NOT interpolate, extrapolate or otherwise invent
 * a missing denominator period"). The result is restricted to the maximal
 * CONTIGUOUS sub-span (within the indicator's own period sequence) where a
 * same-period population observation exists for every period in that span
 * — design.md D-5's "coverage" rule, computed here rather than assumed.
 */
export function computePerCapita(
  points: PerCapitaTransformPoint[],
  population: PerCapitaTransformPoint[],
): PerCapitaResult {
  const populationByPeriod = new Map<string, number>();
  for (const p of population) {
    if (p.value !== null) populationByPeriod.set(p.period, p.value);
  }

  const indicatorPeriods = points.map((p) => p.period);
  const rawByPeriod = new Map<string, number>();
  for (const p of points) {
    if (p.value === null) continue;
    const denominator = populationByPeriod.get(p.period); // same period only
    if (denominator === undefined || denominator === 0) continue;
    rawByPeriod.set(p.period, p.value / denominator);
  }

  if (rawByPeriod.size === 0) return { coverage: null, points: [] };

  // Longest contiguous run of covered periods against the indicator's OWN
  // period sequence (not the calendar) — a gap in coverage breaks the run.
  let bestStart = -1;
  let bestLength = 0;
  let currentStart = -1;
  let currentLength = 0;
  indicatorPeriods.forEach((period, index) => {
    if (rawByPeriod.has(period)) {
      if (currentLength === 0) currentStart = index;
      currentLength++;
      if (currentLength > bestLength) {
        bestLength = currentLength;
        bestStart = currentStart;
      }
    } else {
      currentLength = 0;
    }
  });

  if (bestLength === 0) return { coverage: null, points: [] };

  const coverage: PerCapitaCoverage = {
    from: indicatorPeriods[bestStart],
    to: indicatorPeriods[bestStart + bestLength - 1],
  };
  const spanPoints: PerCapitaPoint[] = indicatorPeriods
    .slice(bestStart, bestStart + bestLength)
    .filter((period) => rawByPeriod.has(period))
    .map((period) => ({ period, value: rawByPeriod.get(period) as number }));

  return { coverage, points: spanPoints };
}
