// design.md D-5: "one shared geometry module feeds both the build-time SVG
// and the island" — `renderChartSVG` is the ONE function both
// `IndicatorChart.astro` (this slice) and `ChartIsland.svelte` (slice 8)
// call. This file's golden test is the anti-divergence device D-5 commits
// to: slice 8's own island-parity test (task 8.12) re-asserts the SAME
// golden fixture against the island's initial client-side render.
import { describe, expect, it } from "vitest";
import { narrowChartVariant, renderChartSVG } from "../../src/lib/chart/svg";
import {
  NARROW_ANNOTATION_LABEL_FONT_SIZE,
  WIDE_ANNOTATION_LABEL_FONT_SIZE,
} from "../../src/lib/chart/annotationLabels";
import {
  GLYPH_ADVANCE_RATIO,
  NARROW_MAX_X_TICKS,
  NARROW_TICK_FONT_SIZE,
  WIDE_TICK_FONT_SIZE,
  narrowDimensions,
  wideDimensions,
  type ChartPoint,
} from "../../src/lib/chart/geometry";

const GOLDEN_POINTS: ChartPoint[] = [
  { period: "2019-Q1", value: 10.2, status: "D" },
  { period: "2019-Q2", value: 10.5, status: "D" },
  { period: "2019-Q3", value: 10.1, status: "D" },
  { period: "2019-Q4", value: 9.8, status: "D" },
  { period: "2020-Q1", value: 14.4, status: "D" },
  { period: "2020-Q2", value: 15.3, status: "D" },
  { period: "2020-Q3", value: 16.3, status: "P" },
];

const GOLDEN_BREAKS = [{ key: "covid-2020", date: "2020-04-01" }];

/** One change of government INSIDE the golden span. Synthetic, like
 * `covid-2020` above: the golden series is not a real one either.
 *
 * NOT part of `GOLDEN_INPUT`, and that is the point rather than an oversight.
 * The `governments` annotation group is OFF by default, and the marker layer is
 * now gated on the reader's own selection, so the drawing the island
 * server-renders — the one the committed fixture pins, and the one a reader
 * with JavaScript disabled receives — carries no marker. A golden that showed
 * one would pin a state neither consumer can reach, and `island-ssr.test.ts`'s
 * parity assertion would fail against it. The marker markup is covered by the
 * explicit assertions below, which pass this list in the way a reader who
 * opened the group makes the renderer receive it. */
const GOLDEN_GOVERNMENTS = [
  { id: "gobierno-ejemplo", group: "governments", name: "Gobierno de ejemplo", dateStart: "2019-10-01" },
];

/** One editorial event SPAN inside the golden window, for exactly the reason
 * `GOLDEN_GOVERNMENTS` above exists: a layer absent from the committed fixture
 * is a layer the static component and the island can silently disagree about.
 *
 * `milestones` and not `exogenous`, deliberately — that is the one annotation
 * group both consumers open BY DEFAULT, so the golden pins the drawing an
 * unhydrated reader (and a reader with no JavaScript at all) really receives. */
const GOLDEN_EVENT_SPANS = [
  {
    id: "hito-ejemplo",
    group: "milestones",
    name: "Hito de ejemplo",
    dateStart: "2019-04-01",
    dateEnd: "2019-12-31",
  },
];

const GOLDEN_INPUT = {
  points: GOLDEN_POINTS,
  breaks: GOLDEN_BREAKS,
  eventSpans: GOLDEN_EVENT_SPANS,
  frequency: "Q" as const,
  decimals: 1,
  unit: "% población activa",
  titleId: "golden-title",
  descriptionId: "golden-description",
  tableId: "golden-table",
};

/** The same drawing as a reader who has opened the `governments` group
 * receives it. Everything about the marker layer is asserted through this
 * input, because that is the only state in which the layer exists at all. */
const GOVERNMENT_INPUT = { ...GOLDEN_INPUT, governmentChanges: GOLDEN_GOVERNMENTS };

/** The estimated advance width of one axis label, using the SAME measured
 * glyph ratio `geometry.ts` sizes the margins from — so this test and that
 * code can only disagree if one of them is wrong. Deliberately an estimate
 * and not a real measurement: there is no text engine here. The browser-side
 * proof, which measures `getBBox()` on the built page for all six slugs,
 * lives in `tests/e2e/indicator/indicator-pages.spec.ts`. */
function estimatedLabelWidth(label: string, tickFontSize: number): number {
  return label.length * GLYPH_ADVANCE_RATIO * tickFontSize;
}

/** Every axis label in one rendered drawing that falls outside its own
 * viewBox, by name.
 *
 * Anything drawn outside the viewBox is clipped by the SVG's own overflow, so
 * this is the whole failure mode in one function, applied identically to both
 * variants: a y label is right-anchored at its `x` and runs leftwards from
 * there; an x label is centred on its `x` and runs half its width each way. */
function ticksOutsideViewBox(svg: string, tickFontSize: number): string[] {
  const width = Number(/viewBox="0 0 (\d+) \d+"/.exec(svg)?.[1]);
  const outside: string[] = [];
  for (const [, x, label] of svg.matchAll(
    /class="chart-tick chart-tick--y"[^>]*x="([\d.]+)"[^>]*>([^<]+)</g,
  )) {
    if (Number(x) - estimatedLabelWidth(label, tickFontSize) < 0) outside.push(label);
  }
  for (const [, x, label] of svg.matchAll(
    /class="chart-tick chart-tick--x"[^>]*x="([\d.]+)"[^>]*>([^<]+)</g,
  )) {
    const half = estimatedLabelWidth(label, tickFontSize) / 2;
    if (Number(x) - half < 0 || Number(x) + half > width) outside.push(label);
  }
  return outside;
}

describe("renderChartSVG", () => {
  it("matches the committed golden fixture (build-time/island anti-divergence device)", async () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    await expect(svg).toMatchFileSnapshot("../fixtures/chart/golden-indicator-chart.svg");
  });

  it("renders exactly one break band per resolved break", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    const matches = svg.match(/data-testid="chart-break-band"/g) ?? [];
    expect(matches).toHaveLength(1);
    expect(svg).toContain('data-break-key="covid-2020"');
  });

  it("renders two break bands for two resolved breaks (indicator-page spec scenario)", () => {
    const svg = renderChartSVG({
      ...GOLDEN_INPUT,
      breaks: [
        { key: "b1", date: "2019-06-01" },
        { key: "b2", date: "2020-04-01" },
      ],
    });
    const matches = svg.match(/data-testid="chart-break-band"/g) ?? [];
    expect(matches).toHaveLength(2);
  });

  it("draws no break band at all for a series with no resolved breaks", () => {
    const svg = renderChartSVG({ ...GOLDEN_INPUT, breaks: [] });
    expect(svg).not.toContain("chart-break-band");
  });

  it("renders a provisional marker with a distinct shape (not just colour)", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain('data-testid="chart-marker-provisional"');
    expect(svg).toContain('data-testid="chart-marker-definitive"');
    // Different SVG element tags -- shape, not only fill colour, carries the
    // distinction (indicator-page spec, §12.5: "colour is never the sole
    // channel").
    expect(svg).toMatch(/<rect class="chart-marker chart-marker--provisional"/);
    expect(svg).toMatch(/<circle class="chart-marker chart-marker--definitive"/);
  });

  it("dashes the line segment ending at a provisional point", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain("stroke-dasharray");
  });

  it("consumes design tokens via var(--color-...) on SVG presentational attributes, never a hardcoded hex", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain("var(--color-accent)");
    expect(svg).toContain("var(--color-provisional)");
    expect(svg).toContain("var(--color-break-band)");
    expect(svg).not.toMatch(/#[0-9a-fA-F]{6}/);
  });

  it("is aria-labelled and aria-described, referencing the caller-supplied ids", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain('aria-labelledby="golden-title"');
    expect(svg).toContain('aria-describedby="golden-description golden-table"');
    expect(svg).toContain('role="img"');
  });

  // -------------------------------------------------------------------------
  // The government-change marker — the chart's THIRD vertical/line treatment,
  // and the one that had to be designed around two codes already spoken for:
  //
  //   dotted grey ALONG THE DATA PATH  = this observation is provisional
  //                                      (RESERVED, theme.css's own header)
  //   wide translucent VERTICAL BAND   = a methodological rupture here
  //   thin solid VERTICAL RULE + flag  = a change of government here (new)
  //
  // The assertions below are what keep the third from drifting into either of
  // the first two: solid (never dashed), one unit wide (never a band), and
  // capped by a triangle — a third glyph shape beside the definitive circle
  // and the provisional diamond, so the marker is distinguishable with colour
  // discarded entirely.
  it("marks a change of government with a SOLID vertical rule, never a dashed one", () => {
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    const marker = /<g class="chart-government-marker"[\s\S]*?<\/g>/.exec(svg)?.[0];
    expect(marker, "no government marker was rendered").toBeTruthy();
    // The reserved semantic, stated as an assertion: a dash here would teach
    // one reader two contradictory meanings for one visual code.
    expect(marker).not.toContain("stroke-dasharray");
    expect(marker).not.toContain("var(--color-provisional)");
    expect(marker).toContain('stroke-width="1"');
  });

  it("gives the marker a triangular cap — a third glyph shape, so colour is never the sole channel", () => {
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    // circle = definitive, diamond (rotated rect) = provisional, triangle
    // (closed 3-point path) = change of government. No shape is reused.
    expect(svg).toMatch(/<path class="chart-government-marker__flag" d="M[\d.]+,[\d.]+ L[\d.]+,[\d.]+ L[\d.]+,[\d.]+ Z"/);
  });

  it("is not the break band: one unit wide against the band's full period step", () => {
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    const band = /<rect class="chart-break-band"[^>]*>/.exec(svg)?.[0];
    const bandWidth = Number(/width="([\d.]+)"/.exec(band ?? "")?.[1]);
    expect(bandWidth).toBeGreaterThan(10);
    // The marker is a `<line>`, which has no width at all — the two cannot be
    // confused for one another even before colour is considered.
    expect(svg).toContain('<line class="chart-government-marker__rule"');
  });

  it("spans the full plot height, so the reader can project the boundary onto the curve", () => {
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    const rule = /<line class="chart-government-marker__rule"[^>]*>/.exec(svg)?.[0] ?? "";
    const y1 = Number(/y1="([\d.]+)"/.exec(rule)?.[1]);
    const y2 = Number(/y2="([\d.]+)"/.exec(rule)?.[1]);
    const band = /<rect class="chart-break-band"[^>]*>/.exec(svg)?.[0] ?? "";
    expect(y1).toBeCloseTo(Number(/ y="([\d.]+)"/.exec(band)?.[1]), 2);
    expect(y2 - y1).toBeCloseTo(Number(/height="([\d.]+)"/.exec(band)?.[1]), 2);
  });

  it("names the government in a <title>, so a pointer reader can identify the rule they see", () => {
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    expect(svg).toContain("<title>Cambio de gobierno: Gobierno de ejemplo (2019)</title>");
    expect(svg).toContain('data-government-id="gobierno-ejemplo"');
  });

  it("draws no marker at all for a series carrying no change of government", () => {
    const svg = renderChartSVG({ ...GOLDEN_INPUT, governmentChanges: [] });
    expect(svg).not.toContain("chart-government-marker");
  });

  it("never marks an investiture outside the plotted span (the marker would be a false claim)", () => {
    // Aznar over a 2019-2020 series: `nearestPeriodIndex` would snap 1996 onto
    // the first plotted quarter. `buildGovernmentMarkers` refuses; this pins
    // that the RENDERER inherits the refusal rather than re-deriving it.
    const svg = renderChartSVG({
      ...GOVERNMENT_INPUT,
      governmentChanges: [
        { id: "gobierno-aznar-1996", group: "governments", name: "José María Aznar", dateStart: "1996-05-05" },
      ],
    });
    expect(svg).not.toContain("chart-government-marker");
  });

  // -------------------------------------------------------------------------
  // The editorial event SPAN — the chart's FOURTH annotation treatment, and
  // the first one about an interval rather than an instant or an observation:
  //
  //   dotted grey ALONG THE DATA PATH  = this observation is provisional
  //                                      (RESERVED, theme.css's own header)
  //   filled translucent VERTICAL COLUMN = a methodological rupture here
  //   thin solid VERTICAL rule + flag  = a change of government here
  //   solid HORIZONTAL rail + serifs   = this event covers these periods (new)
  //
  // The assertions below are what keep the fourth from drifting into any of
  // the first three: never dashed, never a fill, and horizontal where the
  // other two vertical treatments are vertical.
  it("draws an event span as a HORIZONTAL rail, which is what separates it from the vertical rule", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    const rail = /<path class="chart-event-span__rail"[^>]*d="([^"]+)"/.exec(svg)?.[1];
    expect(rail, "no event-span rail was rendered").toBeTruthy();
    // The rail's own leg — the horizontal one between the two serifs — has a
    // constant y. Parsed rather than assumed: a vertical rail would pass a
    // "there is a path" assertion and fail this one.
    const ys = [...rail!.matchAll(/[ML]([\d.]+),([\d.]+)/g)].map((m) => Number(m[2]));
    const xs = [...rail!.matchAll(/[ML]([\d.]+),([\d.]+)/g)].map((m) => Number(m[1]));
    expect(new Set(ys).size).toBeGreaterThan(1); // the serifs drop below the rail
    expect(Math.max(...xs) - Math.min(...xs)).toBeGreaterThan(0);
    // The two INNER vertices are the rail itself and share one y.
    expect(ys[1]).toBeCloseTo(ys[2], 5);
  });

  it("is a stroke and never a fill, which is what separates it from the break band", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    const rail = /<path class="chart-event-span__rail"[^>]*>/.exec(svg)?.[0] ?? "";
    expect(rail).toContain('fill="none"');
    expect(rail).toContain("var(--color-event-span)");
    expect(rail).not.toContain("var(--color-break-band)");
  });

  it("is SOLID and never borrows the reserved provisional dash", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    const group = /<g class="chart-event-span"[\s\S]*?<\/g>/.exec(svg)?.[0] ?? "";
    expect(group).not.toContain("stroke-dasharray");
    expect(group).not.toContain("var(--color-provisional)");
  });

  it("names the event in a <title>, so a pointer reader can identify the rail they see", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain("<title>Hito de ejemplo: de T2 2019 a T4 2019</title>");
    expect(svg).toContain('data-event-id="hito-ejemplo"');
    expect(svg).toContain('data-annotation-group="milestones"');
  });

  it("draws no rail at all when the caller shows no annotation group", () => {
    // The projection follows the reader's own group selection: pass nothing
    // and the drawing is byte-identical to what it was before this layer.
    const svg = renderChartSVG({ ...GOLDEN_INPUT, eventSpans: [] });
    expect(svg).not.toContain("chart-event-span");
  });

  it("never projects an event that falls entirely outside the plotted span", () => {
    // The rule the government markers established, inherited here rather than
    // re-derived: `nearestPeriodIndex` would snap a 2005 event onto 2019-Q1.
    const svg = renderChartSVG({
      ...GOLDEN_INPUT,
      eventSpans: [
        { id: "viejo", group: "exogenous", name: "Crisis anterior", dateStart: "2005-01-01", dateEnd: "2007-12-31" },
      ],
    });
    expect(svg).not.toContain("chart-event-span");
  });

  it("never projects an event the registry leaves open-ended, because there is no period to draw", () => {
    const svg = renderChartSVG({
      ...GOLDEN_INPUT,
      eventSpans: [
        { id: "abierto", group: "exogenous", name: "Shock energético", dateStart: "2019-06-01", dateEnd: null },
      ],
    });
    expect(svg).not.toContain("chart-event-span");
  });

  it("leaves the CLAMPED end of a rail uncapped, so the plot's edge never reads as the event's end", () => {
    // A span running past the last plotted period. Capped, the serif would
    // assert the event ended in 2020-Q3; uncapped, the rail simply runs out of
    // chart. Counted rather than eyeballed: two serifs mean four vertices,
    // one serif means three.
    const both = renderChartSVG({
      ...GOLDEN_INPUT,
      eventSpans: [{ id: "dentro", group: "milestones", name: "Dentro", dateStart: "2019-04-01", dateEnd: "2019-12-31" }],
    });
    const clamped = renderChartSVG({
      ...GOLDEN_INPUT,
      eventSpans: [{ id: "fuera", group: "milestones", name: "Fuera", dateStart: "2019-04-01", dateEnd: "2024-12-31" }],
    });
    const vertices = (svg: string) =>
      (/<path class="chart-event-span__rail"[^>]*d="([^"]+)"/.exec(svg)?.[1] ?? "").match(/[ML]/g)?.length ?? 0;
    expect(vertices(both)).toBe(4);
    expect(vertices(clamped)).toBe(3);
  });

  it("draws the rails UNDER the data, so an annotation never obscures what it annotates", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    // Not vacuous: `indexOf` returns -1 for an absent layer, which would pass
    // every comparison below.
    expect(svg).toContain("chart-event-span");
    expect(svg.indexOf("chart-event-span")).toBeLessThan(svg.indexOf("chart-line-segment"));
    expect(svg.indexOf("chart-event-span")).toBeLessThan(svg.indexOf("chart-marker"));
  });

  it("never draws a line across a null-valued gap", () => {
    const svg = renderChartSVG({
      ...GOLDEN_INPUT,
      points: [
        { period: "2020-Q1", value: 1, status: "D" },
        { period: "2020-Q2", value: null, status: "D" },
        { period: "2020-Q3", value: 3, status: "D" },
      ],
      breaks: [],
    });
    const segments = svg.match(/data-testid="chart-line-segment-\d+"/g) ?? [];
    expect(segments).toHaveLength(2);
  });

  // -------------------------------------------------------------------------
  // The on-drawing LABELS — the layer that answers "what is this rule?" without
  // the reader hovering it or reading a paragraph below the chart.
  //
  // `lib/chart/annotationLabels.ts` owns the geometry and is tested there
  // against the real worst case. These assertions cover the part only this file
  // can see: that the text reaches the markup, that it is the registry's own
  // words, and that it is drawn where a reader can actually read it.
  it("prints the government's own name ON the drawing, not only in a <title> a pointer can reach", () => {
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    expect(svg).toContain('data-testid="chart-government-label"');
    expect(svg).toMatch(/<text class="chart-annotation-label[^"]*"[^>]*>Gobierno de ejemplo<\/text>/);
    expect(svg).toContain('data-government-id="gobierno-ejemplo"');
  });

  it("turns the government label onto its side, which is the whole reason six names fit at all", () => {
    // A horizontal name needs a string-length of room beside its rule; a
    // sideways one needs a line-height. On the real page that is 108 units
    // against 12.5, and it is the difference between labelling this layer and
    // not labelling it.
    const label = /<text class="chart-annotation-label chart-annotation-label--government"[^>]*>/.exec(
      renderChartSVG(GOVERNMENT_INPUT),
    )?.[0];
    expect(label, "no government label was rendered").toBeTruthy();
    expect(label).toMatch(/transform="rotate\(-90 [\d.]+ [\d.]+\)"/);
  });

  it("prints the event's own name beside its rail", () => {
    const svg = renderChartSVG(GOLDEN_INPUT);
    expect(svg).toContain('data-testid="chart-event-span-label"');
    expect(svg).toMatch(/<text class="chart-annotation-label[^"]*"[^>]*>Hito de ejemplo<\/text>/);
  });

  it("keeps the rail label upright, so the two labelled layers read as differently as the marks do", () => {
    const label = /<text class="chart-annotation-label chart-annotation-label--event-span"[^>]*>/.exec(
      renderChartSVG(GOLDEN_INPUT),
    )?.[0];
    expect(label, "no event-span label was rendered").toBeTruthy();
    expect(label).not.toContain("rotate(");
  });

  it("draws the labels OVER the data, because a name crossed out by the series is not a name", () => {
    // The marks themselves stay under the data — that rule is asserted two
    // tests up and is unchanged. Text is the exception, and it is a narrow one:
    // a 2-unit accent stroke through a 10-unit glyph leaves a shape a reader
    // has to guess at. The halo below is what keeps the cost to the data at the
    // glyph outlines rather than at the whole label's rectangle.
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    expect(svg.indexOf("chart-annotation-label")).toBeGreaterThan(svg.indexOf("chart-line-segment"));
    expect(svg.indexOf("chart-annotation-label")).toBeGreaterThan(svg.indexOf("chart-marker"));
  });

  it("gives every label a background-coloured halo, so it reads wherever it crosses the series", () => {
    const svg = renderChartSVG(GOVERNMENT_INPUT);
    for (const [, label] of svg.matchAll(/(<text class="chart-annotation-label[^>]*>)/g)) {
      expect(label).toContain('paint-order="stroke"');
      expect(label).toContain('stroke="var(--color-bg)"');
      // The halo is a SOLID outline and never a dash: the dash is the reserved
      // provisional semantic (`test/design-system/reserved-semantics.test.ts`).
      expect(label).not.toContain("stroke-dasharray");
    }
  });

  it("labels nothing when the reader has selected nothing, so the layer is byte-absent", () => {
    const svg = renderChartSVG({ ...GOLDEN_INPUT, eventSpans: [] });
    expect(svg).not.toContain("chart-annotation-label");
  });
});

// ---------------------------------------------------------------------------
// The narrow-viewport variant.
//
// One renderer, two boxes. `renderChartSVG` gains four optional inputs; three
// of them (type size, label offset, tick count) default to exactly what it
// emitted before, and `dims` defaults to the wide box DERIVED for the series
// in hand. The golden fixture above is what proves it: if any of those
// defaults drifted, that snapshot moves. The narrow variant is that same
// function called with the narrow geometry (see `geometry.ts` for why the
// phone needs its own box at all).
describe("renderChartSVG — narrow-viewport variant", () => {
  const NARROW_INPUT = { ...GOLDEN_INPUT, titleId: "golden-title-narrow", ...narrowChartVariant(GOLDEN_POINTS, 1) };
  const NARROW_GOVERNMENT_INPUT = { ...NARROW_INPUT, governmentChanges: GOLDEN_GOVERNMENTS };

  it("leaves the wide variant byte-identical when the new inputs are omitted", () => {
    // Belt and braces alongside the golden snapshot: passing the defaults
    // explicitly must produce the same string as passing nothing, so a
    // future change to a default value cannot hide behind a call site that
    // happens to pass it.
    expect(
      renderChartSVG({ ...GOLDEN_INPUT, tickFontSize: 10, xTickLabelOffset: 16, maxXTicks: 6 }),
    ).toBe(renderChartSVG(GOLDEN_INPUT));
  });

  it("draws its tick labels at the narrow type size", () => {
    expect(renderChartSVG(NARROW_INPUT)).toContain(`font-size="${NARROW_TICK_FONT_SIZE}"`);
    expect(renderChartSVG(NARROW_INPUT)).not.toContain('font-size="10"');
  });

  it("carries the narrow viewBox, so the two variants are genuinely different drawings", () => {
    const narrowDims = narrowDimensions(
      GOLDEN_POINTS.map((p) => p.period),
      GOLDEN_POINTS,
      1,
    );
    expect(renderChartSVG(NARROW_INPUT)).toContain(`viewBox="0 0 ${narrowDims.width} ${narrowDims.height}"`);
    expect(renderChartSVG(GOLDEN_INPUT)).toContain('viewBox="0 0 960 360"');
  });

  it("thins the x axis to the narrow tick count", () => {
    const narrowTicks = renderChartSVG(NARROW_INPUT).match(/class="chart-tick chart-tick--x"/g) ?? [];
    const wideTicks = renderChartSVG(GOLDEN_INPUT).match(/class="chart-tick chart-tick--x"/g) ?? [];
    expect(narrowTicks.length).toBeLessThan(wideTicks.length);
    // `buildXTicks` always appends the final period if the stride missed it,
    // so the count can be the cap plus one — never more.
    expect(narrowTicks.length).toBeLessThanOrEqual(NARROW_MAX_X_TICKS + 1);
  });

  it("suffixes EVERY test id, so the two variants never collide in one document", () => {
    // Both variants are rendered into the same page and CSS shows one. If
    // they shared test ids, every existing `[data-testid=...]` locator and
    // every `toHaveCount(1)` in this suite would start matching two nodes —
    // a strict-mode failure at best, a silently doubled count at worst. The
    // suffix is what keeps this change invisible to locators that predate it.
    const narrow = renderChartSVG(NARROW_INPUT);
    const wide = renderChartSVG(GOLDEN_INPUT);
    for (const id of wide.match(/data-testid="([^"]+)"/g) ?? []) {
      const value = id.slice('data-testid="'.length, -1);
      expect(narrow, `${value} is not suffixed in the narrow variant`).not.toContain(`data-testid="${value}"`);
      expect(narrow).toContain(`data-testid="${value}-narrow"`);
    }
  });

  it("keeps its own class, so stylesheets can target one variant without the other", () => {
    expect(renderChartSVG(NARROW_INPUT)).toContain('class="indicator-chart-svg indicator-chart-svg--narrow"');
    expect(renderChartSVG(GOLDEN_INPUT)).toContain('class="indicator-chart-svg"');
  });

  it("draws every tick label INSIDE the viewBox, which is the whole point of the narrow margins", () => {
    // The assertion that would have caught the pre-existing clipping: no
    // y label may start left of x=0, and no x label may end right of the
    // viewBox width. Both variants are now judged by the SAME helper,
    // because they are now sized by the same derivation.
    const outside = ticksOutsideViewBox(renderChartSVG(NARROW_INPUT), NARROW_TICK_FONT_SIZE);
    expect(outside, `clipped by the narrow viewBox: ${outside.join(", ")}`).toEqual([]);
  });

  it("still consumes design tokens and never a hardcoded hex, exactly like the wide variant", () => {
    const svg = renderChartSVG(NARROW_INPUT);
    expect(svg).toContain("var(--color-accent)");
    expect(svg).not.toMatch(/#[0-9a-fA-F]{6}/);
  });

  it("still bakes in the break bands, so P4 holds in the variant a phone actually sees", () => {
    // indicator-page spec, "Series breaks are always visible and never
    // dismissible". A second variant is a second chance to lose them.
    const svg = renderChartSVG(NARROW_INPUT);
    expect(svg.match(/data-testid="chart-break-band-narrow"/g) ?? []).toHaveLength(1);
    expect(svg).toContain('data-break-key="covid-2020"');
  });

  it("still projects the event span, so a phone reader gets the fourth treatment too", () => {
    // A second variant is a second chance to lose a layer, and the rail is the
    // one layer whose geometry SCALES with the box (its offset, thickness and
    // serif are multiples of the tick type size, which doubles in the narrow
    // drawing) — so it is also the one most likely to survive as a hairline
    // nobody can see rather than not at all.
    const narrow = renderChartSVG(NARROW_INPUT);
    expect(narrow.match(/data-testid="chart-event-span-narrow"/g) ?? []).toHaveLength(1);
    const narrowStroke = Number(
      /<path class="chart-event-span__rail"[^>]*stroke-width="([\d.]+)"/.exec(narrow)?.[1],
    );
    const wideStroke = Number(
      /<path class="chart-event-span__rail"[^>]*stroke-width="([\d.]+)"/.exec(renderChartSVG(GOLDEN_INPUT))?.[1],
    );
    expect(narrowStroke).toBeGreaterThan(wideStroke);
  });

  it("still marks the change of government, so a phone reader gets the third treatment too", () => {
    // A second variant is a second chance to lose a layer. The narrow box is
    // the drawing a real reader on a phone receives, so the marker has to
    // survive into it — solid, and still not the provisional dash.
    const svg = renderChartSVG(NARROW_GOVERNMENT_INPUT);
    expect(svg.match(/data-testid="chart-government-marker-narrow"/g) ?? []).toHaveLength(1);
    const marker = /<g class="chart-government-marker"[\s\S]*?<\/g>/.exec(svg)?.[0] ?? "";
    expect(marker).not.toContain("stroke-dasharray");
  });

  it("labels the marks in the phone drawing too, at its own smaller type size", () => {
    // A second variant is a second chance to lose a layer — and the label layer
    // is the one whose type size differs between the boxes, so it is also the
    // one that could survive as a size nobody chose. 14 against the wide box's
    // 10, because the narrow box's own investitures sit 9.20 units apart on
    // /indicador/poblacion-residente and two stacked names have to fit its 344
    // units of plot height (`annotationLabels.ts`).
    const svg = renderChartSVG(NARROW_GOVERNMENT_INPUT);
    expect(svg).toContain('data-testid="chart-government-label-narrow"');
    expect(svg).toContain('data-testid="chart-event-span-label-narrow"');
    const label = /<text class="chart-annotation-label chart-annotation-label--government"[^>]*>/.exec(svg)?.[0] ?? "";
    expect(label).toContain(`font-size="${NARROW_ANNOTATION_LABEL_FONT_SIZE}"`);
    expect(renderChartSVG(GOVERNMENT_INPUT)).toContain(
      `font-size="${WIDE_ANNOTATION_LABEL_FONT_SIZE}"`,
    );
  });
});

// ---------------------------------------------------------------------------
// The WIDE variant's margins.
//
// THE DEFECT, measured with `getBBox()` in Chromium on the built page
// /indicador/poblacion-residente at a 1280 px viewport, against the wide
// drawing's own `viewBox="0 0 960 360"`:
//
//   y label      left edge          x label   right edge
//   49.477.903     -5.46            2026-Q2      965.50
//   49.553.982     -6.49
//   49.630.061     -4.60
//   49.706.140     -3.61
//
// All four y labels started left of the viewBox origin and the final x tick
// ran 5.5 units past its right edge, so all five were clipped by the SVG's
// own overflow. The same sweep over all six built pages found the last x tick
// clipped on EVERY one of them (2026-Q2, 2026-06, 2026-Q1).
//
// THE CAUSE was that the wide margins were constants — 56 left, 16 right —
// that had never been derived from anything. A y label is right-anchored at
// `marginLeft - 8 = 48`, and a grouped eight-digit figure measures about 54
// units at this type size, so it needed 54 units to the left of 48 and had
// 48. Grouping separators widened the labels and made it visible; they did
// not cause it.
describe("renderChartSVG — the wide variant's derived margins", () => {
  // The real shape of `poblacion-residente`: ten-glyph y labels and
  // seven-glyph period labels, the series that breaks both edges at once.
  const POPULATION_POINTS: ChartPoint[] = Array.from({ length: 8 }, (_, i) => ({
    period: `${2024 + Math.floor(i / 4)}-Q${(i % 4) + 1}`,
    value: 49_477_903 + i * 76_079,
    status: "D" as const,
  }));

  it("keeps every axis label inside the viewBox for the series that clipped five of them", () => {
    const svg = renderChartSVG({ ...GOLDEN_INPUT, points: POPULATION_POINTS, decimals: 0, breaks: [] });
    const outside = ticksOutsideViewBox(svg, WIDE_TICK_FONT_SIZE);
    expect(outside, `clipped by the wide viewBox: ${outside.join(", ")}`).toEqual([]);
  });

  it("derives those margins even when the caller passes no dims at all", () => {
    // Two places compute this box: here, when a caller omits `dims`, and
    // `ChartIsland.svelte`, which needs the same margins to position its
    // interactive overlay over the drawing. One pure function serves both,
    // and this pins it — a default that quietly became a constant again
    // would leave the island's hit targets beside the points rather than on
    // them, and nothing else would notice.
    const periods = GOLDEN_POINTS.map((p) => p.period);
    expect(renderChartSVG(GOLDEN_INPUT)).toBe(
      renderChartSVG({ ...GOLDEN_INPUT, dims: wideDimensions(periods, GOLDEN_POINTS, 1) }),
    );
  });

  it("keeps the LAST x tick inside the viewBox, which every one of the six pages overran", () => {
    // Centred on the final data point, which sits exactly on the plot area's
    // right edge — so half a label always overhangs unless the right margin
    // is derived from the label. 16 units of margin against a 22.4-unit half
    // label is the arithmetic that put "2026-Q2" at 965.5.
    const svg = renderChartSVG(GOLDEN_INPUT);
    const outside = ticksOutsideViewBox(svg, WIDE_TICK_FONT_SIZE);
    expect(outside, `clipped by the wide viewBox: ${outside.join(", ")}`).toEqual([]);
  });
});
