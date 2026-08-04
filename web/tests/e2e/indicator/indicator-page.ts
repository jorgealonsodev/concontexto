import type { Locator, Page } from "@playwright/test";
import { BasePage } from "../base-page";

/** Page Object for any of the six frozen `/indicador/{slug}` routes
 * (indicator-page spec, "Six indicator routes with frozen slugs"). Slice
 * 9a's own real pages are exactly `tasa-de-paro-epa`, `ocupados-epa` and
 * `poblacion-residente` — the remaining three build once slice 9b lands,
 * reusing this SAME Page Object (playwright skill convention: reuse, don't
 * recreate). */
export class IndicatorPage extends BasePage {
  readonly slug: string;
  readonly root: Locator;
  readonly title: Locator;
  readonly chartSection: Locator;
  readonly points: Locator;
  readonly methodologySheet: Locator;
  readonly actionBar: Locator;
  readonly relatedIndicators: Locator;
  /** The range-preset control group. Present only when more than one preset
   * applies to the series' span — which is why it was absent from every built
   * page while the export fixture carried three periods per series
   * (verify-report WARNING-6). */
  readonly rangeControls: Locator;
  /** The "personalizado" range picker (series-transformations spec, "Range
   * presets"; verify-report WARNING-5). Rendered only after hydration — it
   * cannot work without JavaScript on a statically built page, so it is
   * absent rather than dead there. */
  readonly customRangeFrom: Locator;
  readonly customRangeTo: Locator;
  readonly customRangeApply: Locator;
  readonly customRangeStatus: Locator;
  /** The government range control (indicator-page spec, "Annotation layers per
   * PRD §6.1.1(a)"). Rendered only after hydration, and only when at least one
   * government's term genuinely narrows this series — a `<select>` whose
   * options are the editorial registry's own event ids. */
  readonly governmentRange: Locator;
  readonly governmentSelect: Locator;
  readonly governmentStatus: Locator;
  /** The change-of-government markers inside the WIDE drawing — the chart's
   * third vertical treatment, beside the provisional dash and the break band.
   * Scoped to one variant because both are in the document at once (CSS shows
   * one), and an unscoped locator would count every marker twice. */
  readonly governmentMarkers: Locator;
  readonly governmentMarkersNarrow: Locator;
  /** The legend entry that teaches what a marker means. Absent — not empty —
   * when the visible window carries no change of government. */
  readonly legendGovernment: Locator;
  /** The editorial event spans inside the WIDE drawing — the chart's fourth
   * annotation treatment, and the only one about an interval. Scoped to one
   * variant for the same reason the government markers are: both drawings are
   * in the document at once and an unscoped locator would count every rail
   * twice. */
  readonly eventSpans: Locator;
  readonly eventSpansNarrow: Locator;
  /** The legend entry that teaches what a rail means, and the sentence that is
   * a screen-reader reader's only route to the rails (the drawing is a single
   * `role="img"`, which prunes its own children). */
  readonly legendEventSpan: Locator;
  readonly eventSpansNote: Locator;
  readonly yoyVariation: Locator;
  /** The link back to `/` (milestone 1.2). Rendered by `IndicatorPage.astro`,
   * so all six routes carry it. */
  readonly backToHome: Locator;
  /** The page header — a `flex flex-col`, which is why the freshness badge
   * inside it stretched to the full column width before this was measured. */
  readonly header: Locator;
  /** The header's freshness semaphore. Matched on the `freshness-` PREFIX
   * because the state is baked into the test id and a geometry check must
   * not care which of the two states it is measuring. */
  readonly headerFreshness: Locator;
  /** Every card in the "Indicadores relacionados" strip, in document order. */
  readonly relatedCards: Locator;

  constructor(page: Page, slug: string) {
    super(page);
    this.slug = slug;
    this.root = page.getByTestId("indicator-page");
    this.title = page.getByTestId("page-title");
    this.chartSection = page.getByTestId("page-chart-section");
    this.points = this.chartSection.getByTestId("chart-island-point");
    this.methodologySheet = page.getByTestId("methodology-sheet");
    this.actionBar = page.getByTestId("action-bar");
    this.relatedIndicators = page.getByTestId("related-indicators");
    this.rangeControls = this.chartSection.getByTestId("chart-range-controls");
    this.customRangeFrom = this.chartSection.getByTestId("custom-range-from");
    this.customRangeTo = this.chartSection.getByTestId("custom-range-to");
    this.customRangeApply = this.chartSection.getByTestId("custom-range-apply");
    this.customRangeStatus = this.chartSection.getByTestId("custom-range-status");
    this.governmentRange = this.chartSection.getByTestId("chart-government-range");
    this.governmentSelect = this.chartSection.getByTestId("government-select");
    this.governmentStatus = this.chartSection.getByTestId("government-range-status");
    this.governmentMarkers = this.chartSection.locator('[data-testid="chart-government-marker"]');
    this.governmentMarkersNarrow = this.chartSection.locator('[data-testid="chart-government-marker-narrow"]');
    this.legendGovernment = this.chartSection.getByTestId("chart-legend-government");
    this.eventSpans = this.chartSection.locator('[data-testid="chart-event-span"]');
    this.eventSpansNarrow = this.chartSection.locator('[data-testid="chart-event-span-narrow"]');
    this.legendEventSpan = this.chartSection.getByTestId("chart-legend-event-span");
    this.eventSpansNote = this.chartSection.getByTestId("chart-event-spans-note");
    this.yoyVariation = page.getByTestId("page-yoy-variation");
    this.backToHome = page.getByTestId("back-to-home");
    this.header = page.getByTestId("page-header");
    this.headerFreshness = this.header.locator('[data-testid^="freshness-"]');
    this.relatedCards = this.relatedIndicators.getByTestId("indicator-card");
  }

  /** The freshness semaphore inside one related-indicator card. */
  badgeIn(card: Locator): Locator {
    return card.locator('[data-testid^="freshness-"]');
  }

  /** One range-preset button by its preset key ("full", "5y", "10y",
   * "since-2008", "since-2018"). */
  rangePreset(preset: string): Locator {
    return this.chartSection.getByTestId(`range-preset-${preset}`);
  }

  /** One annotation group's show/hide control ("governments", "exogenous",
   * "milestones") — the same gesture that projects that group's event spans
   * onto the drawing. */
  annotationToggle(group: string): Locator {
    return this.chartSection.getByTestId(`annotation-toggle-${group}`);
  }

  /** One point button by the period its announcement names — "T1 2020" and so
   * on, the prose register `pointLabel` builds its `aria-label` from. Used to
   * measure WHERE a projected span really landed against the observation a
   * reader would look for. */
  pointFor(periodLabel: string): Locator {
    return this.chartSection.locator(
      `[data-testid="chart-island-point"][aria-label^="${periodLabel}:"]`,
    );
  }

  async goto(): Promise<void> {
    await super.goto(`/indicador/${this.slug}`);
  }

  /** Same hydration-wait rationale as `WorkbenchPage.waitForChartIslandHydrated`
   * (`tests/e2e/workbench/workbench-page.ts`): the island's markup exists
   * server-rendered before `client:idle` finishes attaching handlers —
   * interacting before that silently does nothing. */
  async waitForChartHydrated(): Promise<void> {
    await this.points.first().waitFor();
    await this.page.waitForLoadState("networkidle");
  }

  /** Every focusable/clickable interactive control on the whole page — same
   * convention as `WorkbenchPage.interactiveControls()`, including the same
   * `input` rule and the same `.sr-only` carve-out (documented there).
   *
   * `select` was added with the government range control, for exactly the
   * reason `input` was added with the custom range picker: the sweep measures
   * what it lists, so a control type absent from this selector ships
   * unmeasured. It is the first `<select>` in this product. */
  interactiveControls(): Locator {
    return this.page.locator('a, button, summary, select, input:not(.sr-only), [tabindex="0"]');
  }
}
