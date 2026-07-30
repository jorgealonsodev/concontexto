import type { Locator, Page } from "@playwright/test";
import { BasePage } from "../base-page";

/** Page Object for `/` (milestone 1.2: the homepage lists the six frozen
 * indicators and links to them). Extended, not replaced — this class already
 * existed for the deploy smoke target, and its `heading`/`description`
 * locators still name the same two elements. */
export class HomePage extends BasePage {
  readonly heading: Locator;
  readonly description: Locator;
  /** Every indicator card on the page, in document order. */
  readonly indicatorCards: Locator;

  constructor(page: Page) {
    super(page);
    this.heading = page.getByRole("heading", { level: 1, name: "ConContexto" });
    this.description = page.getByText("Portal de Datos Económicos de España");
    this.indicatorCards = page.getByTestId("indicator-card");
  }

  async goto(): Promise<void> {
    await super.goto("/");
  }

  /** The card linking to one frozen slug, located by the destination it
   * promises rather than by position — a listing that rendered six cards all
   * pointing at the same page would still satisfy a positional locator. */
  cardFor(slug: string): Locator {
    return this.page.locator(`a[data-testid="indicator-card"][href="/indicador/${slug}"]`);
  }

  /** Every focusable/clickable interactive control on the page — the same
   * selector `WorkbenchPage` and `IndicatorPage` use, including the same
   * `.sr-only` carve-out, so the 44 px sweep measures `/` by exactly the rule
   * it measures the other pages by. */
  interactiveControls(): Locator {
    return this.page.locator('a, button, summary, input:not(.sr-only), [tabindex="0"]');
  }
}
