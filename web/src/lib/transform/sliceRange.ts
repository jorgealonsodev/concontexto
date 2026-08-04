// Range-preset slicing (series-transformations spec, "Range presets": full
// series default + 5 años / 10 años / desde 2008 / desde 2018 /
// personalizado; "A preset whose start precedes the series' first
// observation MUST be absent, not disabled"). Pure function, no DOM;
// permalink encoding and the visible controls are slice 8's UI concern —
// this module only decides WHICH presets apply and WHAT they slice to.
import { formatPeriod, parsePeriod, periodOrdinalIndex, type Frequency } from "../chart/periods";

export const RANGE_PRESETS = ["full", "5y", "10y", "since-2008", "since-2018"] as const;
export type RangePreset = (typeof RANGE_PRESETS)[number];

/** The spec's "personalizado" entry (verify-report WARNING-5). Deliberately
 * NOT a member of `RANGE_PRESETS`: every preset in that enum is fully
 * determined by the series' own span, whereas a custom range carries a
 * reader-supplied `from`/`to` pair that has to travel with it — through the
 * component's state and through the permalink. Modelling it as a sixth enum
 * member would force every consumer of `RangePreset` to carry an
 * only-sometimes-meaningful pair of bounds alongside it. */
export const CUSTOM_RANGE = "custom";

/** One government's own term (indicator-page spec, "Annotation layers per PRD
 * §6.1.1(a)" — the `governments` event group). Kept out of `RANGE_PRESETS` for
 * exactly the reason `CUSTOM_RANGE` is: it carries a payload the enum members
 * do not — the selected government's editorial event id — and that id has to
 * travel with the selection through the component's state and through the
 * permalink.
 *
 * It is a THIRD kind rather than a variant of `CUSTOM_RANGE` because its
 * bounds are not the reader's: they are derived from the editorial registry's
 * succession (`lib/transform/governmentTerms`), which is an inference the
 * product has to keep distinguishable from a range a reader typed. It reuses
 * `sliceCustomRange` to do the actual slicing, so there is one slicing
 * primitive here, not two. */
export const GOVERNMENT_RANGE = "government";

/** Everything the chart can currently be ranged BY: one of the fixed presets,
 * the custom `[from, to]` selection, or one government's derived term. */
export type RangeSelection = RangePreset | typeof CUSTOM_RANGE | typeof GOVERNMENT_RANGE;

export interface RangeTransformPoint {
  period: string;
}

/** The preset's own start period label, or `null` for "full" (no lower
 * bound at all). `latestPeriod` anchors the two relative presets (5/10
 * años, counted back from the series' own latest point, never from
 * "today" — a statically built page has no "today"). */
function presetStartPeriod(preset: RangePreset, frequency: Frequency, latestPeriod: string): string | null {
  switch (preset) {
    case "full":
      return null;
    case "5y":
    case "10y": {
      const years = preset === "5y" ? 5 : 10;
      const { year, ordinal } = parsePeriod(latestPeriod, frequency);
      return formatPeriod(year - years, ordinal, frequency);
    }
    case "since-2008":
      return formatPeriod(2008, 1, frequency);
    case "since-2018":
      return formatPeriod(2018, 1, frequency);
  }
}

/**
 * A preset is available only when its own start does not precede the
 * series' first observation (spec: absent, never rendered disabled) — a
 * preset identical in effect to "full" for a shorter series would be a
 * confusing duplicate control, not a meaningful narrower view.
 */
export function isPresetAvailable(
  preset: RangePreset,
  frequency: Frequency,
  firstPeriod: string,
  latestPeriod: string,
): boolean {
  if (preset === "full") return true;
  const start = presetStartPeriod(preset, frequency, latestPeriod);
  if (start === null) return true;
  return periodOrdinalIndex(start, frequency) >= periodOrdinalIndex(firstPeriod, frequency);
}

/** The subset of `presets` that apply to a series spanning
 * `firstPeriod`–`latestPeriod`, in `RANGE_PRESETS` order, "full" always
 * included first. */
export function availablePresets(
  frequency: Frequency,
  firstPeriod: string,
  latestPeriod: string,
  presets: readonly RangePreset[] = RANGE_PRESETS,
): RangePreset[] {
  return presets.filter((preset) => isPresetAvailable(preset, frequency, firstPeriod, latestPeriod));
}

/** Slices `points` (assumed already ordered oldest-to-newest) to the given
 * preset. "full" and an unavailable preset both return every point
 * unchanged — callers are expected to consult `isPresetAvailable` before
 * offering a preset as a control in the first place. */
export function sliceRange<T extends RangeTransformPoint>(
  points: T[],
  frequency: Frequency,
  preset: RangePreset,
): T[] {
  if (points.length === 0) return points;
  const latestPeriod = points[points.length - 1].period;
  const start = presetStartPeriod(preset, frequency, latestPeriod);
  if (start === null) return points;
  const startOrdinal = periodOrdinalIndex(start, frequency);
  return points.filter((p) => periodOrdinalIndex(p.period, frequency) >= startOrdinal);
}

/** A custom, caller-supplied `[from, to]` range (the "personalizado" preset).
 * The pure slicing primitive; `resolveCustomRange` below is what decides
 * whether a given pair may be sliced with in the first place, and every
 * caller is expected to go through it rather than calling this directly with
 * unvalidated bounds. */
export function sliceCustomRange<T extends RangeTransformPoint>(
  points: T[],
  frequency: Frequency,
  from: string,
  to: string,
): T[] {
  const fromOrdinal = periodOrdinalIndex(from, frequency);
  const toOrdinal = periodOrdinalIndex(to, frequency);
  return points.filter((p) => {
    const ordinal = periodOrdinalIndex(p.period, frequency);
    return ordinal >= fromOrdinal && ordinal <= toOrdinal;
  });
}

/** Why a reader-entered range cannot be rendered as asked.
 *
 * `no-observations` covers three physically distinct cases that all reduce to
 * the same honest statement to the reader — the range holds no data — namely
 * a range wholly before the series, wholly after it, or landing entirely in a
 * gap in the series' own cadence (e.g. `poblacion-residente`'s semiannual
 * historical segment). Splitting them would multiply the copy without
 * changing what the reader can do about it. */
export type CustomRangeRejection = "incomplete" | "unparseable" | "inverted" | "no-observations";

export type CustomRangeResolution =
  | { status: "ok"; from: string; to: string; clamped: boolean }
  | { status: "rejected"; reason: CustomRangeRejection };

/** `periodOrdinalIndex`, but returning `null` instead of throwing.
 *
 * `parsePeriod` throws by design on a label that does not match the
 * frequency — the right behaviour for the export artifact's own data, where a
 * mismatch is a genuine data-contract defect (P4 fail-closed). A custom range
 * arrives from two places that are NOT internal data: two date inputs and a
 * query string a reader can hand-edit to anything at all. For those, a throw
 * would take the whole island down; a rejection is the correct fail-closed
 * response. */
function tryPeriodOrdinal(label: string, frequency: Frequency): number | null {
  try {
    return periodOrdinalIndex(label, frequency);
  } catch {
    return null;
  }
}

/**
 * Decides whether a reader-entered `[from, to]` range can be rendered, and on
 * what terms (series-transformations spec, "Range presets" — the
 * "personalizado" entry; verify-report WARNING-5).
 *
 * The spec states the absence rule only for the FIXED presets: "a preset
 * whose start precedes the series' first observation MUST be absent, not
 * disabled". A free-form range has no start until the reader types one, so
 * the rule cannot be applied at render time; this function is its equivalent,
 * applied at commit time, and it resolves the three cases as follows.
 *
 *   1. PARTLY outside the span → clamped to the span, `clamped: true`.
 *      The years outside the span do not exist, so clamping is the only
 *      renderable outcome; the flag exists so the caller can DISCLOSE the
 *      narrowing in the UI. A silent clamp would be the free-form equivalent
 *      of the disabled preset the spec forbids: a control that appears to
 *      honour the request while quietly showing something else.
 *   2. Containing not one observation → rejected. This is the direct
 *      analogue of "absent, not disabled" — an empty chart is a view of
 *      nothing, so the request is refused and the current view is kept.
 *      Checking against the actual observation list, not merely against
 *      `[first, last]`, is what also catches a range landing inside a
 *      cadence gap.
 *   3. Inverted, blank or unparseable → rejected; there is no defensible
 *      view to render at all.
 *
 * `periods` is the series' own observation labels, ordered oldest-first (the
 * RAW series, not a transformed view — a custom range is a statement about
 * the series' history, and must not silently change meaning when the reader
 * toggles a transformation, exactly as a fixed preset does not).
 */
export function resolveCustomRange(
  periods: readonly string[],
  frequency: Frequency,
  from: string,
  to: string,
): CustomRangeResolution {
  if (periods.length === 0) return { status: "rejected", reason: "no-observations" };
  if (from.trim() === "" || to.trim() === "") return { status: "rejected", reason: "incomplete" };

  const fromOrdinal = tryPeriodOrdinal(from, frequency);
  const toOrdinal = tryPeriodOrdinal(to, frequency);
  if (fromOrdinal === null || toOrdinal === null) return { status: "rejected", reason: "unparseable" };
  if (fromOrdinal > toOrdinal) return { status: "rejected", reason: "inverted" };

  // `periods` is ordered oldest-first by contract, so the span's bounds are
  // its ends — no scan needed, and no assumption that the labels sort
  // lexicographically (they do not, for every frequency).
  const firstLabel = periods[0];
  const lastLabel = periods[periods.length - 1];
  const firstOrdinal = periodOrdinalIndex(firstLabel, frequency);
  const lastOrdinal = periodOrdinalIndex(lastLabel, frequency);

  const clampedFrom = fromOrdinal < firstOrdinal ? firstLabel : from;
  const clampedTo = toOrdinal > lastOrdinal ? lastLabel : to;
  const clamped = fromOrdinal < firstOrdinal || toOrdinal > lastOrdinal;

  const lowOrdinal = Math.max(fromOrdinal, firstOrdinal);
  const highOrdinal = Math.min(toOrdinal, lastOrdinal);
  const covers = periods.some((period) => {
    const ordinal = periodOrdinalIndex(period, frequency);
    return ordinal >= lowOrdinal && ordinal <= highOrdinal;
  });
  if (!covers) return { status: "rejected", reason: "no-observations" };

  return { status: "ok", from: clampedFrom, to: clampedTo, clamped };
}

/** The first calendar day of a period, as an ISO "YYYY-MM-DD" string.
 *
 * The inverse of `periodFromCalendarDate` (`lib/chart/periods`), and it lives
 * here rather than beside it because it exists for exactly one reason: the
 * custom range's own two native `<input type="date">` controls speak calendar
 * dates, while everything else in this product speaks period labels. It is
 * used to PREFILL those inputs (and to set their `min`/`max`) when a custom
 * permalink is reopened.
 *
 * Deliberately lossy in one direction and exactly stable in the other: a
 * period maps to its FIRST day, so `2015-Q4` prefills as `2015-10-01` rather
 * than as any later day inside that quarter. Feeding that date back through
 * `periodFromCalendarDate` returns `2015-Q4` again, so the reader can reopen
 * a permalink and press the commit control without the range moving under
 * them. */
export function periodStartCalendarDate(label: string, frequency: Frequency): string {
  const { year, ordinal } = parsePeriod(label, frequency);
  const month = frequency === "Q" ? (ordinal - 1) * 3 + 1 : frequency === "M" ? ordinal : 1;
  return `${String(year).padStart(4, "0")}-${String(month).padStart(2, "0")}-01`;
}
