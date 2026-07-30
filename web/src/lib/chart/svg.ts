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
  DEFAULT_DIMENSIONS,
  buildBreakBands,
  buildLineSegments,
  buildXTicks,
  buildYTicks,
  plotArea,
  projectPoints,
  valueDomain,
  type ChartDimensions,
  type ChartPoint,
} from "./geometry";
import type { Frequency } from "./periods";

export interface ChartBreakInput {
  key: string;
  date: string;
}

export interface RenderChartSVGInput {
  points: ChartPoint[];
  breaks: ChartBreakInput[];
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
  dims?: ChartDimensions;
}

function escapeXml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function renderChartSVG(input: RenderChartSVGInput): string {
  const dims = input.dims ?? DEFAULT_DIMENSIONS;
  const area = plotArea(dims);
  const periods = input.points.map((p) => p.period);
  const projected = projectPoints(input.points, dims);
  const segments = buildLineSegments(projected);
  const bands = buildBreakBands(input.breaks, periods, input.frequency, dims);
  const domain = valueDomain(input.points);
  const xTicks = buildXTicks(periods, dims);
  const yTicks = buildYTicks(domain, dims, input.decimals);

  const bandRects = bands
    .map(
      (b) =>
        `<rect class="chart-break-band" data-testid="chart-break-band" data-break-key="${escapeXml(b.key)}" ` +
        `x="${b.x.toFixed(2)}" y="${area.y0.toFixed(2)}" width="${b.width.toFixed(2)}" height="${area.height.toFixed(2)}" ` +
        `fill="var(--color-break-band)" fill-opacity="0.35" />`,
    )
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
            `<path class="chart-line" data-testid="chart-line-edge-${segIndex}-${i}" ` +
            `d="M${a.x.toFixed(2)},${(a.y as number).toFixed(2)} L${b.x.toFixed(2)},${(b.y as number).toFixed(2)}" ` +
            `fill="none" stroke="${stroke}" stroke-width="2"${dash} />`
          );
        })
        .join("");
      return `<g class="chart-line-segment" data-testid="chart-line-segment-${segIndex}">${edges}</g>`;
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
          `<rect class="chart-marker chart-marker--provisional" data-testid="chart-marker-provisional" ` +
          `x="${(p.x - 3).toFixed(2)}" y="${(p.y - 3).toFixed(2)}" width="6" height="6" ` +
          `transform="rotate(45 ${p.x.toFixed(2)} ${p.y.toFixed(2)})" fill="${fill}" />`
        );
      }
      return (
        `<circle class="chart-marker chart-marker--definitive" data-testid="chart-marker-definitive" ` +
        `cx="${p.x.toFixed(2)}" cy="${p.y.toFixed(2)}" r="3" fill="${fill}" />`
      );
    })
    .join("");

  const xTickMarks = xTicks
    .map(
      (t) =>
        `<text class="chart-tick chart-tick--x" x="${t.x.toFixed(2)}" y="${(dims.height - dims.marginBottom + 16).toFixed(2)}" ` +
        `text-anchor="middle" font-size="10" fill="var(--color-ink-muted)">${escapeXml(t.label)}</text>`,
    )
    .join("");

  const yTickMarks = yTicks
    .map(
      (t) =>
        `<text class="chart-tick chart-tick--y" x="${(dims.marginLeft - 8).toFixed(2)}" y="${t.y.toFixed(2)}" ` +
        `text-anchor="end" dominant-baseline="middle" font-size="10" fill="var(--color-ink-muted)">${escapeXml(t.label)}</text>`,
    )
    .join("");

  const axisLine =
    `<line class="chart-axis" x1="${area.x0.toFixed(2)}" y1="${area.y1.toFixed(2)}" ` +
    `x2="${area.x1.toFixed(2)}" y2="${area.y1.toFixed(2)}" stroke="var(--color-ink-muted)" stroke-width="1" />`;

  return (
    `<svg class="indicator-chart-svg" data-testid="indicator-chart-svg" viewBox="0 0 ${dims.width} ${dims.height}" ` +
    `preserveAspectRatio="xMidYMid meet" role="img" aria-labelledby="${input.titleId}" ` +
    `aria-describedby="${input.descriptionId} ${input.tableId}" xmlns="http://www.w3.org/2000/svg">` +
    `<title id="${input.titleId}">Evolución de la serie (${escapeXml(input.unit)})</title>` +
    bandRects +
    axisLine +
    linePaths +
    markers +
    xTickMarks +
    yTickMarks +
    `</svg>`
  );
}
