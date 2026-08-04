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
import { buildGovernmentMarkers, type GovernmentChangeAnnotation } from "./governmentMarkers";
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
  /** The editorial registry's `governments` entries, unfiltered — which of
   * them may honestly be marked is `buildGovernmentMarkers`' decision, made
   * once, inside the renderer both consumers share. Omitted (or empty), the
   * drawing carries no marker and is byte-identical to what it was before
   * this layer existed. */
  governmentChanges?: GovernmentChangeAnnotation[];
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
  const governmentMarkers = buildGovernmentMarkers(
    input.governmentChanges ?? [],
    periods,
    input.frequency,
    dims,
  )
    .map((marker) => {
      const x = marker.x;
      const flagHalfWidth = 5;
      const flagHeight = 7;
      return (
        `<g class="chart-government-marker" data-testid="chart-government-marker${idSuffix}" ` +
        `data-government-id="${escapeXml(marker.id)}">` +
        // A `<title>` on the group, so a pointer reader can identify the rule
        // in front of them without six presidential names being printed
        // across a 560-unit-wide drawing. It is NOT the accessibility answer:
        // the root `<svg>` is a single `role="img"`, which prunes its own
        // descendants from the accessibility tree, so a screen-reader reader
        // never reaches this string. That reader is served by the sentence
        // `describeGovernmentChanges` adds to the chart's own description
        // paragraph — the node this drawing already names in
        // `aria-describedby`.
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
    governmentMarkers +
    axisLine +
    linePaths +
    markers +
    xTickMarks +
    yTickMarks +
    `</svg>`
  );
}
