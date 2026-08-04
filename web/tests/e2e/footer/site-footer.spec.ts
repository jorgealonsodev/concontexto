import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { HomePage, INTERACTIVE_CONTROL_SELECTOR } from "../home/home-page";
import { IndicatorPage } from "../indicator/indicator-page";
import { WorkbenchPage } from "../workbench/workbench-page";

// The acceptance side of the site footer. Written alongside the
// implementation rather than red-first, matching the Strict-TDD boundary this
// project already states for its Playwright acceptance gates (see
// `indicator-pages.spec.ts` and `home.spec.ts` headers); the behaviour is
// driven red-first one layer down, in `test/pages/site-footer.test.ts`.
//
// A DISCLOSED LIMIT, STATED HERE RATHER THAN DISCOVERED LATER. This suite
// runs against `astro preview` over `web/dist`, and `dist/` does NOT contain
// `/transparencia/raw-files.sha256` or `/data-derived/**`. Both are RUNTIME
// artifacts: the Go binary writes them into the container's mounted volumes
// on every ingest (Dockerfile "dist/transparencia — each ingest copies
// app_data/raw_files.sha256 to STATIC_ROOT/transparencia/raw-files.sha256";
// docker-compose.yml's `raw_hashes` volume). So this suite asserts the footer
// EMITS the right href and never that the href returns 200 here — it cannot,
// and a test that demanded it would fail forever on a correct footer. The
// href itself is pinned against the writer's own Go source in
// `test/pages/site-footer.test.ts`; that its 200 holds in production was
// verified against the running container.
//
// The same limit already applies to the action bar's CSV/JSON links, which
// this suite has never asserted resolve either, for the same reason.

const THEMES = ["light", "dark"] as const;

/** Every document this site serves, with the page object that knows how to
 * get there. The footer is site-level furniture: "on every page" is the whole
 * claim, so the sweep below is over all three rather than over the one that
 * happened to be convenient. */
const PAGES = [
  { name: "/", goto: async (page: import("@playwright/test").Page) => new HomePage(page).goto() },
  { name: "/indicador/pib", goto: async (page: import("@playwright/test").Page) => new IndicatorPage(page, "pib").goto() },
  { name: "/workbench", goto: async (page: import("@playwright/test").Page) => new WorkbenchPage(page).goto() },
];

test.describe("Site footer", () => {
  for (const { name, goto } of PAGES) {
    test(
      `is present, once, as a contentinfo landmark on ${name}`,
      { tag: ["@footer", "@a11y"] },
      async ({ page }) => {
        await goto(page);

        const footer = page.getByTestId("site-footer");
        await expect(footer).toBeVisible();
        // Exactly one: a second contentinfo would make the landmark
        // ambiguous for a screen-reader user and trip axe's landmark rules.
        await expect(page.locator("footer")).toHaveCount(1);
        // Outside `<main>`, which is what makes it contentinfo at all — a
        // `<footer>` nested inside `<main>` has no landmark role.
        expect(await footer.evaluate((el) => el.closest("main") === null)).toBe(true);
        await expect(page.getByRole("contentinfo")).toBeVisible();
      },
    );
  }

  test(
    "emits exactly the four site-level destinations, and nothing that duplicates a per-series link",
    { tag: ["@footer"] },
    async ({ page }) => {
      await new HomePage(page).goto();

      const hrefs = await page.getByTestId("site-footer").locator("a").evaluateAll((nodes) =>
        nodes.map((node) => node.getAttribute("href") ?? ""),
      );

      expect(hrefs).toEqual([
        "https://github.com/jorgealonsodev/concontexto",
        "https://github.com/jorgealonsodev/concontexto/blob/main/LICENSE",
        "https://github.com/jorgealonsodev/concontexto/tree/main/config/sources",
        "/transparencia/raw-files.sha256",
      ]);
    },
  );

  test(
    "every footer link measures at least 44x44 CSS px at a 375 px viewport",
    { tag: ["@footer", "@touch-target"] },
    async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 900 });
      await new HomePage(page).goto();

      const links = page.getByTestId("site-footer").locator(INTERACTIVE_CONTROL_SELECTOR);
      const count = await links.count();
      // The footer's links are the newest interactive controls on the site,
      // and the page-wide sweeps in `home.spec.ts` / `indicator-pages.spec.ts`
      // already measure them. This asserts the count is non-zero so that
      // those sweeps cannot pass vacuously over a footer that rendered none.
      expect(count).toBe(4);

      const undersized: string[] = [];
      for (let i = 0; i < count; i++) {
        const box = await links.nth(i).boundingBox();
        const label = (await links.nth(i).innerText()).slice(0, 50);
        if (!box || box.width < 44 || box.height < 44) {
          undersized.push(`${label} (${box ? `${box.width.toFixed(1)}x${box.height.toFixed(1)}` : "no box"})`);
        }
      }
      expect(undersized, `footer links under the 44x44 CSS px minimum: ${undersized.join(", ")}`).toEqual([]);
    },
  );

  for (const theme of THEMES) {
    test(
      `introduces zero automatically-detectable accessibility violations (${theme} theme)`,
      { tag: ["@footer", "@a11y"] },
      async ({ page }) => {
        await new HomePage(page).goto();
        if (theme === "dark") {
          await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
        }

        // Scoped to the footer so a failure here names the footer rather than
        // the page; the unscoped page-wide scans in `home.spec.ts` and
        // `indicator-pages.spec.ts` already cover it in context.
        const results = await new AxeBuilder({ page }).include('[data-testid="site-footer"]').analyze();
        expect(results.violations, `${theme}: ${JSON.stringify(results.violations, null, 2)}`).toEqual([]);
      },
    );
  }

  test(
    "says no licence covers all the data, and names no licence for it",
    { tag: ["@footer"] },
    async ({ page }) => {
      await new HomePage(page).goto();

      const text = (await page.getByTestId("site-footer").innerText()).replace(/\s+/g, " ");

      // Read off the BUILT page, not off the source module: the constraint is
      // about what a reader sees. `CC BY` next to the data would be the
      // overclaim `source-attribution-licensing` forbids.
      expect(text).toMatch(/no est[áa]n cubiertos por una licencia [úu]nica/i);
      expect(text).not.toMatch(/cc\s*by/i);
      expect(text).not.toMatch(/creative\s*commons/i);
    },
  );

  test(
    "adds no runtime JavaScript to the pages that had none",
    { tag: ["@footer"] },
    async ({ page }) => {
      await new HomePage(page).goto();

      expect(await page.locator("script").count()).toBe(0);
    },
  );
});
