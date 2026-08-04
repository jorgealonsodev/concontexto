// Representative props for the slice-6 static component workbench (design-system
// spec, "Eight components in a documented workbench" — "each has documented
// props and at least one state variant"). Single source shared by the workbench
// page itself and its Vitest/Playwright tests, so a test never guesses at what
// the workbench actually renders. Values are illustrative, drawn from the real
// shape of the slice-3 golden export fixture (`web/test/fixtures/export/`), not
// invented field names.
import { es } from "../i18n/es";
import type { Props as IndicatorCardProps } from "../components/IndicatorCard.astro";
import type { Props as MethodologySheetProps } from "../components/MethodologySheet.astro";
import type { Props as BreakBandProps } from "../components/BreakBand.astro";
import type { Props as AnnotationChipProps } from "../components/AnnotationChip.astro";
import type { Props as ActionBarProps } from "../components/ActionBar.astro";
import type { Props as FreshnessSemaphoreProps } from "../components/FreshnessSemaphore.astro";
import type { Props as AccessibleDataTableProps } from "../components/AccessibleDataTable.astro";
import type { Props as IndicatorChartProps } from "../components/IndicatorChart.astro";
import type { ChartPoint } from "../lib/chart/geometry";
import type { IndicatorTransformConfig } from "../lib/chart/applicability";

export const indicatorCardFresh: IndicatorCardProps = {
  slug: "tasa-de-paro-epa",
  name: "Tasa de paro",
  latestValue: 10.98,
  latestPeriod: "2026-Q1",
  unit: "% población activa",
  freshness: "fresh",
  decimals: 2,
};

export const indicatorCardPending: IndicatorCardProps = {
  slug: "poblacion-residente",
  name: "Población residente",
  latestValue: 48592909,
  latestPeriod: "2026-Q1",
  unit: "personas",
  freshness: "source-pending",
  decimals: 0,
};

export const freshnessFresh: FreshnessSemaphoreProps = { state: "fresh" };
export const freshnessPending: FreshnessSemaphoreProps = { state: "source-pending" };

export const methodologySheet: MethodologySheetProps = {
  slug: "tasa-de-paro-epa",
  measures:
    "El porcentaje de población activa que se encuentra en desempleo, según la Encuesta de Población Activa (EPA).",
  doesNotMeasure:
    "No es lo mismo que el paro registrado del SEPE: la EPA es una encuesta muestral trimestral; el paro registrado cuenta inscripciones administrativas.",
  sourceName: "Instituto Nacional de Estadística (INE)",
  sourceUrl: "https://www.ine.es",
  operation: "ine-epa",
  originLabel: "EPA453100",
  originHref: "https://servicios.ine.es/wstempus/js/ES/DATOS_SERIE/EPA453100",
  periodicity: "Trimestral",
  // The workbench showed a bare `2026-10-29` here, which was a second
  // instance of the same defect the real pages carried: a machine date
  // standing in for reader-facing copy. The fixture now mirrors what a real
  // page passes — the honest generic disclosure plus the source's own
  // calendar URL, rendered by the sheet as one link.
  nextPublicationLabel: es.page.nextPublicationFallback,
  nextPublicationHref: "https://www.ine.es/dyngs/INEbase/es/calendario.htm",
  extractedAt: "2026-07-29T06:10:00Z",
  unit: "% población activa",
  base: null,
  breaks: [{ date: "2020-Q2", note: "Cambio metodológico por la pandemia de COVID-19." }],
  vintageLabel: "Versión 2",
  revisionHistoryHref: "/indicador/tasa-de-paro-epa/revisiones",
  ingestionScriptHref: "https://github.com/concontexto/concontexto/blob/main/app/internal/adapters/ine",
  perCapitaAttribution: null,
};

export const breakBand: BreakBandProps = {
  date: "2020-Q2",
  kind: "metodológica",
  summary: "El INE cambió el cuestionario de la EPA por el impacto de la pandemia de COVID-19.",
  noteHref: "/indicador/tasa-de-paro-epa#ruptura-2020-q2",
};

export const annotationChipWithLink: AnnotationChipProps = {
  group: "exogenous",
  name: "Crisis financiera",
  dateLabel: "2008–2013",
  href: "/glosario/crisis-financiera-2008",
};

export const annotationChipWithoutLink: AnnotationChipProps = {
  group: "governments",
  name: "Cambio de gobierno",
  dateLabel: "2018",
  href: null,
};

export const actionBarFull: ActionBarProps = {
  permalink: "https://concontexto.example/indicador/tasa-de-paro-epa",
  csvHref: "/data-derived/tasa-de-paro-epa.csv",
  jsonHref: "/data-derived/series/tasa-de-paro-epa.json",
};

export const actionBarCsvOnly: ActionBarProps = {
  permalink: "https://concontexto.example/indicador/poblacion-residente",
  csvHref: "/data-derived/poblacion-residente.csv",
  jsonHref: null,
};

export const accessibleDataTable: AccessibleDataTableProps = {
  id: "workbench-data-table",
  caption: "Datos de Tasa de paro",
  unit: "% población activa",
  decimals: 2,
  points: [
    { period: "2025-Q3", value: 11.36, status: "D" },
    { period: "2025-Q4", value: 11.21, status: "D" },
    { period: "2026-Q1", value: 10.98, status: "P" },
  ],
};

export const indicatorChart: IndicatorChartProps = {
  slug: "tasa-de-paro-epa",
  name: "Tasa de paro",
  unit: "% población activa",
  frequency: "Q",
  decimals: 1,
  points: [
    { period: "2019-Q1", value: 10.2, status: "D" },
    { period: "2019-Q2", value: 10.5, status: "D" },
    { period: "2019-Q3", value: 10.1, status: "D" },
    { period: "2019-Q4", value: 9.8, status: "D" },
    { period: "2020-Q1", value: 14.4, status: "D" },
    { period: "2020-Q2", value: 15.3, status: "D" },
    { period: "2020-Q3", value: 15.9, status: "D" },
    { period: "2020-Q4", value: 15.5, status: "D" },
    { period: "2021-Q1", value: 15.98, status: "D" },
    { period: "2021-Q2", value: 15.26, status: "D" },
    { period: "2025-Q4", value: 11.21, status: "D" },
    { period: "2026-Q1", value: 10.98, status: "P" },
  ],
  breaks: [
    {
      key: "covid-2020",
      date: "2020-04-01",
      kind: "metodológica",
      noteMd: "El INE cambió el cuestionario de la EPA por el impacto de la pandemia de COVID-19.",
      sourceUrl: null,
    },
  ],
  annotations: [
    {
      // OUTSIDE this fixture's span (2019-Q1 onwards), and kept that way on
      // purpose: it demonstrates the half of the marker rule that is easiest
      // to get wrong. The chip below the chart still names this government —
      // it genuinely governed the series — but no vertical rule is drawn for
      // it, because the change of government happened before the first
      // observation and a rule at the left edge would claim it happened
      // there.
      id: "gob-2018",
      group: "governments",
      name: "Cambio de gobierno",
      dateStart: "2018-06-01",
      dateEnd: null,
      href: null,
    },
    {
      // INSIDE the span, so the workbench actually shows the marker (the
      // design-system spec asks each catalog component for at least one state
      // variant, and an undrawn layer is not one). Illustrative like every
      // other value in this file — a workbench fixture is not the editorial
      // registry, so this carries a plainly generic name rather than a real
      // president's beside a date that was never his.
      id: "gob-ejemplo-2021",
      group: "governments",
      name: "Cambio de gobierno (ejemplo)",
      dateStart: "2021-01-15",
      dateEnd: null,
      href: null,
    },
    {
      id: "crisis-2008",
      group: "exogenous",
      name: "Crisis financiera",
      dateStart: "2008-01-01",
      dateEnd: "2013-12-31",
      href: "/glosario/crisis-financiera-2008",
    },
    {
      id: "reforma-2012",
      group: "milestones",
      name: "Reforma laboral",
      dateStart: "2012-02-01",
      dateEnd: null,
      href: null,
    },
  ],
};

// Slice 8's own workbench entry (ChartIsland.svelte): reuses the exact same
// points/breaks/annotations as `indicatorChart` above (D-5: both components
// call the same shared geometry module), plus a `transforms` config with
// per-capita ENABLED (unlike tasa-de-paro-epa's real config, which has none
// — series-transformations spec's own applicability table) so the workbench
// exercises every one of the three toggle kinds, and a partial population
// series so the per-capita coverage-disclosure copy (task 8.4, D6) has
// something real to demonstrate: population is supplied only for
// 2019-Q1..2020-Q4, a genuine sub-span of the indicator's own 2019-Q1..
// 2026-Q1 range.
export const chartIslandTransforms: IndicatorTransformConfig = { yoy: "optional", qoq: "optional", perCapita: true };

// A DIFFERENT break than the static `indicatorChart.breaks` demo above,
// deliberately positioned so it stays inside BOTH available range presets
// this fixture's own span offers ("full" and "5 años" — see `sliceRange`'s
// own `isPresetAvailable`, "since 2008"/"since 2018" are absent for a series
// whose first observation is 2019). This is what
// `chart-island.spec.ts`'s "a break survives a range change" test proves —
// P4 must hold across the island's OWN interactive range switch, not only
// across the static component's fixed full-series render.
export const chartIslandBreaks = [
  {
    key: "reform-2022",
    date: "2022-04-01",
    kind: "metodológica",
    noteMd: "Cambio de metodología de referencia (dato ilustrativo del workbench).",
    sourceUrl: null,
  },
];

export const chartIslandPopulation: ChartPoint[] = [
  { period: "2019-Q1", value: 47026208, status: "D" },
  { period: "2019-Q2", value: 47100396, status: "D" },
  { period: "2019-Q3", value: 47190493, status: "D" },
  { period: "2019-Q4", value: 47332614, status: "D" },
  { period: "2020-Q1", value: 47431256, status: "D" },
  { period: "2020-Q2", value: 47450795, status: "D" },
  { period: "2020-Q3", value: 47415750, status: "D" },
  { period: "2020-Q4", value: 47398695, status: "D" },
];
