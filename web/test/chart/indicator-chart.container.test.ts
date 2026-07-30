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
    expect(html).toContain("2019-Q1"); // first observation
    expect(html).toContain("2026-Q1"); // last observation
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
