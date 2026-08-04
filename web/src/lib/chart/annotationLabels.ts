// Where each annotation's own NAME is drawn on the chart — the layer that
// turns a vertical rule and a horizontal rail from marks a reader has to
// decode into marks that say what they are.
//
// ── WHY THE NAMES WERE NOT ON THE DRAWING BEFORE, AND WHAT CHANGED ─────────
//
// They were left off for a measured reason, not an aesthetic one. Six
// horizontal labels do not fit: on /indicador/poblacion-residente the
// Calvo-Sotelo and González investitures are 21.25 units apart in the wide
// drawing and 9.20 apart in the narrow one, while "Leopoldo Calvo-Sotelo" is
// 108 units wide at the wide label size and "José Luis Rodríguez Zapatero" is
// 141. Any horizontal treatment draws the second name through the first before
// the third is reached.
//
// What changed is the ORIENTATION, not the arithmetic. A label rotated onto
// its side needs one LINE HEIGHT of horizontal room instead of one string
// length of it — 12.5 units at the wide size against 108 — and spends its
// length down the plot's own height, which is the axis with room to spare. The
// classic timeline solution, and it makes the density problem mostly vanish:
// at the wide size the 21.25-unit pair no longer collides at all.
//
// Mostly, not entirely. 9.20 units is still narrower than one line height at
// the narrow label size, so the second label of that pair starts below the
// first — the same lane discipline `selectEventSpans` already applies to two
// rails that overlap. Which labels need that is DECIDED HERE from the real
// geometry rather than assumed, because the answer depends on the series, the
// box and the reader's current range.
//
// ── WHAT A LABEL SAYS, AND WHY IT IS NOT THE YEAR ─────────────────────────
//
// The registry's own `name`, verbatim, and nothing else.
//
// Not the year: the x axis under the mark is already a calendar, and the chip
// below the chart already prints "1981: Leopoldo Calvo-Sotelo". A label that
// repeated the year would spend the drawing's scarcest resource — horizontal
// room next to a rule — on the one fact the reader can already read in two
// other places. The name is the fact that was ONLY available on hover.
//
// Not a surname either, tempting as "Zapatero" is at 8 glyphs against 28.
// `config/gobiernos.yaml` carries no surname field, so a short form could only
// be DERIVED, and every derivation rule breaks on real Spanish names: a
// last-token rule turns "José Luis Rodríguez Zapatero" into "Zapatero"
// correctly and would turn a "Fernández de la Vega" into "Vega". This project
// prints editorial data verbatim precisely so that no rendering layer can
// invent a fact; a rewritten name is exactly that.
//
// ── THE ONE THING THIS MODULE REFUSES TO DO ───────────────────────────────
//
// Truncate. "Crisis financiera global y crisis de deuda soberana europea" is
// 58 glyphs — about 520 units at the narrow label size, against a 371-unit
// plot area — and there is no honest way to print it there. An ellipsis would
// rename the event on screen; shrinking the type until it fits would trade an
// unreadable label for an illegible one. So a label that cannot be drawn WHOLE
// and INSIDE the plot area is not drawn at all, and the sentence under the
// chart — which names every projected event in full, in the same left-to-right
// order the rails appear in, and is the only channel a screen-reader reader
// has — is what keeps the omission disclosed rather than silent.
import { GLYPH_ADVANCE_RATIO, type PlotArea } from "./geometry";

/** Label type size for the wide drawing, in user units.
 *
 * The same 10 the axis ticks use, which is not a coincidence: at a 1280 px
 * viewport the chart column renders the 960-unit box at about 0.88, so both
 * land at roughly 8.8 CSS px. One type size in one drawing. */
export const WIDE_ANNOTATION_LABEL_FONT_SIZE = 10;

/** Label type size for the narrow drawing, in user units — deliberately SMALLER
 * than that box's own 20-unit ticks, and chosen by measurement rather than by
 * taste.
 *
 * Two constraints meet here. At a 375 px viewport the 560-unit box renders at
 * 0.557, so 14 units is 7.8 CSS px: small, and about what the wide drawing's
 * own axis ticks already render at on a desktop. Above that the labels stop
 * fitting: "Leopoldo Calvo-Sotelo" and "Felipe González" are 9.20 units apart
 * on /indicador/poblacion-residente, so one must sit under the other, and the
 * two together need 188 + 134 units of the plot's 344. At 16 units they need
 * 215 + 154 and the second name is refused; at 14 they fit with 14 units to
 * spare. The axis keeps the larger type because the axis is what the chart is
 * read against; the annotation label is what it is annotated with. */
export const NARROW_ANNOTATION_LABEL_FONT_SIZE = 14;

/** Line box height as a multiple of the type size — the horizontal room one
 * SIDEWAYS label occupies, and the vertical room one upright label occupies.
 *
 * 1.25 rather than the 1.19-1.24 Chromium reports for the shipped typeface
 * between 10 and 20 units: over-reserving costs a fraction of a unit of
 * clearance, under-reserving draws a descender through the next label. */
const LINE_HEIGHT_RATIO = 1.25;

/** Clear space between two labels, and between a rail and its own label, as a
 * multiple of the type size. Enough that two labels read as two, small enough
 * that a stacked pair still fits the plot. */
const LABEL_GAP_RATIO = 0.5;

/** Distance from a sideways label's own rule to the text baseline, as a
 * multiple of the type size. One em puts the ascenders just clear of the rule
 * and the descenders just inside the line box modelled below, so the whole
 * glyph box sits on the RIGHT of the mark it names. */
const BASELINE_OFFSET_RATIO = 1;

export interface GovernmentLabelInput {
  id: string;
  /** The registry's own name, printed verbatim. */
  name: string;
  /** The x of the rule this label names, in the box's own user units. */
  x: number;
}

export interface EventSpanLabelInput {
  id: string;
  name: string;
  /** The rail's own ends and geometry, exactly as `buildEventSpanRails`
   * produces them — this module never re-derives a rail's position. */
  x1: number;
  x2: number;
  y: number;
  serifLength: number;
}

export interface PlaceAnnotationLabelsInput {
  governments: readonly GovernmentLabelInput[];
  eventSpans: readonly EventSpanLabelInput[];
  area: PlotArea;
  fontSize: number;
}

export interface PlacedAnnotationLabel {
  id: string;
  /** The registry's own name, unchanged — never shortened, never elided. */
  text: string;
  /** The label's own rectangle in the box's user units. `svg.ts` draws from
   * `anchorX`/`anchorY`; this rectangle is what every collision and
   * containment claim is made about, and what a browser test measures
   * `getBBox()` against. */
  x: number;
  y: number;
  width: number;
  height: number;
  /** Where the `<text>` element's own x/y go. */
  anchorX: number;
  anchorY: number;
  /** True for the government labels, which are drawn `rotate(-90)` and read
   * bottom-to-top; false for the rail labels, which stay upright. */
  sideways: boolean;
}

export interface AnnotationLabels {
  governments: PlacedAnnotationLabel[];
  eventSpans: PlacedAnnotationLabel[];
}

/** Advance width of one glyph of a NAME, estimated from the same measured
 * ratio `geometry.ts` sizes its margins with.
 *
 * Deliberately the axis alphabet's 0.64 rather than a second, tighter constant
 * measured for letters (Chromium reports 0.50-0.55 per glyph for the real
 * presidential names at these sizes). One conservative estimate, used in one
 * place, erring the same way the margins already err: a label reserved a few
 * units too wide loses a little clear space, a label reserved a few units too
 * narrow is drawn through its neighbour. The browser gate in
 * `tests/e2e/indicator/indicator-annotation-labels.spec.ts` measures the real
 * `getBBox()` of every label on the built pages, so an estimate that ever
 * stops being conservative fails there rather than shipping. */
function estimateTextWidth(text: string, fontSize: number): number {
  return text.length * GLYPH_ADVANCE_RATIO * fontSize;
}

interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}

function intersects(a: Rect, b: Rect): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

/**
 * Slides one rectangle DOWN the plot until it clears everything already
 * placed, or reports that it cannot.
 *
 * Down and never sideways, because a label's x is what ties it to the mark it
 * names: a solver free to move a label horizontally would eventually put one
 * president's name over another's rule, which is worse than no name at all.
 * Vertical room is the axis this drawing has to spare, which is the whole
 * reason the labels were turned on their side in the first place.
 *
 * The loop re-runs after every move because clearing one obstacle can slide the
 * rectangle into another; it terminates because each pass moves strictly down
 * and the plot area's bottom edge is the exit.
 */
function settle(rect: Rect, obstacles: readonly Rect[], area: PlotArea, gap: number): Rect | null {
  let candidate = { ...rect };
  let moved = true;
  while (moved) {
    moved = false;
    for (const obstacle of obstacles) {
      const padded = {
        x: obstacle.x - gap,
        y: obstacle.y - gap,
        width: obstacle.width + gap * 2,
        height: obstacle.height + gap * 2,
      };
      if (!intersects(candidate, padded)) continue;
      candidate = { ...candidate, y: obstacle.y + obstacle.height + gap };
      moved = true;
    }
    if (candidate.y + candidate.height > area.y1) return null;
  }
  return candidate;
}

/**
 * Every annotation label this drawing may honestly carry, with the rectangle
 * each occupies.
 *
 * ORDER IS LOAD-BEARING. The rails are placed first, because a rail label's y
 * is dictated by the rail it names — it has to sit under its own serifs or it
 * would be read as belonging to the mark above. A government label's y is free,
 * so it is the one that yields when the two compete. On
 * /indicador/tasa-de-paro-epa that is not hypothetical: Rajoy's 2011 rule
 * stands inside the 2008-2013 crisis rail, and its name settles below the
 * crisis label rather than through it.
 *
 * The rails themselves are placed as obstacles too, not just their labels: a
 * government name starting at the plot's top edge would otherwise be drawn
 * straight through a rail's own stroke.
 */
export function placeAnnotationLabels(input: PlaceAnnotationLabelsInput): AnnotationLabels {
  const { area, fontSize } = input;
  const lineHeight = fontSize * LINE_HEIGHT_RATIO;
  const gap = fontSize * LABEL_GAP_RATIO;
  const obstacles: Rect[] = input.eventSpans.map((rail) => ({
    x: rail.x1,
    y: rail.y,
    width: Math.max(rail.x2 - rail.x1, 0),
    height: rail.serifLength,
  }));

  const eventSpans: PlacedAnnotationLabel[] = [];
  for (const rail of input.eventSpans) {
    const width = estimateTextWidth(rail.name, fontSize);
    // CENTRED on the rail, and clamped into the plot area.
    //
    // The first rule tried here anchored the label on the event's own first
    // period, which reads well, and fell back to its last period when that
    // overran the box. A real browser threw it out: on the narrow
    // /indicador/tasa-de-paro-epa drawing "Pandemia de COVID-19" ended up
    // entirely to the LEFT of the pandemic rail and partly under the 2008-2013
    // crisis rail above it — a name that appeared to belong to a different
    // event. Centring puts the words over the interval they describe in every
    // case a rail is wide enough to have a middle.
    const left = Math.min(
      Math.max((rail.x1 + rail.x2) / 2 - width / 2, area.x0),
      Math.max(area.x1 - width, area.x0),
    );
    if (left < area.x0 || left + width > area.x1) continue;
    // ...and the clamp must not have pushed the words clear of the mark. A
    // label parked beside its rail rather than over it names nothing, so it is
    // refused, and the sentence under the chart carries the event instead.
    if (left + width < rail.x1 || left > rail.x2) continue;
    const placed = settle(
      { x: left, y: rail.y + rail.serifLength + gap, width, height: lineHeight },
      obstacles,
      area,
      gap,
    );
    if (!placed) continue;
    obstacles.push(placed);
    eventSpans.push({
      id: rail.id,
      text: rail.name,
      ...placed,
      anchorX: placed.x,
      anchorY: placed.y + fontSize,
      sideways: false,
    });
  }

  const governments: PlacedAnnotationLabel[] = [];
  for (const marker of input.governments) {
    const height = estimateTextWidth(marker.name, fontSize);
    // The line box sits to the RIGHT of the rule, which is the side a reader
    // scanning left to right meets after the mark rather than before it. A rule
    // close enough to the plot's right edge that its label would overhang keeps
    // the label inside the box instead; the rule is still the label's own left
    // or right neighbour, and a clipped name is not a name.
    const x = Math.min(marker.x, area.x1 - lineHeight);
    if (x < area.x0) continue;
    const placed = settle({ x, y: area.y0, width: lineHeight, height }, obstacles, area, gap);
    if (!placed) continue;
    obstacles.push(placed);
    governments.push({
      id: marker.id,
      text: marker.name,
      ...placed,
      // `rotate(-90)` about the anchor maps the text's own baseline direction
      // onto -y, so the string runs UP from the anchor and the anchor is the
      // BOTTOM of the label. Ascenders map onto -x, which is why the anchor
      // sits one em right of the rectangle's left edge.
      anchorX: placed.x + fontSize * BASELINE_OFFSET_RATIO,
      anchorY: placed.y + placed.height,
      sideways: true,
    });
  }

  return { governments, eventSpans };
}
