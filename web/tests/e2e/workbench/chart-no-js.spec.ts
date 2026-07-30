import { test, expect } from "@playwright/test";
import { WorkbenchPage } from "./workbench-page";

// web-accessibility-gates spec, "The no-JavaScript baseline is a blocking
// gate" — a `javaScriptEnabled: false` context MUST find the SVG chart,
// every break band, the data table, the textual description and the
// methodology sheet present and legible (PRD §14.2). This slice proves it
// for the chart itself against the workbench's static build (the six real
// indicator pages don't exist until slice 9 — `9b.7` re-runs the full gate
// against those pages; this is the FIRST real proof the mechanism works,
// established at the earliest slice that has a chart to test, matching this
// project's own "acceptance gates present by the end of their slice"
// convention).
//
// Most importantly (indicator-page spec, "Breaks survive with JavaScript
// disabled" — principle P4): break bands are baked into the shared
// `renderChartSVG` string, not added by a script, so this context proves
// there is no JS-only band layer that could silently omit them.
test.describe("IndicatorChart — no-JavaScript baseline", () => {
  test.use({ javaScriptEnabled: false });

  test(
    "the SVG chart, its break band, the data table and the textual description are all present with JavaScript disabled",
    { tag: ["@a11y", "@no-js"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();

      const section = page.getByTestId("indicator-chart-section-light");
      await expect(section).toBeVisible();

      // The SVG chart itself.
      const svg = section.locator('[data-testid="indicator-chart-svg"]');
      await expect(svg).toBeVisible();

      // The break band — P4's own guarantee: present with JS disabled,
      // because it is part of the same static SVG string, never a
      // script-appended layer.
      const band = section.locator('[data-testid="chart-break-band"]');
      await expect(band).toHaveCount(1);
      await expect(band).toHaveAttribute("data-break-key", "covid-2020");

      // The accessible data table (the chart's textual equivalent).
      const table = section.locator('[data-testid="accessible-data-table"]');
      await expect(table).toBeVisible();
      const rows = table.locator("tbody tr");
      await expect(rows).toHaveCount(12); // one per fixture observation

      // The generated textual description.
      const description = section.locator('[data-testid="chart-description"]');
      await expect(description).toBeVisible();
      await expect(description).not.toBeEmpty();

      // No loading state, empty placeholder or error region.
      await expect(section).not.toContainText("Cargando");
      await expect(section).not.toContainText("Error");
    },
  );

  // series-transformations spec, "Range presets" (the "personalizado"
  // entry) read together with the spec's own discipline for a preset that
  // cannot apply: "absent, not disabled".
  //
  // A custom range is the one range control that CANNOT work without
  // JavaScript. The five fixed presets are, in principle, expressible as
  // pre-rendered variants; a free-form `[from, to]` pair is not, because
  // these pages are statically built and there is no server to ask. Astro
  // server-renders the island's markup regardless of its `client:idle`
  // directive, so shipping the picker unconditionally would put two date
  // inputs and a commit button in front of a no-JavaScript reader that look
  // exactly like every working control on the page and do nothing at all.
  // The island therefore renders the picker only after `onMount` — absent
  // without JavaScript, never present-but-dead.
  //
  // DISCLOSED, not fixed here: the five fixed preset BUTTONS are still
  // server-rendered and are equally inert without JavaScript. That predates
  // this change and is out of WARNING-5's scope; it is recorded rather than
  // silently swept in.
  test(
    "the custom range picker is absent — not present-but-dead — with JavaScript disabled",
    { tag: ["@a11y", "@no-js", "@custom-range"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      const section = page.getByTestId("chart-island-section-light");

      // The island's own server-rendered markup really is here...
      await expect(section.getByTestId("indicator-chart-island")).toBeVisible();
      await expect(section.locator('[data-testid="accessible-data-table"]')).toBeVisible();
      // ...and the picker deliberately is not.
      await expect(section.getByTestId("chart-custom-range")).toHaveCount(0);
      await expect(section.getByTestId("custom-range-from")).toHaveCount(0);
      await expect(section.getByTestId("custom-range-apply")).toHaveCount(0);
    },
  );

  test(
    "the annotation groups' visible enable controls are present and reachable without JavaScript",
    { tag: ["@a11y", "@no-js"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      const section = page.getByTestId("indicator-chart-section-light");

      const govToggle = section.locator('[data-testid="annotation-toggle-governments"]');
      await expect(govToggle).toBeVisible();

      // Native checkbox+label association works without any script: clicking
      // the visible label toggles the checkbox and reveals its content via
      // pure CSS (`peer-checked:flex`) — proven here, not merely asserted.
      const content = section.locator('[data-testid="annotation-group-content-governments"]');
      await expect(content).toBeHidden();
      await govToggle.click();
      await expect(content).toBeVisible();
    },
  );
});
