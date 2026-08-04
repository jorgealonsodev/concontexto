import { test, expect } from "@playwright/test";
import { WorkbenchPage } from "./workbench-page";

// The POLICY-MEASURE annotation layer, in a real browser, on real built
// markup.
//
// WHY THIS SUITE EXISTS SEPARATELY FROM THE UNIT TESTS. The unit tests assert
// the numbers `buildPolicyMeasureMarks` produces; only a browser can confirm
// what those numbers become on screen. The single claim this layer has to make
// good on is geometric — the mark never enters the plot area, because a mark
// crossing the data where the curve turns would assert an effect by adjacency,
// with no author and no citable source — and that claim is settled by measured
// bounding boxes, not by arithmetic in a test's head.
//
// Run against the workbench rather than an indicator page because the built
// e2e artifact's own registry carries no measures: the workbench's fixture
// does, and it is the same island, the same renderer and the same toggle a
// reader meets on /indicador/{slug}.
test.describe("ChartIsland — the policy-measure layer", () => {
  test(
    "the measure's mark is drawn entirely below the axis, never across the data",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      // The WIDE drawing specifically. Both variants are emitted into the
      // document and CSS shows one (`lib/chart/geometry.ts`), so an unscoped
      // locator resolves to two nodes, one of them `display: none` at this
      // viewport — and the hidden one has no measurable box.
      const wide = workbench.chartIsland.getByTestId("indicator-chart-svg");

      // Located rather than asserted `toBeVisible()`: an SVG `<line>` has a
      // zero-width geometric bounding box, which Playwright's visibility
      // heuristic reads as hidden even while the browser paints its stroke.
      // The height check below is the real evidence that something is drawn.
      const mark = wide.locator('[data-measure-id="medida-ejemplo-2021"] line');
      await expect(mark).toHaveCount(1);

      // The axis line is the plot area's bottom edge. Measured in the SVG's
      // own user units rather than in CSS pixels, because that is where the
      // claim is made and a scaled viewBox would only add rounding.
      const axisY = await wide
        .locator(".chart-axis")
        .evaluate((node) => Number(node.getAttribute("y1")));
      const stub = await mark.evaluate((node) => ({
        y1: Number(node.getAttribute("y1")),
        y2: Number(node.getAttribute("y2")),
      }));

      expect(stub.y1).toBeGreaterThan(axisY);
      expect(stub.y2).toBeGreaterThan(stub.y1);

      // And it is really painted, at a size a person can see.
      const painted = await mark.evaluate((node) => (node as SVGGraphicsElement).getBoundingClientRect().height);
      expect(painted).toBeGreaterThan(4);
    },
  );

  test(
    "the mark sits below the axis tick labels rather than being drawn through one",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      // The gutter holds two things and only two: this mark and the period
      // labels. The unit tests derive the clearance; this measures the real
      // rendered boxes, which is the only way to know the derivation still
      // holds for the type the browser actually set.
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const wide = workbench.chartIsland.getByTestId("indicator-chart-svg");

      const markBox = await wide
        .locator('[data-measure-id="medida-ejemplo-2021"] line')
        .boundingBox();
      expect(markBox).not.toBeNull();

      const tickBoxes = await wide.locator(".chart-tick--x").evaluateAll((nodes) =>
        nodes.map((node) => (node as SVGGraphicsElement).getBoundingClientRect()).map((r) => ({
          top: r.top,
          bottom: r.bottom,
          left: r.left,
          right: r.right,
        })),
      );
      expect(tickBoxes.length).toBeGreaterThan(0);

      const box = markBox as { x: number; y: number; width: number; height: number };
      for (const tick of tickBoxes) {
        // The mark sits in the band UNDER the labels, so wherever the two
        // share horizontal space it must begin below the label's own box.
        const overlapsHorizontally = box.x <= tick.right && tick.left <= box.x + box.width;
        if (!overlapsHorizontally) continue;
        expect(box.y).toBeGreaterThanOrEqual(tick.bottom);
      }
    },
  );

  test(
    "the sentence beside the chart names the instrument, its date, and no consequence",
    { tag: ["@chart-island", "@a11y"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const note = workbench.chartIsland.getByTestId("chart-measures-note");

      await expect(note).toContainText("entrada en vigor");
      await expect(note).toContainText("Medida de política pública (ejemplo)");
      await expect(note).toContainText("15 de septiembre de 2021");
      // The refusal, on the built page: the chart says outright that it
      // represents no relation between those measures and the series.
      await expect(note).toContainText("no representa ninguna relación");
    },
  );

  test(
    "closing the group removes the mark, its legend entry and its sentence together",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      // Nothing is drawn unless it is selected — the same rule the government
      // and event-span layers already answer. The three have to disappear
      // TOGETHER: a legend entry for a mark that is not drawn is a lie, and a
      // sentence naming instruments the drawing does not carry is a worse one.
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      // One per drawing — the wide one and the narrow one, of which CSS shows
      // exactly one at any viewport.
      await expect(island.locator('[data-measure-id="medida-ejemplo-2021"]')).toHaveCount(2);
      await expect(island.getByTestId("chart-legend-measure")).toBeVisible();

      await island.getByTestId("annotation-toggle-measures").click();

      await expect(island.locator('[data-measure-id="medida-ejemplo-2021"]')).toHaveCount(0);
      await expect(island.getByTestId("chart-legend-measure")).toHaveCount(0);
      await expect(island.getByTestId("chart-measures-note")).toHaveText("");
    },
  );

  test(
    "narrowing past the measure removes its chip, its mark and its sentence together",
    { tag: ["@chart-island", "@annotation-range"] },
    async ({ page }) => {
      // The reported defect, for the group it was reported on. On
      // /indicador/tasa-de-paro-epa with the measures group open, selecting
      // "Desde 2018" narrowed the series from 98 points to 34 and the measure
      // stubs from 12 to 8 — and left the chip row naming all three measures,
      // including a 2012 reform, under a chart beginning in 2018.
      //
      // Proved HERE rather than on that page for the reason this file's header
      // already gives: the built e2e artifact's registry carries no measures,
      // and the workbench's fixture does. Same island, same renderer, same
      // toggle a reader meets on /indicador/{slug}.
      //
      // The fixture's one measure enters into force on 2021-09-15, so a custom
      // range ending in 2020 puts it outside the window while leaving real
      // data on screen — a narrowing, not an emptying.
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;
      const chips = island
        .getByTestId("annotation-group-content-measures")
        .getByTestId("annotation-chip");

      await expect(chips).toHaveCount(1);
      await expect(island.locator('[data-measure-id="medida-ejemplo-2021"]')).toHaveCount(2);

      await island.getByTestId("custom-range-from").fill("2019-01-01");
      await island.getByTestId("custom-range-to").fill("2020-12-31");
      await island.getByTestId("custom-range-apply").click();
      await expect(island.getByTestId("custom-range-status")).toContainText(
        "Rango personalizado aplicado",
      );

      // All three together. A chip naming an instrument the drawing does not
      // mark is the defect; a sentence that still named it would be worse,
      // because it is the only identification a measure ever gets.
      await expect(island.locator('[data-measure-id="medida-ejemplo-2021"]')).toHaveCount(0);
      await expect(island.getByTestId("chart-measures-note")).toHaveText("");
      // The group is ABSENT, not present and empty — its only entry is out of
      // range, so its toggle could not have changed one pixel of the page.
      await expect(island.getByTestId("annotation-toggle-measures")).toHaveCount(0);
      await expect(chips).toHaveCount(0);
    },
  );

  test(
    "the measure's chip links to the primary source the registry recorded",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      // A measure's whole content is a date, and `validate-config` requires
      // every entry to carry the document that date was verified against. The
      // chip is where a reader reaches it: without the link the page would
      // name an instrument and offer no way to check it.
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      const chip = island
        .getByTestId("annotation-group-content-measures")
        .getByTestId("annotation-chip")
        .first();
      await expect(chip).toBeVisible();
      await expect(chip).toHaveAttribute("href", /boe\.es/);
    },
  );
});
