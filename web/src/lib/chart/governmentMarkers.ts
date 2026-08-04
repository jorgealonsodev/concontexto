// Where a change of government falls against a series' own period axis —
// the positioning half of the chart's third vertical treatment (PRD
// §6.1.1(a)'s `governments` event group, drawn ON the chart rather than only
// listed as a chip beside it).
//
// ── WHY THIS REUSES THE BREAK BAND'S MECHANISM RATHER THAN INVENTING ONE ───
//
// A government's start is a CALENDAR date; the x axis is ordinal by data-point
// RANK (`xForIndex` — deliberately, so an irregular cadence never has to
// invent a position for a period nobody observed). That is exactly the
// mismatch `buildBreakBands` already solves, and it solves it with two shared
// functions: `periodFromCalendarDate` maps the date onto the series' own label
// shape, and `nearestPeriodIndex` snaps that label onto the nearest period the
// series really carries. Both are used here, unchanged. A second mapping would
// be a second place for a marker and a band to disagree about the same date.
//
// ── THE ONE RULE THIS MODULE ADDS, AND THE LIE IT PREVENTS ─────────────────
//
// `nearestPeriodIndex` snaps ANY date, including one decades outside the
// series: over `tasa-de-paro-epa` (2002-2026) it maps Aznar's 1996 investiture
// onto 2002-Q1. Four governments overlap that series but only THREE changes
// happen inside it, and a marker at the left edge would assert a change of
// government in 2002 that did not occur. So a change is marked only when its
// snapped period falls inside `[first, last]` of the periods handed in — the
// same window rule `ChartIsland`'s `visibleBreaks` already applies to breaks.
//
// The filter lives HERE, inside the one function both renderers call, rather
// than at each call site: the static component and the island would otherwise
// each have to remember it, and the one that forgot would ship the lie.
//
// That single rule also answers the range question by construction. The island
// passes the CURRENTLY VISIBLE periods, so narrowing the chart re-runs this
// filter against the new window with no extra logic: a marker outside the
// window disappears because its date is no longer inside the span on screen.
// Under the government filter itself the selected term begins at its own
// investiture and the successor's falls outside it (attribution is half-open —
// see `lib/transform/governmentTerms`), so AT MOST one marker survives, on the
// left edge, naming the boundary the reader selected. That edge marker is the
// point, not an artefact.
//
// "At most" and not "exactly", measured rather than assumed: on
// /indicador/poblacion-residente — semiannual before 2021 — Rajoy's 2011-Q4
// investiture falls in a gap in the cadence, so the term's first OBSERVED
// period is 2012-Q1 and NO marker is drawn. That is correct rather than a
// miss: the change of government did not happen inside the span on screen, and
// a rule on that first point would place the boundary a quarter after the real
// one. The contiguous quarterly series keep their edge marker.
//
// ── WHAT IS DELIBERATELY NOT HERE ─────────────────────────────────────────
//
// No colour, no party, no ordering by anything but time. `config/gobiernos.
// yaml` and `EventConfig` carry no party field (PRD §12.1), so there is
// nothing to encode even if this module wanted to.
//
// An UNCONFIRMED investiture never reaches this module at all:
// `gobierno-suarez-1976` carries `date_status: unconfirmed` and is never
// projected to the database, so it is absent from the artifact's `events` and
// therefore absent from the `annotations` prop. A series beginning before the
// first CONFIRMED government (`poblacion-residente`, 1971) simply carries no
// marker over that stretch. Inventing one there would be exactly the
// unverified-fact-as-verified-fact failure `date_status` exists to prevent.
import { GOVERNMENTS_GROUP } from "../transform/governmentTerms";
import { xForIndex, type ChartDimensions } from "./geometry";
import { nearestPeriodIndex, periodFromCalendarDate, periodOrdinalIndex, type Frequency } from "./periods";

/** One editorial event as the export artifact delivers it, narrowed to the
 * fields a marker needs. Structural rather than an import of the island's own
 * `IndicatorChartAnnotation`, for the same reason `GovernmentAnnotation` in
 * `lib/transform/governmentTerms` is: this is a pure transform and must be
 * callable from a test with a plain object literal. */
export interface GovernmentChangeAnnotation {
  id: string;
  group: string;
  name: string;
  dateStart: string;
}

export interface GovernmentChange {
  id: string;
  /** The president's name, from the registry, printed verbatim — never
   * translated, never reworded. */
  name: string;
  /** The registry's own `date_start`, verbatim. */
  dateStart: string;
  /** Its four-digit year — the same label `AnnotationChip` already prints for
   * a government, so the marker and the chip name the same event the same
   * way. Every ISO date begins with its year. */
  year: string;
  /** The PLOTTED period the marker is drawn on: the nearest observation to
   * the investiture, not necessarily the investiture's own period. */
  period: string;
  /** Its index into the `periods` list handed in — what `xForIndex` needs. */
  index: number;
}

export interface GovernmentMarker extends GovernmentChange {
  /** The x of `index`, in the box's own user units. */
  x: number;
}

/**
 * The changes of government a chart of `periods` may honestly mark, oldest
 * first.
 *
 * Sorted here rather than assumed: the artifact's `events` array carries no
 * ordering contract, and the generated sentence that names these markers reads
 * left to right. ISO `YYYY-MM-DD` sorts lexicographically as it sorts
 * chronologically — the same argument `deriveGovernmentTerms` makes.
 *
 * Two investitures snapping onto the SAME plotted period (possible on a coarse
 * annual axis) both survive, and both are drawn. Merging them would hide one
 * change of government behind another; two coincident rules is a legible
 * consequence of the axis' resolution, not an error to paper over.
 */
export function selectGovernmentChanges(
  annotations: readonly GovernmentChangeAnnotation[],
  periods: readonly string[],
  frequency: Frequency,
): GovernmentChange[] {
  if (periods.length === 0) return [];
  const firstOrdinal = periodOrdinalIndex(periods[0], frequency);
  const lastOrdinal = periodOrdinalIndex(periods[periods.length - 1], frequency);
  const mutablePeriods = [...periods];

  return annotations
    .filter((a) => a.group === GOVERNMENTS_GROUP)
    .slice()
    .sort((a, b) => (a.dateStart < b.dateStart ? -1 : a.dateStart > b.dateStart ? 1 : 0))
    .flatMap((annotation) => {
      const targetLabel = periodFromCalendarDate(annotation.dateStart, frequency);
      const targetOrdinal = periodOrdinalIndex(targetLabel, frequency);
      // Inclusive at both edges — see this file's header for why the edge
      // case is the point rather than a rounding artefact.
      if (targetOrdinal < firstOrdinal || targetOrdinal > lastOrdinal) return [];
      const index = nearestPeriodIndex(mutablePeriods, frequency, targetLabel);
      return [
        {
          id: annotation.id,
          name: annotation.name,
          dateStart: annotation.dateStart,
          year: annotation.dateStart.slice(0, 4),
          period: mutablePeriods[index],
          index,
        },
      ];
    });
}

/** The same changes, each carrying the x its rule is drawn at. One function
 * per concern: `renderChartSVG` needs the geometry, while the generated
 * description needs only the names and years and has no box to compute
 * against. */
export function buildGovernmentMarkers(
  annotations: readonly GovernmentChangeAnnotation[],
  periods: readonly string[],
  frequency: Frequency,
  dims: ChartDimensions,
): GovernmentMarker[] {
  return selectGovernmentChanges(annotations, periods, frequency).map((change) => ({
    ...change,
    x: xForIndex(change.index, periods.length, dims),
  }));
}
