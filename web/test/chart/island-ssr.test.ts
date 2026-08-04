// design.md D-5: "A Vitest golden test asserts the island's initial render
// equals the build-time SVG" — task 8.12. `svelte/server`'s `render()`
// server-renders `ChartIsland.svelte` to a plain string with NO DOM/browser
// required (the same mechanism Astro itself uses to produce an island's
// initial HTML before hydration), so this proves byte-identical parity by
// construction: both `IndicatorChart.astro` (slice 7) and `ChartIsland.svelte`
// call the exact same `renderChartSVG` function with the exact same
// arguments in the default (raw/full) view — there is no second renderer to
// drift from the first.
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { render } from "svelte/server";
import ChartIsland from "../../src/components/ChartIsland.svelte";
import { narrowChartVariant, renderChartSVG } from "../../src/lib/chart/svg";
import type { ChartPoint } from "../../src/lib/chart/geometry";
import { formatPeriodCompact } from "../../src/lib/format/period";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Identical to svg.test.ts's own GOLDEN_POINTS/GOLDEN_BREAKS — the whole
// point of this test is to feed the SAME input the committed golden fixture
// was generated from.
const GOLDEN_POINTS: ChartPoint[] = [
  { period: "2019-Q1", value: 10.2, status: "D" },
  { period: "2019-Q2", value: 10.5, status: "D" },
  { period: "2019-Q3", value: 10.1, status: "D" },
  { period: "2019-Q4", value: 9.8, status: "D" },
  { period: "2020-Q1", value: 14.4, status: "D" },
  { period: "2020-Q2", value: 15.3, status: "D" },
  { period: "2020-Q3", value: 16.3, status: "P" },
];
const GOLDEN_BREAKS = [{ key: "covid-2020", date: "2020-04-01", kind: "metodológica", noteMd: "n/a", sourceUrl: null }];
// ...and svg.test.ts's own GOLDEN_GOVERNMENTS, in the shape the island's
// `annotations` prop takes. The golden fixture now covers the
// change-of-government marker, so parity is only meaningful if this side
// feeds the island the same change.
const GOLDEN_ANNOTATIONS = [
  {
    id: "gobierno-ejemplo",
    group: "governments" as const,
    name: "Gobierno de ejemplo",
    dateStart: "2019-10-01",
    dateEnd: null,
    href: null,
  },
];

/** One of the island's two rendered drawings, by its own test id.
 *
 * This used to take the FIRST `<svg>` in the markup, which was
 * unambiguous while there was only one. The island now emits a narrow and a
 * wide variant (CSS shows one — see `geometry.ts` for why a phone needs its
 * own box), and the narrow one comes first in source order, so "the first
 * svg" silently became "the mobile chart". Selecting by test id says which
 * drawing is being asserted instead of depending on where it sits. */
function extractSvg(html: string, testId = "indicator-chart-svg"): string {
  const match = new RegExp(`<svg[^>]*data-testid="${testId}"[\\s\\S]*?</svg>`).exec(html);
  if (!match) throw new Error(`no <svg data-testid="${testId}"> found in rendered island HTML`);
  return match[0];
}

describe("ChartIsland — island-parity golden test (task 8.12)", () => {
  it("the island's initial SSR render's <svg> is byte-identical to the build-time SVG golden", () => {
    const { body } = render(ChartIsland, {
      props: {
        slug: "golden",
        name: "Golden fixture",
        unit: "% población activa",
        frequency: "Q" as const,
        decimals: 1,
        points: GOLDEN_POINTS,
        breaks: GOLDEN_BREAKS,
        annotations: GOLDEN_ANNOTATIONS,
        transforms: { yoy: false, qoq: false, perCapita: false },
        titleId: "golden-title",
        descriptionId: "golden-description",
        tableId: "golden-table",
      },
    });

    const golden = readFileSync(path.join(__dirname, "../fixtures/chart/golden-indicator-chart.svg"), "utf-8").trim();
    expect(extractSvg(body)).toBe(golden);
  });

  it("the island's NARROW drawing is byte-identical to the static component's, so the second box did not create a second renderer", () => {
    // D-5's anti-divergence device, extended to the variant it now has to
    // cover. The golden fixture pins the wide drawing; nothing pinned the
    // narrow one, and a second box is exactly the kind of thing that grows
    // a second implementation. Both sides are asserted against the SAME
    // `renderChartSVG` call rather than against a committed second fixture,
    // because the point is that there is only one renderer — not that its
    // output happens to match a file.
    const { body } = render(ChartIsland, {
      props: {
        slug: "golden",
        name: "Golden fixture",
        unit: "% población activa",
        frequency: "Q" as const,
        decimals: 1,
        points: GOLDEN_POINTS,
        breaks: GOLDEN_BREAKS,
        annotations: GOLDEN_ANNOTATIONS,
        transforms: { yoy: false, qoq: false, perCapita: false },
        titleId: "golden-title",
        descriptionId: "golden-description",
        tableId: "golden-table",
      },
    });

    const expected = renderChartSVG({
      points: GOLDEN_POINTS,
      breaks: GOLDEN_BREAKS.map((b) => ({ key: b.key, date: b.date })),
      governmentChanges: GOLDEN_ANNOTATIONS,
      frequency: "Q",
      decimals: 1,
      unit: "% población activa",
      titleId: "golden-title-narrow",
      descriptionId: "golden-description",
      tableId: "golden-table",
      ...narrowChartVariant(GOLDEN_POINTS, 1),
    });
    expect(extractSvg(body, "indicator-chart-svg-narrow")).toBe(expected);
  });

  it("the default (unhydrated) view is raw/full — no transform or range is pre-selected", () => {
    const { body } = render(ChartIsland, {
      props: {
        slug: "tasa-de-paro-epa",
        name: "Tasa de paro",
        unit: "% población activa",
        frequency: "Q" as const,
        points: GOLDEN_POINTS,
        transforms: { yoy: "optional", qoq: "optional", perCapita: false },
      },
    });
    // Every raw point's own period is present -- the default view was never
    // narrowed by a range preset or replaced by a derived transform series.
    for (const p of GOLDEN_POINTS) {
      // Present as the READER sees it. The point is still that no period was
      // dropped by a range preset or replaced by a derived series; the label
      // it is looked for under is now the compact register the island's data
      // table and axis both draw (`src/lib/format/period.ts`).
      expect(body).toContain(formatPeriodCompact(p.period));
    }
  });

  // series-transformations spec, "Range presets" — the "personalizado" entry
  // (verify-report WARNING-5). The custom range picker is the one control in
  // this component that CANNOT function without JavaScript: these pages are
  // statically built, so an arbitrary `[from, to]` pair has nothing to ask.
  // Astro server-renders an island's markup regardless of `client:idle`, so
  // rendering the picker unconditionally would hand a no-JavaScript reader
  // two date inputs and a commit button indistinguishable from the working
  // controls beside them. It is therefore gated on `onMount` — absent, not
  // present-but-dead, matching the spec's own discipline for a preset that
  // cannot apply.
  //
  // Asserted at BOTH levels on purpose: here, on the exact server-render
  // Astro ships, and in `tests/e2e/workbench/chart-no-js.spec.ts` against a
  // real `javaScriptEnabled: false` browser context. This one is the cheap,
  // always-run guard; that one is the honest end-to-end proof.
  it("server-renders no custom-range picker — it cannot work without JavaScript, so it must not appear to", () => {
    const { body } = render(ChartIsland, {
      props: {
        slug: "tasa-de-paro-epa",
        name: "Tasa de paro",
        unit: "% población activa",
        frequency: "Q" as const,
        points: GOLDEN_POINTS,
        transforms: { yoy: "optional", qoq: "optional", perCapita: false },
      },
    });
    expect(body).not.toContain('data-testid="chart-custom-range"');
    expect(body).not.toContain('data-testid="custom-range-from"');
    expect(body).not.toContain('data-testid="custom-range-apply"');
    expect(body).not.toContain('type="date"');
    // The rest of the island IS server-rendered — this is a targeted
    // omission, not the component failing to render at all.
    expect(body).toContain('data-testid="accessible-data-table"');
  });

  // The government range control, held to exactly the same standard as the
  // custom-range picker above and for exactly the same reason: it is a
  // hydration-only control on a statically built page, so shipping it in the
  // server-rendered markup would put a `<select>` in front of a
  // no-JavaScript reader that looks like every working control beside it and
  // changes nothing when used. Absent, not present-but-dead — the discipline
  // the spec states for a preset that cannot apply ("absent, not disabled").
  //
  // The props below are chosen so the control genuinely WOULD render once
  // hydrated — the 2010-2022 span overlaps all three government terms, and
  // none of them covers the whole of it — so this test cannot pass merely by
  // the control having nothing to offer. Written alongside rather than
  // red-first (the control cannot be absent from markup that does not exist
  // yet) and mutation-checked instead: removing the island's `hydrated`
  // guard fails it.
  it("server-renders no government range control — it cannot work without JavaScript, so it must not appear to", () => {
    const { body } = render(ChartIsland, {
      props: {
        slug: "tasa-de-paro-epa",
        name: "Tasa de paro",
        unit: "% población activa",
        frequency: "Q" as const,
        points: [
          { period: "2010-Q1", value: 20.1, status: "D" as const },
          { period: "2013-Q1", value: 26.9, status: "D" as const },
          { period: "2016-Q1", value: 21.0, status: "D" as const },
          { period: "2019-Q1", value: 14.7, status: "D" as const },
          { period: "2022-Q1", value: 13.6, status: "D" as const },
        ],
        annotations: [
          { id: "gobierno-zapatero-2004", group: "governments" as const, name: "José Luis Rodríguez Zapatero", dateStart: "2004-04-17", dateEnd: null, href: null },
          { id: "gobierno-rajoy-2011", group: "governments" as const, name: "Mariano Rajoy", dateStart: "2011-12-21", dateEnd: null, href: null },
          { id: "gobierno-sanchez-2018", group: "governments" as const, name: "Pedro Sánchez", dateStart: "2018-06-02", dateEnd: null, href: null },
        ],
        transforms: { yoy: "optional", qoq: "optional", perCapita: false },
      },
    });
    expect(body).not.toContain('data-testid="chart-government-range"');
    expect(body).not.toContain('data-testid="government-select"');
    expect(body).not.toContain("<select");
    // The governments themselves ARE still server-rendered as annotation
    // chips — this is a targeted omission of an inert CONTROL, not the island
    // withholding the editorial layer from a no-JavaScript reader.
    expect(body).toContain('data-testid="annotation-group-governments"');
    expect(body).toContain('data-testid="accessible-data-table"');
  });

  it("renders zero fetch/XHR-issuing markup and the whole component composes only from delivered props (series-transformations spec, no network request)", () => {
    const { body } = render(ChartIsland, {
      props: {
        slug: "tasa-de-paro-epa",
        name: "Tasa de paro",
        unit: "% población activa",
        frequency: "Q" as const,
        points: GOLDEN_POINTS,
        transforms: { yoy: "optional", qoq: "optional", perCapita: false },
      },
    });
    expect(body).not.toContain("fetch(");
    expect(body).not.toContain("XMLHttpRequest");
  });
});

// ---------------------------------------------------------------------------
// The island carries the same two variants the static component does.
//
// This matters more here than there: `IndicatorChart.astro` is the
// workbench's catalog entry, but a real `/indicador/{slug}` page composes
// `ChartIsland.svelte`, so THIS is the markup a reader on a phone actually
// receives — including a reader with JavaScript disabled, who gets this
// server-rendered string and nothing else.
describe("ChartIsland — responsive geometry", () => {
  function renderIsland() {
    const { body } = render(ChartIsland, {
      props: {
        slug: "golden",
        name: "Golden fixture",
        unit: "% población activa",
        frequency: "Q" as const,
        decimals: 1,
        points: GOLDEN_POINTS,
        breaks: GOLDEN_BREAKS,
        annotations: [],
        transforms: { yoy: false, qoq: false, perCapita: false },
      },
    });
    return body;
  }

  it("server-renders BOTH drawings, so the no-JavaScript phone reader gets the narrow one", () => {
    const body = renderIsland();
    expect(body).toContain('data-testid="indicator-chart-svg"');
    expect(body).toContain('data-testid="indicator-chart-svg-narrow"');
  });

  it("shows exactly one of them at any viewport, at the same breakpoint the static component uses", () => {
    const body = renderIsland();
    expect(body).toMatch(/class="[^"]*indicator-chart-island__figure--narrow[^"]*md:hidden[^"]*"/);
    expect(body).toMatch(/class="[^"]*indicator-chart-island__figure--wide[^"]*hidden md:block[^"]*"/);
  });

  it("gives the narrow drawing its own title id", () => {
    const ids = [...renderIsland().matchAll(/<title id="([^"]+)"/g)].map((m) => m[1]);
    expect(ids.length).toBe(2);
    expect(new Set(ids).size).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// The change-of-government layer, on the half a real `/indicador/{slug}` page
// actually composes — including for a reader with JavaScript disabled, who
// receives this server-rendered string and nothing else.
//
// The RANGE half of the behaviour cannot be proven here (`onMount` is a
// documented no-op on the server, so the SSR view is always raw/full); it is
// proven in a real browser by `tests/e2e/indicator/indicator-pages.spec.ts`.
describe("ChartIsland — changes of government", () => {
  const ANNOTATIONS = [
    { id: "gobierno-aznar-1996", group: "governments" as const, name: "José María Aznar", dateStart: "1996-05-05", dateEnd: null, href: null },
    { id: "gobierno-sanchez-2018", group: "governments" as const, name: "Pedro Sánchez", dateStart: "2018-06-02", dateEnd: null, href: null },
    { id: "crisis-2008", group: "exogenous" as const, name: "Crisis financiera", dateStart: "2008-01-01", dateEnd: "2013-12-31", href: null },
  ];
  const POINTS: ChartPoint[] = [
    { period: "2017-Q1", value: 18.8, status: "D" },
    { period: "2018-Q1", value: 16.7, status: "D" },
    { period: "2019-Q1", value: 14.7, status: "D" },
    { period: "2020-Q1", value: 14.4, status: "D" },
  ];

  function renderWithGovernments() {
    const { body } = render(ChartIsland, {
      props: {
        slug: "tasa-de-paro-epa",
        name: "Tasa de paro",
        unit: "% población activa",
        frequency: "Q" as const,
        decimals: 1,
        points: POINTS,
        annotations: ANNOTATIONS,
        transforms: { yoy: false, qoq: false, perCapita: false },
      },
    });
    return body;
  }

  it("marks the investiture inside the span and refuses the one that predates it", () => {
    // Aznar (1996) is a government this series lived under, and his chip is
    // rendered — but no change of government happened on this chart, so no
    // rule is drawn for him. Marking him would put a boundary at 2017-Q1.
    const body = renderWithGovernments();
    expect(body).toContain('data-government-id="gobierno-sanchez-2018"');
    expect(body).not.toContain('data-government-id="gobierno-aznar-1996"');
    expect((body.match(/data-testid="chart-government-marker"/g) ?? []).length).toBe(1);
    expect((body.match(/data-testid="chart-government-marker-narrow"/g) ?? []).length).toBe(1);
  });

  it("never marks a shock or a milestone as a change of government", () => {
    const body = renderWithGovernments();
    expect(body).not.toContain('data-government-id="crisis-2008"');
  });

  it("carries the legend entry and the naming sentence a no-JavaScript reader depends on", () => {
    const body = renderWithGovernments();
    expect(body).toContain('data-testid="chart-legend-government"');
    const description = /data-testid="chart-description"[^>]*>([\s\S]*?)<\/p>/.exec(body)?.[1] ?? "";
    expect(description).toContain("cambios de gobierno registrados");
    expect(description).toContain("Pedro Sánchez (2018)");
  });

  it("says nothing about governments for a series that overlaps no investiture", () => {
    const { body } = render(ChartIsland, {
      props: {
        slug: "tasa-de-paro-epa",
        name: "Tasa de paro",
        unit: "% población activa",
        frequency: "Q" as const,
        decimals: 1,
        points: POINTS,
        annotations: ANNOTATIONS.filter((a) => a.group !== "governments"),
        transforms: { yoy: false, qoq: false, perCapita: false },
      },
    });
    expect(body).not.toContain("chart-government-marker");
    expect(body).not.toContain('data-testid="chart-legend-government"');
    expect(body).not.toContain("cambios de gobierno registrados");
  });
});
