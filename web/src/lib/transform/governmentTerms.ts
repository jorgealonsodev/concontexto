// Government terms, derived from the editorial event registry's `governments`
// group (indicator-page spec, "Annotation layers per PRD §6.1.1(a)"), so the
// chart can be sliced to one government's period the way it can already be
// sliced to a preset or to a reader-typed range.
//
// ── THE INFERENCE, STATED IN FULL ──────────────────────────────────────────
//
// Every entry in `config/gobiernos.yaml` carries `date_start` and NONE carries
// `date_end`. There is therefore no configured fact anywhere in this product
// that says when a government's term ended, and "slice the chart to Rajoy's
// term" cannot be answered without deciding one.
//
// This module decides it by SUCCESSION: government N runs until government
// N+1 takes office, and the most recent government is open-ended. That is
// correct for Spanish prime-ministerial succession, which is continuous by
// constitutional construction — the outgoing president remains in functions
// until the successor is sworn in, so consecutive terms leave no unattributed
// gap.
//
// It is still an INFERENCE, not data, and this project does not let inferences
// pass unmarked. `BreakConfig.DateStatus` refuses to project a break whose
// date is merely believed (`date_status: unconfirmed` + `todo`), and
// `reconocimientos.yaml` refuses to treat a well-argued draft as a human
// signature; in both cases the rule is the same — never present an unverified
// fact as a verified one. Refusing to derive anything here would mean the
// filter cannot exist at all, so the discipline is applied where it can be:
//
//   1. `GovernmentTerm.endKind` records WHERE the end came from
//      ("configured" | "succession" | "open"), so no consumer can mistake a
//      derived boundary for a registry fact;
//   2. a CONFIGURED `dateEnd` always wins — the inference never overwrites the
//      data it stands in for;
//   3. `succeededById` names the entry the boundary was read off, so the
//      derivation is traceable to its source rather than anonymous;
//   4. the island DISCLOSES the derived end to the reader in Spanish, beside
//      the chart it narrowed (`es.chart.government.derivedEndNote`).
//
// ── WHAT WOULD BREAK IT ────────────────────────────────────────────────────
//
//   • A genuine GAP between governments (a caretaker period modelled as an
//     absence, an interregnum): the gap would be silently absorbed into the
//     preceding term, which would then claim months it did not govern.
//   • A government MISSING from the registry: its whole term is absorbed into
//     its predecessor's, invisibly. This is not hypothetical —
//     `gobierno-suarez-1976` carries `date_status: unconfirmed` and is
//     deliberately never projected. It happens to be the FIRST entry, so
//     nothing absorbs it (the observations before 1981 simply belong to no
//     selectable term, and stay reachable through the full range and the
//     custom range); a missing MIDDLE entry would not be so harmless.
//   • Two governments recorded at the same period on a coarse axis: the
//     earlier one's window collapses to nothing and it drops out of the
//     control entirely (`availableGovernmentTerms`) rather than rendering an
//     empty chart.
//
// The remedy for all three is the same and lives outside this file: record the
// missing entry, or record a real `date_end`, in `config/gobiernos.yaml` —
// after which `endKind` becomes "configured" and this inference stops applying
// to it.
import { periodFromCalendarDate, periodOrdinalIndex, previousPeriod, type Frequency } from "../chart/periods";

/** The `governments` group's own event id, as the artifact and this product's
 * permalink both spell it. */
export const GOVERNMENTS_GROUP = "governments";

/** One editorial event as the export artifact delivers it (`EventRefSchema`),
 * narrowed to the fields a term needs. Deliberately structural rather than an
 * import of the island's own `IndicatorChartAnnotation`: this module is a pure
 * transform and must be callable from a test with a plain object literal. */
export interface GovernmentAnnotation {
  id: string;
  group: string;
  name: string;
  dateStart: string;
  dateEnd?: string | null;
}

/** Where a term's end date came from. The whole reason this type exists is so
 * a derived boundary can never be read as a configured one. */
export type TermEndKind = "configured" | "succession" | "open";

export interface GovernmentTerm {
  id: string;
  name: string;
  /** The registry's own `date_start`, verbatim — always a configured fact. */
  startDate: string;
  /** That date snapped onto the series' period axis, exactly as break bands
   * and annotations are already positioned (`periodFromCalendarDate`). */
  startPeriod: string;
  /** The last period INCLUDED in the term, or `null` when the term is open
   * (the most recent government: there is no end to state, and inventing one
   * would assert a resignation that has not happened). */
  endPeriod: string | null;
  endKind: TermEndKind;
  /** The successor whose start date the end was inferred from — non-null only
   * when `endKind` is "succession", so the inference is traceable. */
  succeededById: string | null;
}

/**
 * Derives one term per `governments` entry, in chronological order.
 *
 * Attribution is HALF-OPEN: term N covers `[start(N), start(N+1))`. A
 * president is sworn in mid-period and a period is a span, not an instant —
 * 2018-Q2 holds two months of Rajoy and one of Sánchez — so the handover
 * period has to go to one of them. It goes to the INCOMING government, which
 * is the only rule under which no observation belongs to two terms at once.
 * The alternative (inclusive ends) would show the same quarter under both
 * governments, and a reader comparing two terms would be double-counting it.
 *
 * Non-`governments` annotations are ignored here rather than by the caller, so
 * a shock or a milestone falling between two investitures can never be read as
 * a successor.
 */
export function deriveGovernmentTerms(
  annotations: readonly GovernmentAnnotation[],
  frequency: Frequency,
): GovernmentTerm[] {
  const governments = annotations
    .filter((a) => a.group === GOVERNMENTS_GROUP)
    // Sorted here, not assumed: the artifact's `events` array carries no
    // ordering contract, and a succession read off an unsorted list attributes
    // terms to the wrong governments with nothing on screen to notice it by.
    // ISO `YYYY-MM-DD` sorts lexicographically as it sorts chronologically.
    .slice()
    .sort((a, b) => (a.dateStart < b.dateStart ? -1 : a.dateStart > b.dateStart ? 1 : 0));

  return governments.map((government, index) => {
    const startPeriod = periodFromCalendarDate(government.dateStart, frequency);
    const successor = governments[index + 1];

    // A configured end is a FACT and always wins over the inference. Nothing
    // in `gobiernos.yaml` carries one today, but the schema admits it (that is
    // how an `exogenous` episode is closed), and the day one appears the
    // derived boundary must step aside rather than overwrite it.
    if (government.dateEnd) {
      return {
        id: government.id,
        name: government.name,
        startDate: government.dateStart,
        startPeriod,
        endPeriod: periodFromCalendarDate(government.dateEnd, frequency),
        endKind: "configured" as const,
        succeededById: null,
      };
    }

    if (!successor) {
      return {
        id: government.id,
        name: government.name,
        startDate: government.dateStart,
        startPeriod,
        endPeriod: null,
        endKind: "open" as const,
        succeededById: null,
      };
    }

    return {
      id: government.id,
      name: government.name,
      startDate: government.dateStart,
      startPeriod,
      // The period BEFORE the successor's own start period — the half-open
      // rule, expressed with the same period arithmetic every other transform
      // in this product uses rather than with a hand-rolled date subtraction.
      endPeriod: previousPeriod(periodFromCalendarDate(successor.dateStart, frequency), frequency),
      endKind: "succession" as const,
      succeededById: successor.id,
    };
  });
}

/** The observation labels a term would select, out of a series' own
 * (oldest-first) period list. Checked against the REAL observations rather
 * than against `[first, last]`, so a term landing inside a gap in the series'
 * cadence — `poblacion-residente`'s semiannual historical segment is the real
 * example — is correctly seen to select nothing. */
function selectedCount(term: GovernmentTerm, periods: readonly string[], frequency: Frequency): number {
  const fromOrdinal = periodOrdinalIndex(term.startPeriod, frequency);
  const toOrdinal = term.endPeriod === null ? Number.POSITIVE_INFINITY : periodOrdinalIndex(term.endPeriod, frequency);
  return periods.filter((period) => {
    const ordinal = periodOrdinalIndex(period, frequency);
    return ordinal >= fromOrdinal && ordinal <= toOrdinal;
  }).length;
}

/**
 * The subset of `terms` worth offering as a control for a series spanning
 * `periods`, in chronological order.
 *
 * A term is offered only when it both selects something AND narrows something:
 *
 *   1. selecting NO observation → absent. A control that renders an empty
 *      chart is a view of nothing.
 *   2. selecting EVERY observation → absent. It is "Todo el periodo" under a
 *      president's name: pressing it changes nothing, which reads as a broken
 *      control rather than as a filter.
 *
 * This is `isPresetAvailable`'s rule EXTENDED BY ANALOGY, not applied
 * literally, and the distinction is worth stating. The spec's own words —
 * "A preset whose start precedes the series' first observation MUST be absent,
 * not disabled" (series-transformations, "Range presets") — name the five
 * FIXED presets and state one specific disqualifying condition (a start
 * earlier than the series). A government term is not one of those presets, and
 * neither of the two conditions above is "its start precedes the series' first
 * observation" — a term can start long before the series and still be a
 * perfectly good filter (Aznar, 1996, over a series beginning in 2002). What
 * carries across is the REASON the spec gives for absence: a control identical
 * in effect to "full" is a confusing duplicate. `resolveCustomRange` already
 * extended the same rule by analogy in the other direction (applying it at
 * commit time to a range that has no start until the reader types one).
 */
export function availableGovernmentTerms(
  terms: readonly GovernmentTerm[],
  periods: readonly string[],
  frequency: Frequency,
): GovernmentTerm[] {
  if (periods.length === 0) return [];
  return terms.filter((term) => {
    const count = selectedCount(term, periods, frequency);
    return count > 0 && count < periods.length;
  });
}
