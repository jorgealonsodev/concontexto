// Whether an editorial annotation belongs to the stretch of calendar a chart
// is currently showing — the ONE rule, in the one place, for every consumer
// that has to answer that question.
//
// ── WHY THIS MODULE EXISTS ─────────────────────────────────────────────────
//
// It did not, and the cost was measurable in a browser. The rule was written
// out three times, once inside each selector that needed it
// (`selectGovernmentChanges`, `selectPolicyMeasures`, `selectEventSpans`), and
// a FOURTH consumer had no copy at all: the annotation chips below the chart
// listed every entry in the group, unconditionally. So on
// /indicador/tasa-de-paro-epa narrowed to "Desde 2018", the drawing marked two
// policy measures and the chip row underneath it listed three — including a
// 2012 reform, presented to the reader as though it were in view. The
// governments and exogenous groups did the same thing, and `governments` did
// it even at the FULL range: six chips over three rules, because three of the
// six investitures predate the series' own first observation.
//
// Three copies of a rule plus one consumer that forgot it is not an accident
// that happened once. This module is the fix for the shape of the problem
// rather than for the one symptom: the three selectors now READ their rule
// from here instead of restating it, so there is exactly one expression per
// rule and a fifth consumer cannot invent a sixth answer.
//
// ── THE TWO RULES, AND WHY THERE ARE TWO ───────────────────────────────────
//
// An annotation is one of two shapes, and the honest test differs:
//
//   AN INSTANT — a change of government, a policy measure's entry into force,
//   or an event the registry gave a start and no end. It is IN RANGE when its
//   own date falls inside the span on screen, inclusive at both edges. The
//   weaker test would be a lie here: `nearestPeriodIndex` snaps ANY date onto
//   the nearest plotted period, so admitting a 2012 instrument to a 2020-2026
//   chart pins it to the left edge and puts a mark under a date at which
//   nothing was enacted.
//
//   AN INTERVAL — an event the registry bounded with BOTH dates. It is in
//   range when it INTERSECTS the span on screen, never requiring containment:
//   the 2008-2013 crisis genuinely covers 2010-2013 of a series that begins in
//   2010, and refusing it would hide a real overlap.
//
// Both rules were already load-bearing before this module; neither is new
// here. What is new is that the chips now ask the same question the marks
// have always asked, and get the same answer by construction rather than by
// two implementations agreeing.
//
// ── WHY AN END-LESS EVENT IS AN INSTANT AND NOT A ONE-SIDED INTERVAL ───────
//
// Because the registry says so by saying nothing. `shock-energetico-2022` and
// `ngeu-primer-desembolso` carry a `date_start` and no `date_end`: the event
// began, and the registry makes no claim about when it stopped. Running the
// interval to the series' last observation would invent an end date, which is
// exactly the unverified-fact-as-verified-fact failure `date_status` exists to
// prevent. `selectEventSpans` already draws these as nothing at all — no end,
// no period to project, no rail. The chip is therefore the whole statement
// about such an event, and the only date it can honestly be judged against is
// the one date it has.
//
// A consequence worth stating outright, because it looks like a bug and is
// not: in the `exogenous` group a reader can see MORE chips than rails. Four
// chips and two rails at the full range on /indicador/tasa-de-paro-epa. The
// two extra are the end-less pair, in range as instants, projecting nothing —
// and the generated sentence beside the chart accounts for the difference in
// its own words, naming only the events the registry "acota con fecha de
// inicio y de fin".
import { periodFromCalendarDate, periodOrdinalIndex, type Frequency } from "./periods";

/** The stretch of calendar a chart is showing, in the ordinal space
 * `periodOrdinalIndex` defines — the two ENDS of the series on screen, never a
 * count of the observations between them. A gap in the cadence must not narrow
 * the window: /indicador/poblacion-residente is semiannual before 2021, and a
 * rule derived from the observation count would move its edges. */
export interface PeriodWindow {
  firstOrdinal: number;
  lastOrdinal: number;
}

/** One editorial event as the export artifact delivers it, narrowed to the
 * only two fields the window rule reads. Structural rather than an import of
 * the island's own `IndicatorChartAnnotation`, for the same reason every other
 * transform in `lib/chart` is: this is a pure function and must be callable
 * from a test with a plain object literal. */
export interface WindowedAnnotation {
  dateStart: string;
  dateEnd?: string | null;
}

/**
 * The window a list of plotted periods defines, or `null` when there is no
 * series to define one.
 *
 * Null rather than a degenerate window, because the two answers differ where
 * it matters: a caller that maps over annotations returns `[]` for an empty
 * series, and a caller that FILTERS must reject everything rather than admit
 * everything. An empty-but-valid window would quietly do the second.
 */
export function periodWindow(periods: readonly string[], frequency: Frequency): PeriodWindow | null {
  if (periods.length === 0) return null;
  return {
    firstOrdinal: periodOrdinalIndex(periods[0], frequency),
    lastOrdinal: periodOrdinalIndex(periods[periods.length - 1], frequency),
  };
}

/**
 * Whether an INSTANT falls inside the window. Inclusive at both edges — see
 * `governmentMarkers.ts`'s header for why the edge case is the point rather
 * than a rounding artefact: under the government filter the selected term
 * begins at its own investiture, and that left-edge marker is what shows the
 * reader where the window they chose came from.
 */
export function instantOrdinalInWindow(ordinal: number, window: PeriodWindow): boolean {
  return ordinal >= window.firstOrdinal && ordinal <= window.lastOrdinal;
}

/**
 * Whether an INTERVAL intersects the window.
 *
 * Fails closed (P4) on an interval whose end predates its start: silently
 * swapping the bounds would admit a span nobody configured.
 */
export function intervalOrdinalsInWindow(
  startOrdinal: number,
  endOrdinal: number,
  window: PeriodWindow,
): boolean {
  if (endOrdinal < startOrdinal) return false;
  return endOrdinal >= window.firstOrdinal && startOrdinal <= window.lastOrdinal;
}

/**
 * Whether one annotation belongs to the stretch of calendar `periods` covers,
 * dispatching on its own shape: an entry with both dates is an interval, an
 * entry with only a start is an instant.
 *
 * This is the predicate the CHIPS ask, and the whole point of it is that it
 * cannot answer differently from the marks: the same two comparisons above
 * are the ones the three selectors run.
 */
export function annotationInWindow(
  annotation: WindowedAnnotation,
  periods: readonly string[],
  frequency: Frequency,
): boolean {
  const window = periodWindow(periods, frequency);
  if (window === null) return false;
  return annotationInPeriodWindow(annotation, window, frequency);
}

/** The same judgement against an ALREADY-DERIVED window, so a caller filtering
 * a list resolves the series' two ends once rather than once per entry. */
function annotationInPeriodWindow(
  annotation: WindowedAnnotation,
  window: PeriodWindow,
  frequency: Frequency,
): boolean {
  const startOrdinal = periodOrdinalIndex(periodFromCalendarDate(annotation.dateStart, frequency), frequency);
  if (!annotation.dateEnd) return instantOrdinalInWindow(startOrdinal, window);
  const endOrdinal = periodOrdinalIndex(periodFromCalendarDate(annotation.dateEnd, frequency), frequency);
  return intervalOrdinalsInWindow(startOrdinal, endOrdinal, window);
}

/**
 * The entries of `annotations` the window covers, in the order handed in.
 *
 * Order is preserved rather than sorted: this list is rendered as the chip
 * row, and the artifact's own order is the order that row already printed.
 * Selecting is the whole job — the three selectors sort because their outputs
 * feed a sentence that reads left to right, and a chip row has no such
 * contract to keep.
 */
export function annotationsInWindow<T extends WindowedAnnotation>(
  annotations: readonly T[],
  periods: readonly string[],
  frequency: Frequency,
): T[] {
  const window = periodWindow(periods, frequency);
  if (window === null) return [];
  return annotations.filter((annotation) => annotationInPeriodWindow(annotation, window, frequency));
}
