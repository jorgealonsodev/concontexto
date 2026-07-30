import { test, expect } from "@playwright/test";
import { IndicatorPage } from "./indicator-page";

// indicator-page spec, "The page works with JavaScript disabled" +
// web-accessibility-gates spec, "A javaScriptEnabled: false context" — task
// 9a.13. Matches `chart-no-js.spec.ts`'s established pattern: a
// `javaScriptEnabled: false` context proving the SVG chart, break bands,
// the accessible data table, the generated textual description and the
// methodology sheet are all present with no script running at all — because `ChartIsland.svelte`'s markup is
// server-rendered by Astro regardless of its own `client:idle` directive
// (design.md D-5: build-time SVG and client redraw share one module), this
// is the SAME markup a hydrated visit would start from, not a degraded
// stand-in.
// Slice 9b (tasks.md 9b.7) widens this to all six frozen slugs.
const SLUGS = [
  "tasa-de-paro-epa",
  "ocupados-epa",
  "poblacion-residente",
  "ipc-general",
  "ipc-subyacente",
  "pib",
];

// The slugs whose span contains a real break configured in
// `config/rupturas.yaml` (EPA methodology 2021 / IPC base 2021). `pib` and
// `poblacion-residente` have none, and must not grow a band.
const SLUGS_WITH_BREAKS = ["tasa-de-paro-epa", "ocupados-epa", "ipc-general", "ipc-subyacente"];

test.describe("Indicator pages — no-JavaScript baseline", () => {
  test.use({ javaScriptEnabled: false });

  for (const slug of SLUGS) {
    test(
      `/indicador/${slug}: the SVG chart, data table, textual description and methodology sheet are all present with JavaScript disabled`,
      { tag: ["@indicator-page", "@a11y", "@no-js"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();

        await expect(indicator.title).toBeVisible();

        const svg = indicator.chartSection.locator('[data-testid="indicator-chart-svg"]');
        await expect(svg).toBeVisible();

        const table = indicator.chartSection.locator('[data-testid="accessible-data-table"]');
        await expect(table).toBeVisible();

        const description = indicator.chartSection.locator('[data-testid="chart-description"]');
        await expect(description).toBeVisible();
        await expect(description).not.toBeEmpty();

        await expect(indicator.methodologySheet).toBeVisible();

        // verify-report WARNING-6: with three observations per series, every
        // configured methodology break fell outside every span, so no built
        // page ever rendered a break band and this gate had nothing to
        // assert. The full-length fixture puts the EPA-2021 and IPC-base-2021
        // breaks inside their series' spans, so the band is now server-
        // rendered markup a no-JavaScript reader genuinely receives.
        if (SLUGS_WITH_BREAKS.includes(slug)) {
          await expect(indicator.chartSection.getByTestId("chart-breaks")).toBeVisible();
        }

        await expect(page.locator("body")).not.toContainText("Cargando");
        await expect(page.locator("body")).not.toContainText("Error");
      },
    );
  }
});
