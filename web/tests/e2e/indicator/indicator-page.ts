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
  readonly yoyVariation: Locator;
  /** The link back to `/` (milestone 1.2). Rendered by `IndicatorPage.astro`,
   * so all six routes carry it. */
  readonly backToHome: Locator;

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
    this.yoyVariation = page.getByTestId("page-yoy-variation");
    this.backToHome = page.getByTestId("back-to-home");
  }

  /** One range-preset button by its preset key ("full", "5y", "10y",
   * "since-2008", "since-2018"). */
  rangePreset(preset: string): Locator {
    return this.chartSection.getByTestId(`range-preset-${preset}`);
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
   * `input` rule and the same `.sr-only` carve-out (documented there). */
  interactiveControls(): Locator {
    return this.page.locator('a, button, summary, input:not(.sr-only), [tabindex="0"]');
  }
}
