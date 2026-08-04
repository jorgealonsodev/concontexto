import type { Locator, Page } from "@playwright/test";
import { BasePage } from "../base-page";

/** The selector the 44 px touch-target sweep walks — the same one
 * `WorkbenchPage` and `IndicatorPage` spell inline, hoisted here so a test
 * can ask whether a given element IS swept rather than restating the rule
 * and risking the two drifting apart. */
export const INTERACTIVE_CONTROL_SELECTOR = 'a, button, summary, input:not(.sr-only), [tabindex="0"]';

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
    return this.page.locator(INTERACTIVE_CONTROL_SELECTOR);
  }

  /** The freshness semaphore inside one card. Matched on the `freshness-`
   * PREFIX because the state is baked into the test id (`freshness-fresh` /
   * `freshness-source-pending`) and a geometry check must not care which of
   * the two states it happens to be measuring. */
  badgeIn(card: Locator): Locator {
    return card.locator('[data-testid^="freshness-"]');
  }

  /**
   * Measures every card as the browser actually lays it out, grouped into
   * the grid rows the reader sees. Rows are derived from each card's own
   * rendered `y`, not from the column count, so this keeps working if the
   * `sm:`/`lg:` breakpoints ever change.
   *
   * Returns rendered boxes rather than class names on purpose: the defect
   * this guards is "the cards in a row are different heights", and a check
   * that greps the markup for `h-full` would pass forever the moment someone
   * achieves the same result by another mechanism — while a card that is
   * genuinely short again would sail through it.
   */
  async cardRows(): Promise<CardGeometry[][]> {
    const cards = this.indicatorCards;
    const count = await cards.count();
    const measured: CardGeometry[] = [];
    for (let i = 0; i < count; i++) {
      const card = cards.nth(i);
      const box = await card.boundingBox();
      const badgeBox = await this.badgeIn(card).boundingBox();
      if (!box || !badgeBox) throw new Error(`card ${i} or its freshness badge has no rendered box`);
      // The card's CONTENT width — its border box less its own border and
      // padding. The badge is compared against this, not against the card's
      // outer width, because the padding is exactly the space the badge is
      // never entitled to occupy anyway.
      const contentWidth = await card.evaluate((el) => {
        const style = getComputedStyle(el);
        return el.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
      });
      measured.push({
        href: (await card.getAttribute("href")) ?? `card ${i}`,
        top: box.y,
        height: box.height,
        contentWidth,
        badgeWidth: badgeBox.width,
        badgeBottom: badgeBox.y + badgeBox.height,
      });
    }

    const rows = new Map<number, CardGeometry[]>();
    for (const card of measured) {
      // Round to whole CSS px: cards that start on the same grid row share a
      // `y` exactly today, but sub-pixel layout is not a promise worth
      // betting a row grouping on.
      const key = Math.round(card.top);
      rows.set(key, [...(rows.get(key) ?? []), card]);
    }
    return [...rows.entries()].sort(([a], [b]) => a - b).map(([, row]) => row);
  }
}

/** One card's rendered geometry, as measured in the browser. */
export interface CardGeometry {
  /** The card's destination, used to name it in failure messages. */
  href: string;
  top: number;
  height: number;
  contentWidth: number;
  badgeWidth: number;
  badgeBottom: number;
}
