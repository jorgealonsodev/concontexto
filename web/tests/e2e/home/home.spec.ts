import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { HomePage } from "./home-page";

// Establishes `npm --prefix web run test:e2e` (web-accessibility-gates
// spec, "Web test commands are declared and blocking") against the real
// production build. A real first test, not a placeholder: it exercises the
// deploy smoke-target page through the design-token pipeline this slice
// built (theme.css / Tailwind / Astro build) and runs the same axe-core
// scanner the full six-page suite (tasks.md 9b.4) will run later.
test.describe("Home (deploy smoke target)", () => {
  test(
    "renders the page and has zero automatically-detectable accessibility violations",
    { tag: ["@design-system", "@a11y"] },
    async ({ page }) => {
      const home = new HomePage(page);
      await home.goto();

      await expect(home.heading).toBeVisible();
      await expect(home.description).toBeVisible();

      const results = await new AxeBuilder({ page }).analyze();
      expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
    },
  );
});
