import type { Locator, Page } from "@playwright/test";
import { BasePage } from "../base-page";

export class WorkbenchPage extends BasePage {
  readonly lightSection: Locator;
  readonly darkSection: Locator;
  /** Slice 8's ChartIsland workbench entry (light theme only — the
   * interactive specs don't need to duplicate every assertion per theme;
   * `workbench.spec.ts`'s existing contrast sweep already covers both
   * themes' visual tokens). */
  readonly chartIsland: Locator;

  constructor(page: Page) {
    super(page);
    this.lightSection = page.getByTestId("workbench-light");
    this.darkSection = page.getByTestId("workbench-dark");
    this.chartIsland = this.lightSection.getByTestId("indicator-chart-island");
  }

  async goto(): Promise<void> {
    await super.goto("/workbench");
  }

  /** Waits for `ChartIsland.svelte`'s `client:idle` hydration to actually
   * finish attaching its own event listeners, not merely for its markup to
   * exist in the (server-rendered) DOM — the two are NOT the same moment,
   * and interacting with the island before hydration completes (e.g.
   * `.focus()`ing a point) silently does nothing, since the framework
   * hasn't wired handlers yet. Every interactive-behaviour test needs this;
   * the no-JS baseline test (`chart-no-js.spec.ts`) deliberately never
   * calls it (`javaScriptEnabled: false` — hydration never happens there).
   */
  async waitForChartIslandHydrated(): Promise<void> {
    await this.chartIsland.getByTestId("chart-island-point").first().waitFor();
    await this.page.waitForLoadState("networkidle");
  }

  /** Every focusable/clickable interactive control across the whole page:
   * links, buttons, native `<summary>` disclosure triggers (MethodologySheet's
   * mobile toggle), form inputs (the custom range's two date fields) and any
   * element carrying an explicit `tabindex="0"` (BreakBand's tooltip
   * trigger). Matches the design-system spec's own scenario wording ("any
   * interactive control"), not just `<a>`/`<button>`.
   *
   * `input` was missing until the custom range picker added the first real
   * form control to this product — so the 44 px sweep would have shipped
   * blind to it. `.sr-only` inputs are the one deliberate exclusion:
   * `IndicatorChart.astro`'s annotation toggles are a visually-hidden native
   * checkbox whose ENTIRE touch target is the associated `<label>`, which
   * this selector already measures. Measuring the 1x1 px checkbox itself
   * would fail a control that is, in the reader's hands, 44 px tall. */
  interactiveControls(): Locator {
    // `select` joined the list with the government range control (the first
    // `<select>` in this product): the sweep measures exactly what it lists,
    // so an unlisted control type ships unmeasured — the same gap `input`
    // filled when the custom range picker arrived.
    return this.page.locator('a, button, summary, select, input:not(.sr-only), [tabindex="0"]');
  }
}
