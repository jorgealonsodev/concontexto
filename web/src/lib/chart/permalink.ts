// series-transformations spec, "Range presets": "Selecting a range preset
// updates the permalink" / "loading that permalink reproduces the same
// range". Pure query-string encode/decode — no DOM, no `window`; the island
// component owns reading/writing `window.location`, this module only
// serializes plain state. Encodes both the active range preset AND the
// active transform (a shared link should reproduce the exact view a reader
// was looking at, not only the range half the spec's own scenario names).
import { CUSTOM_RANGE, GOVERNMENT_RANGE, type RangeSelection } from "../transform/sliceRange";
import type { TransformKind } from "./toggleState";

/** The custom range's own bounds, as PERIOD LABELS ("2010-Q1"), never as the
 * calendar dates the reader's date inputs speak. Period labels are the unit
 * the rest of the product computes in, so the permalink states the range the
 * chart is actually showing rather than a date that must be re-snapped to a
 * period on every read. */
export interface CustomRangeBounds {
  from: string;
  to: string;
}

export interface ChartUrlState {
  range: RangeSelection;
  transform: TransformKind;
  /** Meaningful only when `range` is `"custom"`; ignored (and never emitted)
   * for a fixed preset, whose bounds are entirely derivable from the series'
   * own span. */
  custom?: CustomRangeBounds | null;
  /** The selected government's EDITORIAL EVENT ID (`gobierno-rajoy-2011`),
   * meaningful only when `range` is `"government"`.
   *
   * The id, and never the `[from, to]` window it resolves to, because that
   * window is INFERRED from the succession (`lib/transform/governmentTerms`).
   * Encoding the window would freeze today's inference into every shared link:
   * a later registry correction — a confirmed `date_end`, a government finally
   * recorded — would leave old links pointing at a span nobody would derive
   * again, with nothing to indicate they had gone stale. The id survives that
   * correction; the window is re-derived on every load.
   *
   * A machine surface like every other parameter here: the raw config id,
   * never the reader-facing name and never a formatted year range. */
  government?: string | null;
}

const DEFAULT_STATE: { range: RangeSelection; transform: TransformKind } = { range: "full", transform: "raw" };

/** Encodes non-default state only, so the common case (default range,
 * default transform) yields a clean, empty query string rather than a
 * permalink cluttered with redundant params.
 *
 * A `"custom"` range with no usable bounds degrades to the default range
 * rather than emitting `range=custom` on its own: that URL would decode to
 * an unrenderable selection, and a permalink that cannot reproduce the view
 * it was copied from is worse than one that reproduces the default. */
export function encodeChartState(state: ChartUrlState): string {
  const params = new URLSearchParams();
  const custom = state.range === CUSTOM_RANGE && state.custom?.from && state.custom?.to ? state.custom : null;
  // The same degrade-rather-than-emit rule the custom range follows, for the
  // same reason: `range=government` with no id decodes to a selection that
  // cannot be rendered, and a permalink that fails to reproduce the view it
  // was copied from is worse than one that reproduces the default.
  const government = state.range === GOVERNMENT_RANGE && state.government ? state.government : null;
  const unusable = (state.range === CUSTOM_RANGE && !custom) || (state.range === GOVERNMENT_RANGE && !government);
  const range = unusable ? DEFAULT_STATE.range : state.range;
  if (range !== DEFAULT_STATE.range) params.set("range", range);
  if (custom) {
    params.set("from", custom.from);
    params.set("to", custom.to);
  }
  if (government) params.set("government", government);
  if (state.transform !== DEFAULT_STATE.transform) params.set("transform", state.transform);
  const search = params.toString();
  return search ? `?${search}` : "";
}

/** Parses a query string (with or without a leading `?`), restricted to the
 * caller-supplied `availableRanges`/`availableTransforms` — an unrecognised,
 * garbage or currently-unavailable value (e.g. a preset that does not apply
 * to this series' own span) silently falls back to the default rather than
 * producing an invalid or misleading view.
 *
 * `range=custom` additionally requires BOTH bounds to be present; a
 * half-specified custom range falls back to the default like any other
 * unusable value. The bounds themselves are returned VERBATIM and
 * unvalidated: this module is a serializer with no access to the series, so
 * it cannot tell an out-of-span pair from an in-span one and must not
 * pretend to. `resolveCustomRange` (`lib/transform/sliceRange`) is the single
 * place that judges them, and the island falls back to the full range when it
 * rejects — which is what makes a hand-edited permalink degrade safely rather
 * than crash or render an empty chart. */
export function decodeChartState(
  search: string,
  availableRanges: readonly RangeSelection[],
  availableTransforms: readonly TransformKind[],
): ChartUrlState {
  const params = new URLSearchParams(search.startsWith("?") ? search.slice(1) : search);
  const range = params.get("range");
  const transform = params.get("transform");
  const from = params.get("from");
  const to = params.get("to");
  const government = params.get("government");

  const selection: RangeSelection =
    range && (availableRanges as readonly string[]).includes(range) ? (range as RangeSelection) : DEFAULT_STATE.range;
  const custom = selection === CUSTOM_RANGE && from && to ? { from, to } : null;
  // Returned VERBATIM and unvalidated, exactly like the custom bounds above:
  // this module has no access to the series, so it cannot know which
  // governments overlap it. `availableGovernmentTerms` is the single place
  // that judges, and the island degrades to the full range when it refuses —
  // which is what makes a hand-edited permalink fail safely.
  const selectedGovernment = selection === GOVERNMENT_RANGE && government ? government : null;
  const usable = selection === CUSTOM_RANGE ? custom !== null : selection === GOVERNMENT_RANGE ? selectedGovernment !== null : true;

  const state: ChartUrlState = {
    range: usable ? selection : DEFAULT_STATE.range,
    transform:
      transform && (availableTransforms as readonly string[]).includes(transform)
        ? (transform as TransformKind)
        : DEFAULT_STATE.transform,
  };
  // Set conditionally rather than always assigning `null`: callers that
  // compare a decoded state against a plain `{range, transform}` object (as
  // this module's own pre-custom-range tests do) must keep passing unchanged.
  if (custom) state.custom = custom;
  if (selectedGovernment) state.government = selectedGovernment;
  return state;
}
