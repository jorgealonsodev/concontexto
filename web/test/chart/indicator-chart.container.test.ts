// indicator-page spec, "The default range is the full series" / "Series
// breaks are always visible and never dismissible" / "Three separately
// toggleable annotation groups, two off by default" — task 7.3 (RED).
import { describe, expect, it } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import IndicatorChart from "../../src/components/IndicatorChart.astro";
import * as fx from "../../src/workbench/fixtures";

// Fixtures are spread into a fresh object literal at the call site
// (`{ props: { ...fx.x } }`) because Astro's `ContainerRenderOptions.props` is
// `Record<string, unknown>` and an interface — which each component's `Props`
// is — has no implicit index signature, so a correctly-typed fixture is not
// assignable to it (ts2322). Same values, same component, same assertions; see
// the fuller note in `test/workbench/components.container.test.ts`.

describe("IndicatorChart", () => {
  it("default render spans the whole series (first to last observation)", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    // Read as the reader reads them: the data table's column and the axis
    // ticks are drawn in the compact period register, never in the database's
    // "2019-Q1" storage format (see `src/lib/format/period.ts`).
    expect(html).toContain("T1 2019"); // first observation
    expect(html).toContain("T1 2026"); // last observation
  });

  it("renders exactly one break band for one resolved break, at the correct period", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    const bandMatches = html.match(/data-testid="chart-break-band"/g) ?? [];
    expect(bandMatches).toHaveLength(1);
    expect(html).toContain('data-break-key="covid-2020"');
  });

  it("renders two break bands for two resolved breaks", async () => {
    const container = await AstroContainer.create();
    const twoBreaks = {
      ...fx.indicatorChart,
      breaks: [
        ...fx.indicatorChart.breaks!,
        {
          key: "second-break",
          date: "2021-01-01",
          kind: "de nivel",
          noteMd: "Salto de nivel por cambio de base.",
          sourceUrl: null,
        },
      ],
    };
    const html = await container.renderToString(IndicatorChart, { props: twoBreaks });
    const bandMatches = html.match(/data-testid="chart-break-band"/g) ?? [];
    expect(bandMatches).toHaveLength(2);
  });

  it("no affordance in the rendered markup hides a break (P4)", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    // The break list section carries no checkbox, no <details>, no
    // data-hidden attribute -- unlike the annotation groups below, which
    // deliberately DO have a toggle.
    const breaksSection = html.slice(html.indexOf('data-testid="chart-breaks"'), html.indexOf('data-testid="chart-annotations"'));
    expect(breaksSection).not.toContain("<input");
    expect(breaksSection).not.toContain("<details");
  });

  it("provisional points carry the dotted-grey encoding and the 'Provisional' disclosure", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="chart-marker-provisional"');
    expect(html).toContain("Provisional"); // via the composed AccessibleDataTable
  });

  it("groups (a) governments and (b) exogenous are off by default; group (c) milestones is on", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });

    const govInputMatch = /id="ann-toggle-tasa-de-paro-epa-governments"[^>]*/.exec(html);
    const exoInputMatch = /id="ann-toggle-tasa-de-paro-epa-exogenous"[^>]*/.exec(html);
    const milInputMatch = /id="ann-toggle-tasa-de-paro-epa-milestones"[^>]*/.exec(html);

    expect(govInputMatch?.[0]).not.toContain("checked");
    expect(exoInputMatch?.[0]).not.toContain("checked");
    expect(milInputMatch?.[0]).toContain("checked");
  });

  it("visible enable controls exist for every non-empty annotation group", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="annotation-toggle-governments"');
    expect(html).toContain('data-testid="annotation-toggle-exogenous"');
    expect(html).toContain('data-testid="annotation-toggle-milestones"');
  });

  it("a group with no applicable entries has no control at all (absent, not disabled)", async () => {
    const container = await AstroContainer.create();
    const noMilestones = { ...fx.indicatorChart, annotations: fx.indicatorChart.annotations!.filter((a) => a.group !== "milestones") };
    const html = await container.renderToString(IndicatorChart, { props: noMilestones });
    expect(html).not.toContain('data-testid="annotation-toggle-milestones"');
    expect(html).not.toContain("disabled");
  });

  it("generates a non-generic textual description present without JavaScript (build-time HTML)", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="chart-description"');
    expect(html).toContain("10,2"); // start value, Spanish decimal comma
  });

  it("composes the accessible data table with one row per observation", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="accessible-data-table"');
    const rows = html.match(/<tr class="border-b border-ink\/10"/g) ?? [];
    expect(rows).toHaveLength(fx.indicatorChart.points.length);
  });

  it("the SVG references both the description and the table via aria-describedby", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    const svgMatch = /aria-describedby="([^"]+)"/.exec(html);
    expect(svgMatch).not.toBeNull();
    const [descId, tableId] = svgMatch![1].split(" ");
    expect(html).toContain(`id="${descId}"`);
    expect(html).toContain(`id="${tableId}"`);
  });

  it("renders a legend distinguishing definitive/provisional by shape, not colour alone", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="chart-legend"');
    expect(html).toContain("Definitivo");
    expect(html).toContain("Provisional");
  });

  it("renders no chart-breaks section and no chart-annotations section when both are empty", async () => {
    const container = await AstroContainer.create();
    const minimal = { ...fx.indicatorChart, breaks: [], annotations: [] };
    const html = await container.renderToString(IndicatorChart, { props: minimal });
    expect(html).not.toContain('data-testid="chart-breaks"');
    expect(html).not.toContain('data-testid="chart-annotations"');
  });
});

// ---------------------------------------------------------------------------
// Responsive geometry: both variants are in the markup, CSS picks one.
//
// A static build cannot know the reader's viewport, so the chart uses the
// device this codebase already uses for exactly that problem —
// `MethodologySheet.astro` renders its fields twice and lets Tailwind's
// `md:hidden` / `hidden md:block` show one at the browser's own media-query
// evaluation. The same breakpoint is reused rather than a second one
// invented, so the site has ONE responsive boundary.
describe("IndicatorChart — responsive geometry", () => {
  it("renders BOTH the wide and the narrow drawing", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="indicator-chart-svg"');
    expect(html).toContain('data-testid="indicator-chart-svg-narrow"');
  });

  it("shows exactly one of them at any viewport, via the md breakpoint", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    // The narrow figure is hidden from `md` up; the wide one is hidden below
    // it. Neither is `display: none` in both states, and neither is visible
    // in both.
    expect(html).toMatch(/class="[^"]*indicator-chart__figure--narrow[^"]*md:hidden[^"]*"/);
    expect(html).toMatch(/class="[^"]*indicator-chart__figure--wide[^"]*hidden md:block[^"]*"/);
  });

  it("gives the narrow drawing its own title id, so no DOM id is duplicated", async () => {
    // Both variants carry `<title id=...>` referenced by `aria-labelledby`.
    // A shared id would be a duplicate in the document and an axe
    // `duplicate-id-aria` violation — on a page whose accessibility gate is
    // blocking.
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    const ids = [...html.matchAll(/<title id="([^"]+)"/g)].map((m) => m[1]);
    expect(ids.length).toBe(2);
    expect(new Set(ids).size).toBe(2);
  });

  it("describes both drawings with the SAME description and table, which exist once", async () => {
    // `aria-describedby` REFERENCES shared nodes; that is not duplication.
    // The textual description and the accessible data table are the chart's
    // textual equivalent and there is exactly one of each on the page.
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    // Scoped to the two `<svg>` roots: `BreakBand.astro` also uses
    // `aria-describedby` for its own tooltip, which is a different element
    // and a different relationship.
    const described = [...html.matchAll(/<svg[^>]*aria-describedby="([^"]+)"/g)].map((m) => m[1]);
    expect(described.length).toBe(2);
    expect(new Set(described).size).toBe(1);
    expect((html.match(/data-testid="chart-description"/g) ?? []).length).toBe(1);
    expect((html.match(/data-testid="accessible-data-table"/g) ?? []).length).toBe(1);
  });

  it("keeps the break bands in the narrow drawing too (P4 holds in the variant a phone sees)", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect((html.match(/data-testid="chart-break-band-narrow"/g) ?? []).length).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// The change-of-government markers, as the STATIC (zero-JavaScript) component
// renders them.
//
// THE RULE THIS BLOCK NOW STATES, and it inverted: nothing is drawn unless the
// reader has selected it. `governments` is one of the two annotation groups
// that start CLOSED, so the default drawing carries no rule, no label, no
// legend entry and no sentence — the same treatment the event-span rails have
// always had, applied to the layer that used to be the exception.
//
// This component's own honest limitation is unchanged and is what the second
// test below records: its toggle is a CSS checkbox, which reveals and hides
// CHIPS with zero JavaScript and cannot redraw an SVG serialised to a string at
// build time. So the drawing here shows the default-open groups and stays
// there. That is exactly what a reader with JavaScript disabled receives on a
// real page too, where the island server-renders the same unselected state.
describe("IndicatorChart — changes of government", () => {
  it("draws no marker and no label until the group is selected, in EITHER variant", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    // The fixture carries two `governments` entries and one of them falls
    // squarely inside the plotted span, so this is not vacuous: the layer is
    // withheld by the selection, not by the data.
    expect(html).not.toContain("chart-government-marker");
    expect(html).not.toContain("chart-government-label");
    expect(html).not.toContain('data-government-id="gob-ejemplo-2021"');
  });

  it("still offers the group's toggle and its chips, so the marks are one gesture away", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="annotation-toggle-governments"');
    expect(html).toContain('data-testid="annotation-group-content-governments"');
  });

  it("keeps the legend and the naming sentence away while nothing is marked", async () => {
    // A legend entry for a mark that is not drawn is a lie, and a sentence
    // naming rules the drawing does not carry is a worse one — it would sit in
    // the paragraph both drawings point at with `aria-describedby`, telling a
    // screen-reader reader about marks nobody can see.
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).not.toContain('data-testid="chart-legend-government"');
    expect(html).not.toContain("cambios de gobierno registrados");
  });

  it("keeps a polite live region ready for the sentence, mirroring the island's own markup", async () => {
    // This component is a faithful catalog of the island's markup; nothing on a
    // static page can populate the region, and it exists here for the same
    // reason the event-span one already does.
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="chart-government-note"');
  });

  it("has no marker, no legend entry and no sentence when no change falls inside the span", async () => {
    // Absent, not empty — the same discipline the annotation groups follow. A
    // series predating every recorded government (poblacion-residente's first
    // decade is the real case) simply says nothing about governments.
    const container = await AstroContainer.create();
    const noGovernments = {
      ...fx.indicatorChart,
      annotations: fx.indicatorChart.annotations!.filter((a) => a.group !== "governments"),
    };
    const html = await container.renderToString(IndicatorChart, { props: noGovernments });
    expect(html).not.toContain("chart-government-marker");
    expect(html).not.toContain('data-testid="chart-legend-government"');
    expect(html).not.toContain("cambios de gobierno registrados");
  });
});

// ---------------------------------------------------------------------------
// The editorial event SPAN — the chart's fourth annotation treatment, as the
// STATIC (zero-JavaScript) component renders it.
//
// This component is the workbench's catalog entry and the no-JS gate's
// subject, never a reader route, and it has one honest limitation the island
// does not: its annotation toggle is a CSS checkbox, which reveals and hides
// CHIPS with no script at all and cannot redraw an SVG serialised to a string
// at build time. So the drawing here shows the groups that are open BY
// DEFAULT — the exact map `ChartIsland.svelte` initialises its own toggle
// state from — and stays there. That is asserted below rather than left as a
// comment, because a future change that quietly started projecting every group
// would be projecting annotations the reader never asked to see.
describe("IndicatorChart — editorial event spans", () => {
  it("projects the default-open group's spans onto BOTH drawings", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    // `hito-2021` is the fixture's one milestone carrying both dates, and
    // milestones is the group open by default.
    expect((html.match(/data-testid="chart-event-span"/g) ?? []).length).toBe(1);
    expect((html.match(/data-testid="chart-event-span-narrow"/g) ?? []).length).toBe(1);
    expect(html).toContain('data-event-id="hito-2021"');
  });

  it("projects nothing for a group the reader has not opened", async () => {
    // `pandemia-ejemplo` sits squarely inside the fixture's span and would be
    // drawn the moment `exogenous` were shown — so this is a statement about
    // the selection, not about the data.
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).not.toContain('data-event-id="pandemia-ejemplo"');
    // ...and its chip is still there. The annotation layer is not withheld;
    // only its projection follows the toggle.
    expect(html).toContain("Pandemia (ejemplo)");
  });

  it("projects nothing for an event with no end date, or for one outside the plotted span", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).not.toContain('data-event-id="reforma-2012"'); // no end date
    expect(html).not.toContain('data-event-id="crisis-2008"'); // ends before the series begins
  });

  it("teaches the code with a legend entry carrying the rail's own glyph", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="chart-legend-event-span"');
    expect(html).toContain("Periodo de un acontecimiento");
  });

  it("names the projected events in a sentence, which is the only route a screen reader has to them", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    const note = /data-testid="chart-event-spans-note"[^>]*>([\s\S]*?)<\/p>/.exec(html)?.[1] ?? "";
    expect(note).toContain("Hito normativo (ejemplo)");
    expect(note).toContain("T1 2021");
    // The clause that accounts for the entries carrying no end date, so the
    // sentence never reads as a complete inventory of the group.
    expect(note).toContain("fecha de inicio y de fin");
  });

  it("has no rail, no legend entry and an empty sentence when nothing can be projected", async () => {
    const container = await AstroContainer.create();
    const noSpans = {
      ...fx.indicatorChart,
      annotations: fx.indicatorChart.annotations!.map((a) => ({ ...a, dateEnd: null })),
    };
    const html = await container.renderToString(IndicatorChart, { props: noSpans });
    // Matched on the rail's own class rather than on the bare `chart-event-span`
    // prefix: the sentence's node is `chart-event-spans-note`, which is always
    // in the markup (a live region has to exist before its first change) and
    // would make a prefix match report a rail that is not there.
    expect(html).not.toContain("chart-event-span__rail");
    expect(html).not.toContain('data-testid="chart-legend-event-span"');
    const note = /data-testid="chart-event-spans-note"[^>]*>([\s\S]*?)<\/p>/.exec(html)?.[1] ?? "x";
    expect(note.trim()).toBe("");
  });
});

// ---------------------------------------------------------------------------
// The POLICY-MEASURE group, rendered by the static (no-JavaScript) half.
//
// Two things are being held here at once, and they pull in opposite
// directions. The owner asked to SEE which measures were taken and when, so
// the layer has to be visible without a gesture. He also agreed the site must
// not say whether they worked, so everything the layer renders has to stop at
// an instrument and a date. Each test below pins one side of that.
describe("IndicatorChart — the policy-measure layer", () => {
  it("draws the measure's mark without the reader opening anything", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-measure-id="medida-ejemplo-2021"');
    // Both drawings: a phone reader gets the layer too.
    expect(html).toContain('data-testid="chart-measure-mark"');
    expect(html).toContain('data-testid="chart-measure-mark-narrow"');
  });

  it("offers the group its own toggle and chips, beside the other three", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="annotation-toggle-measures"');
    expect(html).toContain('data-testid="annotation-group-content-measures"');
    expect(html).toContain("medidas de política pública");
  });

  it("names the instrument and its date beside the chart, and claims nothing further", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="chart-measures-note"');
    expect(html).toContain("entrada en vigor");
    expect(html).toContain("15 de septiembre de 2021");
    // The refusal, rendered: the chart states that it represents no relation
    // between the measures and the series. Delete that clause from `es.ts` and
    // this fails.
    expect(html).toContain("no representa ninguna relación");
  });

  it("teaches the mark's code in the legend, only while a mark is drawn", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).toContain('data-testid="chart-legend-measure"');

    const withoutMeasures = {
      ...fx.indicatorChart,
      annotations: fx.indicatorChart.annotations!.filter((a) => a.group !== "measures"),
    };
    const bare = await container.renderToString(IndicatorChart, { props: withoutMeasures });
    expect(bare).not.toContain('data-testid="chart-legend-measure"');
    expect(bare).not.toContain('data-testid="chart-measure-mark"');
  });

  it("renders no on-drawing label for a measure, so no name sits beside the curve", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorChart, { props: { ...fx.indicatorChart } });
    expect(html).not.toContain("chart-measure-label");
  });
});
