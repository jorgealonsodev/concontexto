import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { HomePage } from "./home-page";
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
