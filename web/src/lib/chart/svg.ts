// The ONE renderer both the static build (`IndicatorChart.astro`, slice 7)
// and the interactive island (`ChartIsland.svelte`, slice 8) call — design.md
// D-5's "never two renderers that must agree" applied literally: rather than
// an Astro-only SVG template and a separate Svelte-only one that a golden
// test merely COMPARES, there is exactly one string-serializing function
// producing the exact same markup for both consumers. `svg.test.ts`'s golden
// fixture pins this function's own output; slice 8's island-parity test
// (task 8.12) re-asserts the SAME golden against the island's initial
// client-side render.
//
// Break bands are baked into this function's own output (indicator-page
// spec, "Series breaks are always visible and never dismissible" — P4): a
// reader with JavaScript disabled receives the exact same band markup a
// JavaScript-enabled reader does, because there is no separate JS-only band
// layer to omit.
import {
  NARROW_MAX_X_TICKS,
  NARROW_TICK_FONT_SIZE,
  NARROW_X_TICK_LABEL_OFFSET,
  WIDE_TICK_FONT_SIZE,
  buildBreakBands,
  buildLineSegments,
  buildXTicks,
  buildYTicks,
  narrowDimensions,
  plotArea,
  projectPoints,
  valueDomain,
  wideDimensions,
  type ChartDimensions,
  type ChartPoint,
} from "./geometry";
import { buildEventSpanRails, type EventSpanAnnotation } from "./eventSpans";
import { buildGovernmentMarkers, type GovernmentChangeAnnotation } from "./governmentMarkers";
import { buildPolicyMeasureMarks, type PolicyMeasureAnnotation } from "./measureMarks";
import {
  NARROW_ANNOTATION_LABEL_FONT_SIZE,
  WIDE_ANNOTATION_LABEL_FONT_SIZE,
  placeAnnotationLabels,
  type PlacedAnnotationLabel,
} from "./annotationLabels";
import { formatPeriodProse } from "../format/period";
// A measure's `<title>` states the DAY the instrument entered into force, not
// the period its mark snapped onto — see `measureMarks.ts` for why the day is
// the whole content of that annotation.
import { formatCalendarDate } from "../format/date";
import type { Frequency } from "./periods";
// The marker's `<title>` is read by a person, so its words live where every
// other reader-facing string does. (The `<title>` two elements above — "
// Evolución de la serie" — predates this module's i18n import and is left
// where it is rather than moved as a side effect of this change.)
import { es } from "../../i18n/es";

export interface ChartBreakInput {
  key: string;
  date: string;
}

export interface RenderChartSVGInput {
  points: ChartPoint[];
  breaks: ChartBreakInput[];
  /** The editorial registry's `governments` entries, ALREADY filtered to the
   * annotation groups the reader has chosen to see — exactly the contract
   * `eventSpans` below has always had, and no longer the "pass everything,
   * always draw" one this input started with.
   *
   * WHY IT CHANGED. The marker used to be drawn on every chart unconditionally,
   * which made it the one annotation layer a reader could not turn off. The
   * `governments` group already has a visible toggle beside the chart — the
   * same control that shows and hides its chips — so the drawing now answers
   * that control instead of ignoring it. Nothing is drawn unless it is
   * selected.
   *
   * WHICH of the entries handed in may honestly be marked is still
   * `buildGovernmentMarkers`' single decision, made once, inside the renderer
   * both consumers share. Omitted (or empty), the drawing carries no marker. */
  governmentChanges?: GovernmentChangeAnnotation[];
  /** The editorial events whose PERIOD may be projected onto the plot — the
   * chart's fourth annotation treatment.
   *
   * Unlike `governmentChanges` above, this list arrives ALREADY filtered to
   * the annotation groups the reader has chosen to see, because that choice is
   * the caller's own state and not a property of the series: the static
   * component reads its default-open map, the island reads its live toggle
   * state. Everything else — the end-date rule, the window rule, the clamping
   * and the stacking — is `buildEventSpanRails`' single decision, made inside
   * the renderer both consumers share. Omitted (or empty), the drawing carries
   * no rail and is byte-identical to what it was before this layer existed. */
  eventSpans?: EventSpanAnnotation[];
  /** The editorial POLICY MEASURES whose date of entry into force may be
   * marked on the time axis — the chart's fifth annotation treatment, and the
   * only one drawn outside the plot area.
   *
   * Same contract as the two layers above: this list arrives ALREADY filtered
   * to the annotation groups the reader has chosen to see, because that
   * choice is the caller's state and not a property of the series. Which of
   * the entries handed in may honestly be marked, and where, is
   * `buildPolicyMeasureMarks`' single decision inside the renderer both
   * consumers share. Omitted (or empty), the drawing carries no mark and is
   * byte-identical to what it was before this layer existed — which is what
   * keeps the committed golden fixture unchanged. */
  policyMeasures?: PolicyMeasureAnnotation[];
  frequency: Frequency;
  decimals: number;
  unit: string;
  /** Stable ids so the SVG's `<title>`/`aria-describedby` can reference the
   * generated textual description and the accessible data table
   * (web-accessibility-gates spec, "programmatically associated with its
   * chart"). Caller-supplied so two charts on one page (e.g. the
   * light/dark workbench showcase) never collide. */
  titleId: string;
  descriptionId: string;
  tableId: string;
  /** The box to draw into. Omitted, the wide box is DERIVED for this exact
   * series (`wideDimensions`) rather than taken from a constant — so a caller
   * that says nothing gets margins that fit its own labels, which is what the
   * clipped y axis on /indicador/poblacion-residente cost us. */
  dims?: ChartDimensions;
  /** Tick type size in user units. Defaults to the value this function has
   * always emitted, so an omitting caller gets byte-identical output. */
  tickFontSize?: number;
  /** Distance from the plot area's bottom edge down to the x-label baseline,
   * in user units. Same defaulting contract as `tickFontSize`: it scales
   * with the type, so a variant that enlarges one must move the other. */
  xTickLabelOffset?: number;
  /** Ceiling on x-axis ticks. `buildXTicks` may still append one more to
   * guarantee the final period is labelled. */
  maxXTicks?: number;
  /** Type size for the on-drawing annotation labels, in user units.
   *
   * Its own input rather than a multiple of `tickFontSize`, because the two
   * answer different questions: the tick size is what the chart is READ
   * against, the label size is what fits beside a rule without a second name
   * being pushed off the plot. In the wide box they happen to agree at 10; in
   * the narrow box they deliberately do not (14 against 20 — see
   * `annotationLabels.ts` for the measurement that fixes it). */
  annotationLabelFontSize?: number;
  /** Distinguishes this rendering from the other one on the same page.
   *
   * Both variants are emitted into a single document (CSS shows one), which
   * would otherwise duplicate every `data-testid` and every `class` in the
   * drawing. `"narrow"` suffixes every test id and adds a modifier class;
   * omitting it — the wide variant — changes nothing, which is why every
   * locator written before this variant existed still matches exactly the
   * node it always matched. */
  variant?: "narrow";
}

/** Everything `renderChartSVG` needs to draw the narrow-viewport variant of
 * one series, spread into a call beside the shared inputs:
 *
 *     renderChartSVG({ ...common, titleId: narrowTitleId, ...narrowChartVariant(points, decimals) })
 *
 * Lives here rather than in `geometry.ts` because `variant` is a rendering
 * concern (test ids and class names), while the box it wraps is a geometric
 * one — `narrowDimensions` stays where the rest of the geometry is. */
export function narrowChartVariant(points: ChartPoint[], decimals: number) {
  return {
    dims: narrowDimensions(points.map((p) => p.period), points, decimals),
    tickFontSize: NARROW_TICK_FONT_SIZE,
    xTickLabelOffset: NARROW_X_TICK_LABEL_OFFSET,
    maxXTicks: NARROW_MAX_X_TICKS,
    annotationLabelFontSize: NARROW_ANNOTATION_LABEL_FONT_SIZE,
    variant: "narrow" as const,
  };
}

function escapeXml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function renderChartSVG(input: RenderChartSVGInput): string {
  const dims =
    input.dims ?? wideDimensions(input.points.map((p) => p.period), input.points, input.decimals);
  const tickFontSize = input.tickFontSize ?? WIDE_TICK_FONT_SIZE;
  const xTickLabelOffset = input.xTickLabelOffset ?? 16;
  const maxXTicks = input.maxXTicks ?? 6;
  const annotationLabelFontSize = input.annotationLabelFontSize ?? WIDE_ANNOTATION_LABEL_FONT_SIZE;
  // The empty default is load-bearing: it is what makes every existing test
  // id, and the committed golden fixture, byte-for-byte unchanged.
  const idSuffix = input.variant ? `-${input.variant}` : "";
  const variantClass = input.variant ? ` indicator-chart-svg--${input.variant}` : "";
  const area = plotArea(dims);
  const periods = input.points.map((p) => p.period);
  const projected = projectPoints(input.points, dims);
  const segments = buildLineSegments(projected);
  const bands = buildBreakBands(input.breaks, periods, input.frequency, dims);
  const domain = valueDomain(input.points);
  const xTicks = buildXTicks(periods, dims, maxXTicks);
  const yTicks = buildYTicks(domain, dims, input.decimals);

  const bandRects = bands
    .map(
      (b) =>
        `<rect class="chart-break-band" data-testid="chart-break-band${idSuffix}" data-break-key="${escapeXml(b.key)}" ` +
        `x="${b.x.toFixed(2)}" y="${area.y0.toFixed(2)}" width="${b.width.toFixed(2)}" height="${area.height.toFixed(2)}" ` +
        `fill="var(--color-break-band)" fill-opacity="0.35" />`,
    )
    .join("");

  // The change-of-government marker — the chart's THIRD vertical/line
  // treatment, and the one that had to be designed AROUND two codes already
  // spoken for. What a reader is meant to learn from each:
  //
  //   dotted grey ALONG THE DATA PATH   "this observation is provisional; the
  //                                      source may still revise it"
  //                                      (RESERVED — theme.css's own header)
  //   wide translucent VERTICAL BAND    "the series changed methodology here;
  //                                      the two sides are not directly
  //                                      comparable"
  //   thin SOLID vertical rule + flag   "a different government took office
  //                                      here" (this)
  //
  // Three separations, none of them relying on colour alone:
  //
  //   AGAINST THE PROVISIONAL DASH — solid, never dashed, and drawn ACROSS the
  //   plot rather than along the data path. The dash is a statement about one
  //   observation's status; this is a statement about the calendar, and
  //   reusing the dash would teach one reader two contradictory meanings for
  //   one code. `test/design-system/reserved-semantics.test.ts` enforces it.
  //   The colour is `--color-ink`, not the reserved provisional grey — which
  //   also settles the near-collision that ruled out `--color-ink-muted`
  //   here: read theme.css's two declared values side by side and they are the
  //   same grey to a reader's eye, so a solid rule in the axis grey would have
  //   collided with the reserved provisional semantic in everything but name.
  //   (The hexes are deliberately NOT quoted in this comment: the reserved-
  //   semantics guard scans every source file for the reserved values, and
  //   Tailwind's own candidate scanner does not distinguish code from prose.)
  //
  //   AGAINST THE BREAK BAND — a `<line>` has no width at all, against a band
  //   one full period step wide; solid ink against a 0.35-opacity purple wash;
  //   an instant against an interval. The band shades a region because a
  //   rupture affects the data on both sides of it; the marker names a
  //   boundary because a change of government does not alter a single figure.
  //
  //   AGAINST THE DATA — the flag is a TRIANGLE, a third glyph shape beside
  //   the definitive circle and the provisional diamond. No shape is reused,
  //   so the whole drawing stays readable with colour discarded entirely
  //   (PRD §12.5, ADR-8: colour is never the sole channel).
  //
  // Drawn UNDER the axis and the series, above the break bands: annotation
  // never obscures the data it annotates.
  //
  // No party colour, and none is possible: `config/gobiernos.yaml` and
  // `EventConfig` carry no party field (PRD §12.1), so every marker on every
  // chart is the same ink. Colour carries no information here at all.
  const governmentMarks = buildGovernmentMarkers(
    input.governmentChanges ?? [],
    periods,
    input.frequency,
    dims,
  );
  const governmentMarkers = governmentMarks
    .map((marker) => {
      const x = marker.x;
      const flagHalfWidth = 5;
      const flagHeight = 7;
      return (
        `<g class="chart-government-marker" data-testid="chart-government-marker${idSuffix}" ` +
        `data-government-id="${escapeXml(marker.id)}">` +
        // A `<title>` on the group. The rule now carries its own name on the
        // drawing (see the label layer below), so this is no longer the only
        // way to identify it with a pointer — it stays because it adds the
        // YEAR, which the label deliberately omits: the x axis under the mark
        // is already a calendar and the chip below already prints it, so
        // spending the label's scarce horizontal room on it would buy nothing.
        //
        // It is NOT the accessibility answer, and never was: the root `<svg>`
        // is a single `role="img"`, which prunes its own descendants from the
        // accessibility tree, so a screen-reader reader reaches neither this
        // string nor one glyph of the labels. That reader is served by the
        // sentence `describeGovernmentChanges` renders beside the drawing.
        `<title>${escapeXml(es.chart.government.markerTitle(marker.name, marker.year))}</title>` +
        `<line class="chart-government-marker__rule" x1="${x.toFixed(2)}" y1="${area.y0.toFixed(2)}" ` +
        `x2="${x.toFixed(2)}" y2="${area.y1.toFixed(2)}" stroke="var(--color-ink)" stroke-width="1" />` +
        `<path class="chart-government-marker__flag" ` +
        `d="M${(x - flagHalfWidth).toFixed(2)},${area.y0.toFixed(2)} ` +
        `L${(x + flagHalfWidth).toFixed(2)},${area.y0.toFixed(2)} ` +
        `L${x.toFixed(2)},${(area.y0 + flagHeight).toFixed(2)} Z" fill="var(--color-ink)" />` +
        `</g>`
      );
    })
    .join("");

  // The editorial event span — the chart's FOURTH annotation treatment, and
  // the first one whose subject is a DURATION. What a reader is meant to learn
  // from each of the four, now:
  //
  //   dotted grey ALONG THE DATA PATH   "this observation is provisional; the
  //                                      source may still revise it"
  //                                      (RESERVED — theme.css's own header)
  //   wide translucent VERTICAL BAND    "the series changed methodology here;
  //                                      the two sides are not directly
  //                                      comparable"
  //   thin SOLID vertical rule + flag   "a different government took office
  //                                      here"
  //   solid HORIZONTAL rail + serifs    "the editorial registry dates this
  //                                      event from here to here" (this)
  //
  // Four separations, and not one of them is colour:
  //
  //   AGAINST THE PROVISIONAL DASH — solid, and drawn ACROSS the top of the
  //   plot rather than along the data path. `test/design-system/
  //   reserved-semantics.test.ts` fails the moment this stroke gains a dash.
  //
  //   AGAINST THE BREAK BAND — a STROKE against a FILL. The band shades a
  //   region because a rupture affects the data on both sides of it; the rail
  //   marks an extent without touching the values under it at all. (It is also
  //   the reason the rail is not simply a wider, differently-coloured band:
  //   that would leave hue as the only channel, which PRD §12.5 forbids, and
  //   would wash out a quarter of the plot on the real 2008-2013 case.)
  //
  //   AGAINST THE GOVERNMENT RULE — orientation, which is the property a
  //   reader reads before any other: horizontal against vertical, an interval
  //   against an instant. The serifs turn DOWN into the plot so the eye
  //   projects the interval onto the curve, which is the whole point of
  //   drawing it over the data rather than listing it beside it.
  //
  //   AGAINST THE DATA — no new glyph shape is introduced, so the definitive
  //   circle, the provisional diamond and the government triangle keep meaning
  //   exactly what they meant.
  //
  // An UNCAPPED end is load-bearing rather than a saved vertex: a serif says
  // "the event began/ended here", so putting one at the edge of a window the
  // event runs past would turn the plot's own boundary into a claim about the
  // calendar. `buildEventSpanRails` records which end was clamped; this reads
  // it.
  //
  // Drawn UNDER the axis and the series, above the break bands, exactly like
  // the government markers: annotation never obscures the data it annotates.
  const eventRails = buildEventSpanRails(
    input.eventSpans ?? [],
    periods,
    input.frequency,
    dims,
    tickFontSize,
  );
  const eventSpanRails = eventRails
    .map((rail) => {
      const y = rail.y;
      const foot = y + rail.serifLength;
      const start = `${rail.x1.toFixed(2)},${y.toFixed(2)}`;
      const end = `${rail.x2.toFixed(2)},${y.toFixed(2)}`;
      const d =
        (rail.clampedStart ? `M${start}` : `M${rail.x1.toFixed(2)},${foot.toFixed(2)} L${start}`) +
        ` L${end}` +
        (rail.clampedEnd ? "" : ` L${rail.x2.toFixed(2)},${foot.toFixed(2)}`);
      const from = formatPeriodProse(rail.startPeriod);
      const to = formatPeriodProse(rail.endPeriod);
      const title =
        rail.clampedStart || rail.clampedEnd
          ? es.chart.eventSpan.railTitleClamped(rail.name, from, to)
          : es.chart.eventSpan.railTitle(rail.name, from, to);
      return (
        `<g class="chart-event-span" data-testid="chart-event-span${idSuffix}" ` +
        `data-event-id="${escapeXml(rail.id)}" data-annotation-group="${escapeXml(rail.group)}">` +
        // Pointer-only, exactly like the government marker's: the root `<svg>`
        // is a single `role="img"`, which prunes its own descendants from the
        // accessibility tree. It still earns its place beside the label layer
        // below, on two counts: it carries the rail's own PERIOD RANGE (and,
        // when an end was clamped, says the event runs past the window), and it
        // is the only identification left on a drawing too narrow to print a
        // 58-glyph event name. The screen-reader reader is served by the
        // sentence `describeEventSpans` renders beside the drawing, in a polite
        // live region — this layer is interactive, so that sentence has to
        // re-narrate when the reader's selection changes.
        `<title>${escapeXml(title)}</title>` +
        `<path class="chart-event-span__rail" d="${d}" fill="none" stroke="var(--color-event-span)" ` +
        `stroke-width="${rail.strokeWidth.toFixed(2)}" stroke-linecap="butt" stroke-linejoin="miter" />` +
        `</g>`
      );
    })
    .join("");

  // The POLICY-MEASURE mark — the chart's FIFTH annotation treatment, and the
  // only one drawn OUTSIDE the plot area. What a reader is meant to learn from
  // each of the five, now:
  //
  //   dotted grey ALONG THE DATA PATH   "this observation is provisional; the
  //                                      source may still revise it"
  //                                      (RESERVED -- theme.css's own header)
  //   wide translucent VERTICAL BAND    "the series changed methodology here;
  //                                      the two sides are not directly
  //                                      comparable"
  //   thin SOLID vertical rule + flag   "a different government took office
  //                                      here"
  //   solid HORIZONTAL rail + serifs    "the editorial registry dates this
  //                                      event from here to here"
  //   short vertical STUB IN THE AXIS
  //   GUTTER, detached from the axis    "a measure entered into force on this
  //                                      date" (this)
  //
  // WHY THE GUTTER, AND NOT ANOTHER MARK ON THE PLOT. This is the one layer
  // whose placement is a product requirement rather than a legibility one. The
  // registry records which instrument entered into force on which date and
  // records nothing about what followed; the site must not say otherwise. A
  // full-height rule crossing the series at the exact period the curve turns
  // says otherwise without a single word — it asserts an effect by adjacency,
  // with no author and no source a reader could check, and no caption
  // underneath undoes it.
  //
  // Confined to the gutter, the mark makes exactly the statement the registry
  // supports: a date of entry into force is a fact about the CALENDAR, and the
  // calendar is the axis. It touches no value, spans no interval on the data,
  // and shades nothing.
  //
  // FIVE SEPARATIONS, AND NOT ONE OF THEM IS COLOUR:
  //
  //   AGAINST THE PROVISIONAL DASH -- solid, and drawn outside the plot
  //   entirely. `test/design-system/reserved-semantics.test.ts` fails the
  //   moment this stroke gains a dash.
  //
  //   AGAINST THE BREAK BAND -- a stroke against a fill, an instant against an
  //   interval, and the gutter against the plot.
  //
  //   AGAINST THE GOVERNMENT RULE -- the near miss, and the one this layer was
  //   designed around, because that mark is ALSO "a vertical line at an
  //   instant". Three things separate them at once: it is outside the plot
  //   where that one is inside it; it is a stub where that one spans the full
  //   plot height; and it is DETACHED from the axis where that one ends on it.
  //   The gap is load-bearing rather than decorative -- an investiture and a
  //   measure on the same period would otherwise draw one continuous line
  //   through the axis and collapse two codes into one.
  //
  //   AGAINST THE EVENT RAIL -- orientation and region: vertical below the
  //   axis against horizontal near the top of the plot, an instant against an
  //   interval. They share `--color-event-span` deliberately: both are the
  //   editorial registry speaking about a stretch of the calendar, and one
  //   voice in one ink is more legible than a fifth hue would be. Colour is
  //   the second channel here as everywhere else in this drawing.
  //
  //   AGAINST THE DATA -- it is not in the plot area at all, and it introduces
  //   no glyph shape, so the definitive circle, the provisional diamond and
  //   the government triangle keep meaning exactly what they meant.
  //
  // NO ON-DRAWING LABEL, deliberately. The government rules and the event
  // rails print their names inside the plot because the plot has vertical room
  // to spare; the gutter has none, and a name squeezed in beside the tick
  // labels would either be drawn over a numeral or pushed back onto the
  // series -- which is the one place this mark must never reach. The
  // identification is carried by the `<title>` (pointer), the sentence beside
  // the chart (every reader, and the only channel a screen reader has), the
  // chip below it and the legend entry: four places, all of them away from the
  // curve.
  const measureMarks = buildPolicyMeasureMarks(
    input.policyMeasures ?? [],
    periods,
    input.frequency,
    dims,
    tickFontSize,
    xTickLabelOffset,
  );
  const policyMeasureMarks = measureMarks
    .map((mark) => {
      const x = mark.x.toFixed(2);
      return (
        `<g class="chart-measure-mark" data-testid="chart-measure-mark${idSuffix}" ` +
        `data-measure-id="${escapeXml(mark.id)}">` +
        // Pointer-only, exactly like the government marker's and the rail's:
        // the root `<svg>` is a single `role="img"`, which prunes its own
        // descendants from the accessibility tree. It carries the FULL
        // calendar date rather than the year -- for a law, the day IS the
        // annotation, unlike a government term, which the chip already labels
        // by year.
        `<title>${escapeXml(es.chart.measure.markTitle(mark.name, formatCalendarDate(mark.dateStart)))}</title>` +
        `<line class="chart-measure-mark__stub" x1="${x}" y1="${mark.y1.toFixed(2)}" ` +
        `x2="${x}" y2="${mark.y2.toFixed(2)}" stroke="var(--color-event-span)" ` +
        `stroke-width="${mark.strokeWidth.toFixed(2)}" stroke-linecap="butt" />` +
        `</g>`
      );
    })
    .join("");

  // Line: each individual EDGE (the segment between two consecutive
  // plotted points) is coloured/dashed on its own, based on whether the
  // point it arrives AT is provisional — not the whole contiguous run. A
  // single provisional observation at the end of a long, otherwise
  // definitive history must not paint that entire history as provisional.
  // Colour is never the sole channel (indicator-page spec, §12.5) — the
  // dash pattern carries the same signal the marker shape below also
  // carries. Each contiguous run (split at null gaps) is still wrapped in
  // its own `<g data-testid="chart-line-segment-N">` so a gap-count
  // assertion stays meaningful at the run level.
  const linePaths = segments
    .map((segment, segIndex) => {
      const edges = segment
        .slice(0, -1)
        .map((a, i) => {
          const b = segment[i + 1];
          const isProvisional = b.status === "P";
          const stroke = isProvisional ? "var(--color-provisional)" : "var(--color-accent)";
          const dash = isProvisional ? ' stroke-dasharray="4 3"' : "";
          return (
            `<path class="chart-line" data-testid="chart-line-edge-${segIndex}-${i}${idSuffix}" ` +
            `d="M${a.x.toFixed(2)},${(a.y as number).toFixed(2)} L${b.x.toFixed(2)},${(b.y as number).toFixed(2)}" ` +
            `fill="none" stroke="${stroke}" stroke-width="2"${dash} />`
          );
        })
        .join("");
      return `<g class="chart-line-segment" data-testid="chart-line-segment-${segIndex}${idSuffix}">${edges}</g>`;
    })
    .join("");

  // Markers: circle = definitive, diamond = provisional — a shape channel
  // independent of colour (same requirement as the line dash above).
  const markers = projected
    .filter((p): p is typeof p & { y: number } => p.y !== null)
    .map((p) => {
      const isProvisional = p.status === "P";
      const fill = isProvisional ? "var(--color-provisional)" : "var(--color-accent)";
      if (isProvisional) {
        return (
          `<rect class="chart-marker chart-marker--provisional" data-testid="chart-marker-provisional${idSuffix}" ` +
          `x="${(p.x - 3).toFixed(2)}" y="${(p.y - 3).toFixed(2)}" width="6" height="6" ` +
          `transform="rotate(45 ${p.x.toFixed(2)} ${p.y.toFixed(2)})" fill="${fill}" />`
        );
      }
      return (
        `<circle class="chart-marker chart-marker--definitive" data-testid="chart-marker-definitive${idSuffix}" ` +
        `cx="${p.x.toFixed(2)}" cy="${p.y.toFixed(2)}" r="3" fill="${fill}" />`
      );
    })
    .join("");

  // The on-drawing LABELS — the layer that stops a rule and a rail from being
  // codes the reader has to decode somewhere else.
  //
  // WHY THEY ARE DRAWN LAST, when every mark above is drawn first. "Annotation
  // never obscures the data it annotates" is why the bands, the rails and the
  // rules sit under the series, and that rule is unchanged for all three. Text
  // is the one exception, and it is a narrow one: a 2-unit accent stroke
  // crossing a 10-unit glyph does not dim the letter, it erases the part of it
  // the reader needs. So the label is painted over the series, and the
  // `paint-order="stroke"` halo below — a background-coloured outline drawn
  // BEFORE the fill — confines what the label costs the data to the glyph
  // outlines rather than to the label's whole rectangle. The halo is
  // `--color-bg` because that is the surface these charts sit on (measured on
  // the built /indicador/{slug}: the figure and every ancestor up to the
  // section are transparent, so the page background shows through); on the
  // workbench's surface-backed showcase the two tokens differ by a shade the
  // eye does not separate, in both themes.
  //
  // WHERE each label goes is `placeAnnotationLabels`' single decision — the
  // orientation, the collision solving and the refusal to truncate all live
  // there, with the measurements that justify them.
  const annotationLabels = placeAnnotationLabels({
    governments: governmentMarks.map((marker) => ({ id: marker.id, name: marker.name, x: marker.x })),
    eventSpans: eventRails.map((rail) => ({
      id: rail.id,
      name: rail.name,
      x1: rail.x1,
      x2: rail.x2,
      y: rail.y,
      serifLength: rail.serifLength,
    })),
    area,
    fontSize: annotationLabelFontSize,
  });

  const renderAnnotationLabel = (
    label: PlacedAnnotationLabel,
    kind: "government" | "event-span",
    idAttribute: string,
    fill: string,
  ): string =>
    `<text class="chart-annotation-label chart-annotation-label--${kind}" ` +
    `data-testid="chart-${kind}-label${idSuffix}" ${idAttribute}="${escapeXml(label.id)}" ` +
    `x="${label.anchorX.toFixed(2)}" y="${label.anchorY.toFixed(2)}" ` +
    // `rotate(-90)` about the label's own anchor, so the name reads
    // bottom-to-top alongside the rule it belongs to. Bottom-to-top and not the
    // reverse because that is the direction Latin script is set vertically in
    // every timeline that does this, and the one a reader tilting their head
    // left can follow.
    (label.sideways ? `transform="rotate(-90 ${label.anchorX.toFixed(2)} ${label.anchorY.toFixed(2)})" ` : "") +
    `font-size="${annotationLabelFontSize}" text-anchor="start" fill="${fill}" ` +
    `stroke="var(--color-bg)" stroke-width="${(annotationLabelFontSize * 0.3).toFixed(2)}" ` +
    `stroke-linejoin="round" paint-order="stroke">${escapeXml(label.text)}</text>`;

  const annotationLabelMarkup =
    annotationLabels.eventSpans
      // The rail's own colour, so the words and the mark they name are the same
      // object: a reader who has learnt that rose means "an event covers these
      // periods" reads the label without being told which rail it belongs to.
      // Measured as TEXT (4.5:1, not the 3:1 a non-text graphic needs) against
      // both themes and both surfaces in `lib/design-system/contrast.ts`.
      .map((label) => renderAnnotationLabel(label, "event-span", "data-event-id", "var(--color-event-span)"))
      .join("") +
    annotationLabels.governments
      // `--color-ink` for the same reason the rule itself is drawn in it: the
      // change-of-government layer carries no colour information at all
      // (`config/gobiernos.yaml` has no party field, PRD §12.1), so its label
      // is simply the page's own text colour.
      .map((label) => renderAnnotationLabel(label, "government", "data-government-id", "var(--color-ink)"))
      .join("");

  const xTickMarks = xTicks
    .map(
      (t) =>
        `<text class="chart-tick chart-tick--x" x="${t.x.toFixed(2)}" y="${(dims.height - dims.marginBottom + xTickLabelOffset).toFixed(2)}" ` +
        `text-anchor="middle" font-size="${tickFontSize}" fill="var(--color-ink-muted)">${escapeXml(t.label)}</text>`,
    )
    .join("");

  const yTickMarks = yTicks
    .map(
      (t) =>
        `<text class="chart-tick chart-tick--y" x="${(dims.marginLeft - 8).toFixed(2)}" y="${t.y.toFixed(2)}" ` +
        `text-anchor="end" dominant-baseline="middle" font-size="${tickFontSize}" fill="var(--color-ink-muted)">${escapeXml(t.label)}</text>`,
    )
    .join("");

  const axisLine =
    `<line class="chart-axis" x1="${area.x0.toFixed(2)}" y1="${area.y1.toFixed(2)}" ` +
    `x2="${area.x1.toFixed(2)}" y2="${area.y1.toFixed(2)}" stroke="var(--color-ink-muted)" stroke-width="1" />`;

  return (
    `<svg class="indicator-chart-svg${variantClass}" data-testid="indicator-chart-svg${idSuffix}" viewBox="0 0 ${dims.width} ${dims.height}" ` +
    `preserveAspectRatio="xMidYMid meet" role="img" aria-labelledby="${input.titleId}" ` +
    `aria-describedby="${input.descriptionId} ${input.tableId}" xmlns="http://www.w3.org/2000/svg">` +
    `<title id="${input.titleId}">Evolución de la serie (${escapeXml(input.unit)})</title>` +
    bandRects +
    eventSpanRails +
    governmentMarkers +
    axisLine +
    // After the axis, because the stub hangs BELOW it and the two touch
    // nowhere -- and before the series, which shares the rule every other
    // annotation follows here: annotation never obscures the data. In this
    // layer's case the rule is satisfied by geometry rather than by ordering,
    // since nothing it draws is inside the plot area at all.
    policyMeasureMarks +
    linePaths +
    markers +
    annotationLabelMarkup +
    xTickMarks +
    yTickMarks +
    `</svg>`
  );
}
