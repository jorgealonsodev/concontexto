// Where an editorial event's PERIOD falls against a series' own period axis —
// the positioning half of the chart's fourth annotation treatment (PRD
// §6.1.1(b)/(c)'s `exogenous` and `milestones` event groups, drawn ON the
// chart rather than only listed as a chip beside it).
//
// ── WHY A FOURTH TREATMENT IS SAFE, WHEN A FOURTH WAS THE RISK ─────────────
//
// The drawing already speaks three visual languages, and every one of them was
// chosen so that colour is never the channel that separates them:
//
//   dotted grey ALONG THE DATA PATH + diamond marker
//       "this observation is provisional; the source may still revise it"
//       (RESERVED — theme.css's own header; the dash is unavailable here)
//   filled translucent VERTICAL COLUMN, one period step wide
//       "the series changed methodology here; the two sides are not directly
//        comparable"
//   thin solid VERTICAL rule capped by a triangle flag
//       "a different government took office here"
//
// What this module adds is different in KIND from all three, and the drawing
// says so structurally rather than chromatically:
//
//   solid HORIZONTAL rail near the top of the plot, capped by end serifs
//       "the editorial registry dates this event from here to here"
//
// Against the provisional dash: the rail is SOLID and drawn ACROSS the plot,
// never along the data path. `test/design-system/reserved-semantics.test.ts`
// enforces that a dash in this chart means provisional data and nothing else.
// Against the break band: a stroke, not a fill — a band shades a REGION
// because a rupture affects the data on both of its sides, while a rail marks
// an EXTENT without touching the values under it. Against the change-of-
// government rule: the dominant axis is horizontal where that one is vertical,
// which is the one property a reader reads before any other. Discard colour
// entirely and the four are still four.
//
// It does carry a colour of its own (`--color-event-span`, declared in both
// themes and measured by the contrast harness) — but as a SECOND channel,
// never the first, exactly as ADR-8 permits and PRD §12.5 requires.
//
// ── THE WINDOW RULE, AND HOW IT DIFFERS FROM THE GOVERNMENT MARKER'S ───────
//
// `nearestPeriodIndex` snaps ANY date onto the nearest plotted period,
// including one decades outside the series — which is why a change of
// government is marked only when its date falls INSIDE the plotted span.
// An interval needs the weaker test, and needs it to stay honest: the
// 2008-2013 financial crisis genuinely covers 2010-2013 of a series that
// begins in 2010, so refusing to draw it would hide a real overlap. So an
// event is projected when its interval INTERSECTS the plotted window, and the
// drawn ends are clamped to that window.
//
// A clamped end is then recorded (`clampedStart`/`clampedEnd`) rather than
// forgotten, because the drawing has to treat it differently: an uncapped rail
// running to the edge of the plot says "this continues beyond what is on
// screen", while a serif says "the event began/ended here". Losing that
// distinction would turn the window's edge into a claim about the calendar.
//
// ── WHAT AN EVENT WITH NO END DATE LOOKS LIKE, AND WHY ─────────────────────
//
// Nothing. `shock-energetico-2022` and `ngeu-primer-desembolso` carry a
// `date_start` and no `date_end` — the registry says the event began and says
// nothing about when it stopped. Running the rail to the series' last
// observation would invent an end; capping it one period after the start would
// assert the event lasted one quarter; borrowing the government marker's
// vertical rule would teach one reader two meanings for one code. So an event
// with no recorded end has no period to project, and the chip below the chart
// — which prints a single year rather than a range — remains the whole
// statement. The sentence this layer generates says so in its own words
// ("...que el registro editorial acota con fecha de inicio y de fin"), so the
// absence is disclosed rather than silent.
import { xForIndex, plotArea, type ChartDimensions } from "./geometry";
import { nearestPeriodIndex, periodFromCalendarDate, periodOrdinalIndex, type Frequency } from "./periods";

/** One editorial event as the export artifact delivers it, narrowed to the
 * fields a span needs. Structural rather than an import of the island's own
 * `IndicatorChartAnnotation`, for the same reason `GovernmentChangeAnnotation`
 * is: this is a pure transform and must be callable from a test with a plain
 * object literal. */
export interface EventSpanAnnotation {
  id: string;
  group: string;
  name: string;
  dateStart: string;
  dateEnd?: string | null;
}

export interface EventSpan {
  id: string;
  group: string;
  /** The event's name, from the registry, printed verbatim — never
   * translated, never reworded. */
  name: string;
  /** The FIRST plotted period the rail covers. Equal to the series' own first
   * period when the event began before the chart does. */
  startPeriod: string;
  /** The LAST plotted period the rail covers, under the same rule. */
  endPeriod: string;
  startIndex: number;
  endIndex: number;
  /** True when the event's own start predates the plotted window, so the rail
   * must NOT be capped at that end — the edge of the plot is the edge of what
   * is on screen, not the beginning of the event. */
  clampedStart: boolean;
  /** The same, at the other end. */
  clampedEnd: boolean;
  /** 0 for the topmost rail. Overlapping spans get successive lanes so that
   * neither hides the other; spans that do not overlap share lane 0, which is
   * what keeps the common case one rail deep. */
  lane: number;
}

export interface EventSpanRail extends EventSpan {
  /** Left end of the rail, in the box's own user units. */
  x1: number;
  /** Right end. Always greater than `x1` — a span that snaps onto a single
   * plotted period is widened to one period step rather than collapsed. */
  x2: number;
  /** The rail's own y. Horizontal by construction: there is no `y2`. */
  y: number;
  /** Length of the vertical end serifs, which turn DOWN into the plot so the
   * eye projects the interval onto the data below it. */
  serifLength: number;
  strokeWidth: number;
}

/** Rail offset below the plot area's top edge, as a multiple of the tick type
 * size. Everything in this module scales with that size rather than being a
 * constant, because the narrow box draws its type at twice the wide box's and
 * a constant would render the phone reader a hairline.
 *
 * 1.2 rather than something smaller because of a REAL collision, not a
 * hypothetical one: the change-of-government flag is a triangle 7 units tall
 * hanging from the same top edge, so at the wide variant's type size anything
 * under 0.7 puts a rose rail through an ink flag. 1.2 gives 12 units against
 * that 7, which clears it with room for the rail's own thickness. */
const RAIL_OFFSET_RATIO = 1.2;
/** Vertical drop of each end serif, same units. */
const SERIF_RATIO = 0.6;
/** Distance between two stacked lanes, same units. Comfortably larger than
 * `SERIF_RATIO` so a lower rail never touches the serif above it. */
const LANE_STEP_RATIO = 1.1;
/** Rail thickness, same units. Thick enough to read as a rail rather than as
 * a stray axis line, thin enough never to be mistaken for a filled band. */
const STROKE_RATIO = 0.2;

/**
 * The editorial events a chart of `periods` may honestly project, oldest
 * first, each with the lane it is drawn in.
 *
 * `annotations` arrives ALREADY filtered to the groups the reader has chosen
 * to see — which groups those are is the caller's state, not this module's
 * (the static component reads its own defaults; the island reads its live
 * toggle state). Everything else — the end-date rule, the window rule, the
 * clamping and the stacking — is decided here, once, so the two consumers
 * cannot disagree about the same event.
 *
 * Sorted here rather than assumed: the artifact's `events` array carries no
 * ordering contract, and both the lane packing and the generated sentence read
 * left to right.
 */
export function selectEventSpans(
  annotations: readonly EventSpanAnnotation[],
  periods: readonly string[],
  frequency: Frequency,
): EventSpan[] {
  if (periods.length === 0) return [];
  const firstOrdinal = periodOrdinalIndex(periods[0], frequency);
  const lastOrdinal = periodOrdinalIndex(periods[periods.length - 1], frequency);
  const mutablePeriods = [...periods];

  const candidates = annotations
    .filter((a): a is EventSpanAnnotation & { dateEnd: string } => Boolean(a.dateEnd))
    .slice()
    // ISO `YYYY-MM-DD` sorts lexicographically as it sorts chronologically.
    // The id breaks the tie so two events beginning on one date are ordered
    // deterministically rather than by the artifact's own array order.
    .sort((a, b) =>
      a.dateStart < b.dateStart ? -1 : a.dateStart > b.dateStart ? 1 : a.id < b.id ? -1 : a.id > b.id ? 1 : 0,
    )
    .flatMap((annotation): Omit<EventSpan, "lane">[] => {
      const startLabel = periodFromCalendarDate(annotation.dateStart, frequency);
      const endLabel = periodFromCalendarDate(annotation.dateEnd, frequency);
      const startOrdinal = periodOrdinalIndex(startLabel, frequency);
      const endOrdinal = periodOrdinalIndex(endLabel, frequency);
      // Fail closed (P4) on a registry entry whose end predates its start:
      // silently swapping the bounds would draw a span nobody configured.
      if (endOrdinal < startOrdinal) return [];
      // Intersection, not containment — see this file's header for why an
      // interval needs the weaker test and a change of government does not.
      if (endOrdinal < firstOrdinal || startOrdinal > lastOrdinal) return [];

      const clampedStart = startOrdinal < firstOrdinal;
      const clampedEnd = endOrdinal > lastOrdinal;
      // `nearestPeriodIndex` is monotone in the target ordinal, so an
      // unclamped end can never snap to the left of an unclamped start.
      const startIndex = clampedStart ? 0 : nearestPeriodIndex(mutablePeriods, frequency, startLabel);
      const endIndex = clampedEnd
        ? mutablePeriods.length - 1
        : nearestPeriodIndex(mutablePeriods, frequency, endLabel);
      return [
        {
          id: annotation.id,
          group: annotation.group,
          name: annotation.name,
          startPeriod: mutablePeriods[startIndex],
          endPeriod: mutablePeriods[endIndex],
          startIndex,
          endIndex,
          clampedStart,
          clampedEnd,
        },
      ];
    });

  // Lane packing: the first lane whose previous span ENDS BEFORE this one
  // starts. Two spans meeting on a single plotted period share that period's
  // x, so they count as overlapping — merging them there would draw one rail
  // through a point that belongs to two different events.
  const laneLastIndex: number[] = [];
  return candidates.map((span) => {
    let lane = laneLastIndex.findIndex((last) => last < span.startIndex);
    if (lane === -1) {
      lane = laneLastIndex.length;
      laneLastIndex.push(span.endIndex);
    } else {
      laneLastIndex[lane] = span.endIndex;
    }
    return { ...span, lane };
  });
}

/** The same spans, each carrying the geometry its rail is drawn with. One
 * function per concern, exactly as `governmentMarkers` splits them:
 * `renderChartSVG` needs the coordinates, while the generated description
 * needs only the names and periods and has no box to compute against. */
export function buildEventSpanRails(
  annotations: readonly EventSpanAnnotation[],
  periods: readonly string[],
  frequency: Frequency,
  dims: ChartDimensions,
  tickFontSize: number,
): EventSpanRail[] {
  const area = plotArea(dims);
  const step = periods.length > 1 ? area.width / (periods.length - 1) : area.width;
  return selectEventSpans(annotations, periods, frequency).map((span) => {
    let x1 = xForIndex(span.startIndex, periods.length, dims);
    let x2 = xForIndex(span.endIndex, periods.length, dims);
    if (span.startIndex === span.endIndex) {
      // A rail of zero length is a rail nobody can see. One period step —
      // half either side, the same arithmetic `buildBreakBands` uses — is the
      // width of the period the event falls in at this axis' own resolution,
      // not an invented extent.
      x1 -= step / 2;
      x2 += step / 2;
    }
    return {
      ...span,
      x1: Math.max(area.x0, x1),
      x2: Math.min(area.x1, x2),
      y: area.y0 + tickFontSize * RAIL_OFFSET_RATIO + span.lane * tickFontSize * LANE_STEP_RATIO,
      serifLength: tickFontSize * SERIF_RATIO,
      strokeWidth: tickFontSize * STROKE_RATIO,
    };
  });
}
