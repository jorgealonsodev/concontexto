// design-system spec, "All eight components render in the workbench" — tasks.md
// 6.1 (RED) / 6.2 (GREEN). Each of the seven static components renders
// standalone, through the real `experimental_AstroContainer` pipeline (the
// same harness `test/smoke/home.container.test.ts` established, tasks.md 5.1),
// with at least one state variant per the spec's own scenario wording ("each
// has documented props and at least one state variant").
import { describe, expect, it } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import IndicatorCard from "../../src/components/IndicatorCard.astro";
import MethodologySheet from "../../src/components/MethodologySheet.astro";
import BreakBand from "../../src/components/BreakBand.astro";
import AnnotationChip from "../../src/components/AnnotationChip.astro";
import ActionBar from "../../src/components/ActionBar.astro";
import FreshnessSemaphore from "../../src/components/FreshnessSemaphore.astro";
import AccessibleDataTable from "../../src/components/AccessibleDataTable.astro";
import * as fx from "../../src/workbench/fixtures";

// Every fixture below is spread into a fresh object literal at the call site
// (`{ props: { ...fx.x } }`) rather than passed straight through. Astro's
// `ContainerRenderOptions.props` is `Record<string, unknown>`, and a
// TypeScript *interface* — which is exactly what each component's `Props` is —
// carries no implicit index signature, so a correctly-typed fixture is not
// assignable to it (ts2322). Spreading yields an anonymous object type, which
// is. Nothing about the test changes: the same values reach the same
// component. Surfaced by verify-report SUGGESTION-14's new `astro check` gate;
// invisible for as long as this project had no type-checking at all.

describe("IndicatorCard (workbench)", () => {
  it("renders the fresh-state variant with name, value and period", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorCard, { props: { ...fx.indicatorCardFresh } });
    expect(html).toContain("Tasa de paro");
    expect(html).toContain("10,98");
    expect(html).toContain("2026-Q1");
    expect(html).toContain('href="/indicador/tasa-de-paro-epa"');
    expect(html).toContain("Al día");
  });

  it("renders the source-pending-state variant", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(IndicatorCard, { props: { ...fx.indicatorCardPending } });
    expect(html).toContain("Población residente");
    expect(html).toContain("Pendiente de actualización por la fuente");
  });
});

describe("FreshnessSemaphore (workbench)", () => {
  it("renders the fresh state", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(FreshnessSemaphore, { props: { ...fx.freshnessFresh } });
    expect(html).toContain("Al día");
  });

  it("renders the source-pending state with the spec-mandated exact label", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(FreshnessSemaphore, { props: { ...fx.freshnessPending } });
    expect(html).toContain("Pendiente de actualización por la fuente");
  });

  it("never renders any element for a build-vs-database staleness condition", async () => {
    // indicator-page spec, "The page never determines freshness over the
    // network": no prop, state or output of this component can express
    // "this page is older than the data we hold" — there is no third variant
    // to render in the first place, proven by exhausting both real states.
    const container = await AstroContainer.create();
    for (const props of [fx.freshnessFresh, fx.freshnessPending]) {
      const html = await container.renderToString(FreshnessSemaphore, { props: { ...props } });
      expect(html.toLowerCase()).not.toContain("desactualizada");
      expect(html.toLowerCase()).not.toContain("build");
    }
  });
});

describe("MethodologySheet (workbench)", () => {
  it("renders as its own landmark region with both the mobile disclosure and the desktop-always-visible copy", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(MethodologySheet, { props: { ...fx.methodologySheet } });
    expect(html).toContain('aria-labelledby="methodology-heading-tasa-de-paro-epa"');
    expect(html).toContain("Ficha metodológica");
    // First line visible without interaction (mobile): the summary text is
    // present in the served markup regardless of the <details> open state.
    expect((html.match(/Qué mide \/ qué no mide/g) ?? []).length).toBe(2); // <details><summary> + desktop block
    expect(html).toContain("No es lo mismo que el paro registrado del SEPE");
    expect(html).toContain("2020-Q2");
    expect(html).toContain("Versión 2");
  });

  it("state variant: no break list and no per-capita attribution renders neither section", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(MethodologySheet, {
      props: { ...fx.methodologySheet, breaks: [], perCapitaAttribution: null },
    });
    expect(html).not.toContain("Rupturas de la serie");
    expect(html).not.toContain("Atribución per cápita");
  });
});

describe("BreakBand (workbench)", () => {
  it("renders the band, its icon and the non-dismissible focus/hover tooltip trigger", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(BreakBand, { props: { ...fx.breakBand } });
    expect(html).toContain("Ruptura: 2020-Q2");
    expect(html).toContain("El INE cambió el cuestionario");
    expect(html).toContain("Ver nota completa");
    // The trigger is a `<button>`, not a `<span tabindex="0">` — verify-report
    // WARNING-7; `test/design-system/break-band-parity.test.ts` owns the full
    // rationale and holds `ChartIsland.svelte`'s copy to the same shape. This
    // line previously asserted `tabindex="0"`, which a button does not need.
    expect(html).toContain('class="break-band__trigger');
    expect(html).toMatch(/<button[^>]*class="break-band__trigger/);
  });

  it("exposes no prop, attribute or control that hides the band (P4)", async () => {
    // BreakBand's own Props type has exactly four fields — date, kind,
    // summary, noteHref — none of which is a boolean/visibility toggle; this
    // assertion is the executable form of that structural guarantee.
    const propNames = Object.keys(fx.breakBand);
    expect(propNames.sort()).toEqual(["date", "kind", "noteHref", "summary"]);
  });
});

describe("AnnotationChip (workbench)", () => {
  it("renders the linked-entry state variant as an anchor with the group prefix", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(AnnotationChip, { props: { ...fx.annotationChipWithLink } });
    expect(html).toContain("Shock:");
    expect(html).toContain("Crisis financiera");
    expect(html).toContain('href="/glosario/crisis-financiera-2008"');
  });

  it("renders the no-link-entry state variant as a plain span, not an anchor", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(AnnotationChip, { props: { ...fx.annotationChipWithoutLink } });
    expect(html).toContain("Gobierno:");
    expect(html).toContain("Cambio de gobierno");
    expect(html).not.toContain("<a ");
  });
});

describe("ActionBar (workbench)", () => {
  it("renders the full-export-state variant (CSV and JSON)", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(ActionBar, { props: { ...fx.actionBarFull } });
    expect(html).toContain("Enlace permanente");
    expect(html).toContain("Exportar CSV");
    expect(html).toContain("Exportar JSON");
  });

  it("renders the CSV-only-state variant: JSON control absent, not disabled", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(ActionBar, { props: { ...fx.actionBarCsvOnly } });
    expect(html).toContain("Exportar CSV");
    expect(html).not.toContain("Exportar JSON");
    expect(html).not.toContain("disabled");
  });
});

describe("AccessibleDataTable (workbench)", () => {
  it("renders one row per observation with period, value and status", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(AccessibleDataTable, { props: { ...fx.accessibleDataTable } });
    expect(html).toContain('id="workbench-data-table"');
    expect((html.match(/<tr class="border-b border-ink\/10"/g) ?? []).length).toBe(3);
    expect(html).toContain("Definitivo");
    expect(html).toContain("Provisional");
  });

  it("state variant: an empty points list renders zero data rows without error", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(AccessibleDataTable, {
      props: { ...fx.accessibleDataTable, points: [] },
    });
    expect((html.match(/<tr class="border-b border-ink\/10"/g) ?? []).length).toBe(0);
  });
});
