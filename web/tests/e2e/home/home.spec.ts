import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { HomePage, INTERACTIVE_CONTROL_SELECTOR } from "./home-page";
import { IndicatorPage } from "../indicator/indicator-page";

// Establishes `npm --prefix web run test:e2e` (web-accessibility-gates
// spec, "Web test commands are declared and blocking") against the real
// production build. Slice 5 wrote the first test here against the deploy
// smoke-target page; milestone 1.2 turns `/` into the site's actual entry
// point and widens this file accordingly — the axe scan below is the same
// scanner, now pointed at a page that has content.
//
// Written alongside the implementation rather than red-first, matching the
// Strict-TDD boundary this project already states for its Playwright
// acceptance gates (see `indicator-pages.spec.ts`'s own header, task 9a.13).
// The behaviour they cover is red-first at the layer below, in
// `test/pages/home.container.test.ts` and `test/indicator/homeListing.test.ts`.

// The six permalinks the indicator-page spec freezes, in the frozen order.
// Restated here rather than imported from `src/`: this is the ACCEPTANCE side
// of the contract, and a gate that read the list from the same module the
// page builds it from could not notice the two disagreeing.
const FROZEN_SLUGS = [
  "tasa-de-paro-epa",
  "ocupados-epa",
  "ipc-general",
  "ipc-subyacente",
  "pib",
  "poblacion-residente",
];

// Production pages ship no runtime theme toggle (the page always renders
// `data-theme="light"` server-side), so the dark pass sets the attribute
// directly on `<html>` after load and re-runs the identical audit — the same
// technique `indicator-pages.spec.ts` uses.
const THEMES = ["light", "dark"] as const;

test.describe("Home", () => {
  for (const theme of THEMES) {
    test(
      `renders the page and has zero automatically-detectable accessibility violations (${theme} theme)`,
      { tag: ["@home", "@design-system", "@a11y"] },
      async ({ page }) => {
        const home = new HomePage(page);
        await home.goto();
        if (theme === "dark") {
          await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
        }

        await expect(home.heading).toBeVisible();
        await expect(home.description).toBeVisible();

        const results = await new AxeBuilder({ page }).analyze();
        expect(results.violations, `${theme}: ${JSON.stringify(results.violations, null, 2)}`).toEqual([]);
      },
    );
  }

  test(
    "lists all six frozen indicators, each linking to its own page",
    { tag: ["@home"] },
    async ({ page }) => {
      const home = new HomePage(page);
      await home.goto();

      await expect(home.indicatorCards).toHaveCount(FROZEN_SLUGS.length);
      for (const slug of FROZEN_SLUGS) {
        await expect(home.cardFor(slug), `no card links to /indicador/${slug}`).toBeVisible();
      }
    },
  );

  test(
    "each listed indicator shows a real latest value with its period and freshness before the reader clicks",
    { tag: ["@home"] },
    async ({ page }) => {
      const home = new HomePage(page);
      await home.goto();

      for (const slug of FROZEN_SLUGS) {
        const card = home.cardFor(slug);
        // A digit, not the not-available dash: a listing whose values were
        // all placeholders would pass a mere visibility check.
        await expect(card, `${slug}: no numeric latest value`).toContainText(/\d/);
        await expect(card.getByTestId("indicator-card-name"), `${slug}: unnamed card`).not.toBeEmpty();
        await expect(card.locator('[data-testid^="freshness-"]'), `${slug}: no freshness semaphore`).toBeVisible();
      }
    },
  );

  test(
    "the homepage really navigates: clicking an indicator opens that indicator's page",
    { tag: ["@home"] },
    async ({ page }) => {
      const home = new HomePage(page);
      await home.goto();

      await home.cardFor("ipc-general").click();

      await expect(page).toHaveURL(/\/indicador\/ipc-general\/?$/);
      await expect(new IndicatorPage(page, "ipc-general").title).toBeVisible();
    },
  );

  test(
    "and back: an indicator page returns the reader to the homepage",
    { tag: ["@home"] },
    async ({ page }) => {
      const indicator = new IndicatorPage(page, "ipc-general");
      await indicator.goto();

      await expect(indicator.backToHome).toBeVisible();
      await indicator.backToHome.click();

      await expect(page).toHaveURL(/\/$/);
      await expect(new HomePage(page).indicatorCards).toHaveCount(FROZEN_SLUGS.length);
    },
  );

  test(
    "every interactive control measures at least 44x44 CSS px at a 375px viewport",
    { tag: ["@home", "@touch-target"] },
    async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 900 });
      const home = new HomePage(page);
      await home.goto();

      const controls = home.interactiveControls();
      const count = await controls.count();
      expect(count).toBeGreaterThan(0);

      const undersized: string[] = [];
      for (let i = 0; i < count; i++) {
        const control = controls.nth(i);
        if (!(await control.isVisible())) continue;
        const box = await control.boundingBox();
        const label = (await control.getAttribute("data-testid")) ?? (await control.innerText()).slice(0, 40);
        if (!box || box.width < 44 || box.height < 44) {
          undersized.push(`${label} (${box ? `${box.width.toFixed(1)}x${box.height.toFixed(1)}` : "no box"})`);
        }
      }
      expect(undersized, `controls under the 44x44 CSS px minimum: ${undersized.join(", ")}`).toEqual([]);
    },
  );

  // Measured in a real browser at 1280 px BEFORE this change, on the built
  // page — the grid was ragged and the badges were zigzagged:
  //
  //   row y=180: tasa-de-paro 156 px, ocupados 156 px, ipc-general 188 px
  //   row y=384: ipc-subyacente 188 px, pib 212 px, poblacion 156 px
  //   badge bottoms, row y=180: 319, 319, 351
  //   badge bottoms, row y=384: 555, 579, 523
  //
  // `HomePage.astro` lays the cards out as grid items, so each `<li>` was
  // already stretched to its row's height — the CARD inside it was not, so
  // a card whose unit wrapped to two lines simply ended lower than its
  // neighbours and dragged its badge down with it.
  //
  // One CSS px of tolerance, not zero: sub-pixel layout is real, and the
  // gaps this guards against are 32 and 56 px.
  const ROW_TOLERANCE_PX = 1;

  test(
    "cards in one grid row render at one height, and their freshness badges share one bottom edge",
    { tag: ["@home", "@layout"] },
    async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 1000 });
      const home = new HomePage(page);
      await home.goto();

      const rows = await home.cardRows();
      // A single-row grid would make every assertion below vacuously true —
      // this test only means something at a width where cards sit beside
      // each other.
      expect(rows.length, "1280px must lay the six cards out in more than one row").toBeGreaterThan(1);
      expect(rows.every((row) => row.length > 1), "every measured row must hold more than one card").toBe(true);

      for (const row of rows) {
        const describe = (pick: (c: (typeof row)[number]) => number) =>
          row.map((c) => `${c.href}=${pick(c).toFixed(1)}`).join(", ");

        const heights = row.map((c) => c.height);
        expect(
          Math.max(...heights) - Math.min(...heights),
          `cards in the row starting at y=${row[0].top.toFixed(1)} render at different heights: ${describe((c) => c.height)}`,
        ).toBeLessThanOrEqual(ROW_TOLERANCE_PX);

        const badgeBottoms = row.map((c) => c.badgeBottom);
        expect(
          Math.max(...badgeBottoms) - Math.min(...badgeBottoms),
          `freshness badges in the row starting at y=${row[0].top.toFixed(1)} do not share a bottom edge: ${describe((c) => c.badgeBottom)}`,
        ).toBeLessThanOrEqual(ROW_TOLERANCE_PX);
      }
    },
  );

  // The badge is a non-interactive `<span>` styled as a pill — `inline-flex
  // … rounded-pill border px-2.5 py-1`. Inside `IndicatorCard`'s `flex
  // flex-col` the default `align-items: stretch` widened it to the card's
  // full content width, so it measured 238.0 x 26.0 CSS px in a 238 CSS px
  // content box at 1280 px (and 293.0 in a 293 px box at 375 px): exactly
  // 100% of the available width, i.e. a full-width bordered box that reads
  // as a button and invites a click that does nothing.
  //
  // 70% is the threshold: 100% fails it by a mile today, and a pill sized to
  // its own text has far more headroom than 30% even for the longer of the
  // two states. It is a SHARE rather than an absolute px width so the same
  // assertion holds at both viewports without two magic numbers.
  const MAX_BADGE_SHARE_OF_CARD = 0.7;

  for (const width of [1280, 375]) {
    test(
      `the freshness badge is sized to its own text, not stretched across the card (${width}px viewport)`,
      { tag: ["@home", "@layout"] },
      async ({ page }) => {
        await page.setViewportSize({ width, height: 1000 });
        const home = new HomePage(page);
        await home.goto();

        const overwide: string[] = [];
        for (const row of await home.cardRows()) {
          for (const card of row) {
            const share = card.badgeWidth / card.contentWidth;
            if (share > MAX_BADGE_SHARE_OF_CARD) {
              overwide.push(
                `${card.href} (badge ${card.badgeWidth.toFixed(1)} px of ${card.contentWidth.toFixed(1)} px content = ${(share * 100).toFixed(1)}%)`,
              );
            }
          }
        }
        expect(
          overwide,
          `freshness badges wider than ${MAX_BADGE_SHARE_OF_CARD * 100}% of their card's content width: ${overwide.join(", ")}`,
        ).toEqual([]);
      },
    );
  }

  test(
    "the freshness badge is not interactive, so the 44px sweep must not reach it",
    { tag: ["@home", "@layout", "@a11y"] },
    async ({ page }) => {
      const home = new HomePage(page);
      await home.goto();

      // Stated as a test because the whole shape argument rests on it: this
      // pill must never be reachable, focusable or clickable, which is
      // exactly why it MUST NOT be padded out to a 44x44 px touch target —
      // and why narrowing it below the card width is safe.
      const badges = page.locator('[data-testid^="freshness-"]');
      const count = await badges.count();
      expect(count).toBeGreaterThan(0);

      const interactive = await badges.evaluateAll((nodes) =>
        nodes
          .filter(
            (node) =>
              node.tagName !== "SPAN" ||
              node.hasAttribute("role") ||
              node.hasAttribute("tabindex") ||
              node.hasAttribute("onclick") ||
              node.hasAttribute("href"),
          )
          .map((node) => node.outerHTML.slice(0, 120)),
      );
      expect(interactive, `freshness badges carrying interactive semantics: ${interactive.join(" | ")}`).toEqual([]);

      // And the sweep's own selector agrees. Applied to the badge ITSELF,
      // not as a descendant query: the card is an `<a>`, so every badge has
      // an interactive ancestor and a descendant query would match all six
      // while proving nothing. If the badge ever gained a `tabindex="0"`,
      // this catches it here rather than as a puzzling 26 px failure in the
      // touch-target test.
      const swept = await badges.evaluateAll(
        (nodes, selector) => nodes.filter((node) => node.matches(selector)).map((node) => node.outerHTML.slice(0, 120)),
        INTERACTIVE_CONTROL_SELECTOR,
      );
      expect(swept, `freshness badges the 44px sweep would measure: ${swept.join(" | ")}`).toEqual([]);
    },
  );

  test(
    "ships no runtime JavaScript — the listing is static markup, not an island",
    { tag: ["@home"] },
    async ({ page }) => {
      const home = new HomePage(page);
      await home.goto();

      // Measured on the REAL built page, not on container output: Astro
      // injects its island runtime at build time, so a stray `client:*`
      // directive anywhere in this page's component tree would only show up
      // here.
      expect(await page.locator("script").count()).toBe(0);
    },
  );
});
