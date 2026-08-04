// Where a POLICY MEASURE's date of entry into force falls against a series'
// own period axis — the positioning half of the chart's fifth annotation
// treatment, and the only one deliberately drawn OUTSIDE the plot area.
//
// ── THE PROBLEM THIS LAYER HAD TO SOLVE FIRST ──────────────────────────────
//
// The registry says which instrument entered into force on which date. It
// says NOTHING about what followed, and the site must not either — not in
// words, not in a figure, and not by arrangement. That last one is the hard
// part, and it is a drawing problem rather than a copy problem: a rule
// crossing the data at the exact period a curve turns asserts an effect by
// adjacency, with no author and no source a reader could check. No caption
// underneath undoes it.
//
// So the mark is confined to the bottom margin — under the x-axis tick
// labels, a second track on the calendar, never inside the plot. That is not
// a compromise reached for want of room. It is the exact statement the
// registry supports: a date of entry into force is a fact about the CALENDAR,
// and the calendar is the axis. The mark points at a date. It touches no
// value, spans no interval on the data, and shades nothing, so there is no
// arrangement from which an effect can be read.
//
// ── WHY THIS IS NOT A FIFTH VISUAL LANGUAGE ────────────────────────────────
//
// The drawing already speaks four, and colour is never what separates them:
//
//   dotted grey ALONG THE DATA PATH + diamond
//       "this observation is provisional" (RESERVED — theme.css's own
//        header; the dash is unavailable here and to everything else)
//   filled translucent VERTICAL COLUMN, one period step wide
//       "the series changed methodology here"
//   thin solid VERTICAL rule, plot-height, capped by a triangle flag
//       "a different government took office here"
//   solid HORIZONTAL rail near the top of the plot, capped by end serifs
//       "the editorial registry dates this event from here to here"
//
// The obvious fifth — another vertical rule, in another colour — was rejected
// outright: the change-of-government marker is ALREADY "a vertical rule at an
// instant", so a second one differing only by hue would leave colour as the
// sole channel, which PRD §12.5 and ADR-8 both forbid, and would teach one
// reader two meanings for one shape.
//
// What separates this mark is REGION, which a reader resolves before shape
// and long before colour: it is the only annotation in the whole drawing that
// lives outside the plot. Against the government rule specifically there are
// three separations at once — outside the plot against inside it, a stub
// against the full plot height, and separated from the axis by the entire row
// of tick labels rather than ending on it. That separation is load-bearing
// rather than decorative: an investiture and a measure landing on the same
// period would otherwise draw one continuous line through the axis and
// collapse two codes into one.
//
// It carries `--color-event-span`, the same ink the rails use, and that is
// deliberate rather than economical. Both marks are the editorial registry
// speaking about a stretch of the calendar; one colour for one voice, with
// region, orientation and subject (an instant against an interval) carrying
// the distinction. Colour is the second channel here as it is everywhere else
// in this drawing — discard it entirely and the five treatments are still
// five.
//
// ── WHAT THIS MODULE CANNOT DO, BY CONSTRUCTION ────────────────────────────
//
// It receives an id, a group, a name and a date. There is no effect, no
// magnitude, no direction and no "periods after" anywhere in its input,
// because there is none anywhere in the registry, the table or the artifact.
// It computes a single x and two y's in the gutter. Nothing here can be
// extended into a claim about the data without first adding a field to a
// registry that deliberately has none.
import { instantOrdinalInWindow, periodWindow } from "./annotationWindow";
import { plotArea, xForIndex, type ChartDimensions } from "./geometry";
import { nearestPeriodIndex, periodFromCalendarDate, periodOrdinalIndex, type Frequency } from "./periods";

/** The annotation group whose entries this module draws. Named once so the
 * filter, the toggle and the chip cannot drift apart. */
export const MEASURES_GROUP = "measures";

/** One editorial event as the export artifact delivers it, narrowed to the
 * fields a mark needs. Structural rather than an import of the island's own
 * `IndicatorChartAnnotation`, for the same reason `GovernmentChangeAnnotation`
 * is: this is a pure transform and must be callable from a test with a plain
 * object literal. */
export interface PolicyMeasureAnnotation {
  id: string;
  group: string;
  name: string;
  dateStart: string;
}

export interface PolicyMeasure {
  id: string;
  /** The instrument's name, from the registry, printed verbatim — never
   * translated, never shortened, never reworded. */
  name: string;
  /** The registry's own `date_start`: the date of ENTRY INTO FORCE, verbatim.
   * Kept beside `period` rather than replaced by it, because the two answer
   * different questions — the sentence beside the chart states the real
   * calendar date, while the mark can only stand on a period the series
   * actually observed. */
  dateStart: string;
  /** The PLOTTED period the mark stands on: the nearest observation to the
   * date, not necessarily the date's own period. */
  period: string;
  /** Its index into the `periods` list handed in — what `xForIndex` needs. */
  index: number;
}

export interface PolicyMeasureMark extends PolicyMeasure {
  /** The x of `index`, in the box's own user units. */
  x: number;
  /** Top of the stub: below the plot area's bottom edge, and below the
   * x-axis tick labels' own rendered boxes. */
  y1: number;
  /** Bottom of the stub, inside the viewBox. */
  y2: number;
  strokeWidth: number;
}

/** How much of the band BELOW the x-axis labels is spent on the gap above the
 * mark, and how much on the mark itself. The remainder is clear space before
 * the viewBox's own bottom edge.
 *
 * Fractions of the MEASURED band rather than multiples of the type size,
 * because the two variants do not scale together: the wide box sets 10-unit
 * ticks 16 units below the axis inside a 32-unit bottom margin, the narrow one
 * sets 20-unit ticks 28 units below inside a 56-unit margin. A constant ratio
 * of the font size that fitted one would overrun the other.
 *
 * WHY BELOW THE LABELS AND NOT BETWEEN THEM AND THE AXIS, which is where this
 * mark started. Measured in Chromium rather than estimated: at the wide box's
 * own type the strip between the axis line and the labels' rendered boxes is
 * about 5 units, and a stub short enough to fit it was drawn THROUGH the
 * numerals anyway, because a `<text>` element's box reaches roughly a full em
 * above its baseline, not the three quarters an ink estimate assumes. The band
 * under the labels is twice as deep and completely empty.
 *
 * It is also the better place on the merits, which is why this was not a
 * retreat. A second track under the calendar is a legible shape — the reader
 * meets the axis, then the dates, then the marks that sit on those dates —
 * and it puts the whole layer one row further from the series, which is the
 * direction this particular annotation should always err in. */
const GAP_SHARE = 0.15;
const LENGTH_SHARE = 0.7;

/** How far a tick label's rendered box reaches BELOW its own baseline, as a
 * fraction of the type size. Conservative on purpose, exactly like
 * `GLYPH_ADVANCE_RATIO` in `geometry.ts`: over-reserving costs a fraction of a
 * unit of mark, under-reserving draws a mark through a descender. */
const DESCENDER_RATIO = 0.45;

/** Stub thickness, as a multiple of the type size. Matched to the data
 * line's own 2 units at the wide size — heavy enough to read as a mark
 * rather than as a stray rule, and it is the whole of the mark, so it cannot
 * afford to be thinner. */
const STROKE_RATIO = 0.2;

/**
 * The policy measures a chart of `periods` may honestly mark, oldest first.
 *
 * `annotations` arrives ALREADY filtered to the groups the reader has chosen
 * to see — whose choice that is belongs to the caller, exactly as it does for
 * the government markers and the event rails. Everything else — the group
 * filter, the window rule and the snap — is decided here, once, so the static
 * component and the island cannot disagree about the same measure.
 *
 * THE WINDOW RULE IS THE GOVERNMENT MARKER'S, not the event rail's, and the
 * difference matters. A rail covers an interval, so it may legitimately
 * OVERLAP a window it is not contained in. A measure is an instant: it either
 * falls inside the span on screen or it did not happen there, and
 * `nearestPeriodIndex` would otherwise pin a 2012 instrument to the left edge
 * of a 2020-2026 chart and put a mark under a date at which nothing was
 * enacted. Narrowing the range therefore re-runs this filter with no extra
 * logic — a measure outside the window disappears because its date is no
 * longer inside the span on screen.
 *
 * Sorted here rather than assumed: the artifact's `events` array carries no
 * ordering contract, and the generated sentence reads left to right.
 */
export function selectPolicyMeasures(
  annotations: readonly PolicyMeasureAnnotation[],
  periods: readonly string[],
  frequency: Frequency,
): PolicyMeasure[] {
  const window = periodWindow(periods, frequency);
  if (window === null) return [];
  const mutablePeriods = [...periods];

  return annotations
    .filter((a) => a.group === MEASURES_GROUP)
    .slice()
    // ISO `YYYY-MM-DD` sorts lexicographically as it sorts chronologically.
    // The id breaks the tie so two instruments entering into force on one
    // date are ordered deterministically rather than by the artifact's own
    // array order.
    .sort((a, b) =>
      a.dateStart < b.dateStart ? -1 : a.dateStart > b.dateStart ? 1 : a.id < b.id ? -1 : a.id > b.id ? 1 : 0,
    )
    .flatMap((annotation) => {
      const targetLabel = periodFromCalendarDate(annotation.dateStart, frequency);
      const targetOrdinal = periodOrdinalIndex(targetLabel, frequency);
      // The same INSTANT rule the government markers apply, and now literally
      // the same expression: `annotationWindow` owns it, so the mark and the
      // chip below the chart cannot disagree about the same measure.
      if (!instantOrdinalInWindow(targetOrdinal, window)) return [];
      const index = nearestPeriodIndex(mutablePeriods, frequency, targetLabel);
      return [
        {
          id: annotation.id,
          name: annotation.name,
          dateStart: annotation.dateStart,
          period: mutablePeriods[index],
          index,
        },
      ];
    });
}

/**
 * The same measures, each carrying the geometry its gutter stub is drawn
 * with. One function per concern, exactly as `governmentMarkers` and
 * `eventSpans` split them: `renderChartSVG` needs the coordinates, while the
 * generated description needs only the names and dates and has no box to
 * compute against.
 *
 * `xTickLabelOffset` is an input rather than a constant because it is what
 * actually bounds the gutter: the axis line is the top of the usable space
 * and the tick labels' ink is the bottom, and the second is set by that
 * offset and the tick type size together. Deriving the stub from them is what
 * keeps a mark and a numeral from being drawn through one another on a box
 * neither this module nor its caller has seen.
 */
export function buildPolicyMeasureMarks(
  annotations: readonly PolicyMeasureAnnotation[],
  periods: readonly string[],
  frequency: Frequency,
  dims: ChartDimensions,
  tickFontSize: number,
  xTickLabelOffset: number,
): PolicyMeasureMark[] {
  const area = plotArea(dims);
  // The band this mark lives in: from the bottom of the tick labels' own
  // rendered boxes down to the viewBox's own bottom edge.
  const labelsBottom = area.y1 + xTickLabelOffset + tickFontSize * DESCENDER_RATIO;
  // Floored at zero so a pathological box produces a zero-length mark rather
  // than one drawn upwards into the labels or, worse, into the plot — the one
  // place this mark must never reach.
  const band = Math.max(dims.height - labelsBottom, 0);
  const gap = band * GAP_SHARE;
  const length = band * LENGTH_SHARE;
  return selectPolicyMeasures(annotations, periods, frequency).map((measure) => ({
    ...measure,
    x: xForIndex(measure.index, periods.length, dims),
    y1: labelsBottom + gap,
    y2: labelsBottom + gap + length,
    strokeWidth: tickFontSize * STROKE_RATIO,
  }));
}
