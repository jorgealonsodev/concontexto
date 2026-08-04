import { test, expect } from "@playwright/test";
import { WorkbenchPage } from "./workbench-page";

// PRD §14.2's ONE interactive island. Acceptance gates written alongside
// (web-accessibility-gates spec's own stated test-discipline boundary — a
// red-first browser audit proves nothing), mutation-checked where this
// project's own convention calls for real evidence (P4's break-band
// survival, the no-network-request contract).
test.describe("ChartIsland — interactive behaviour", () => {
  test(
    "hovering or focusing a point discloses its value, period and provisional/definitive status",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      const firstPoint = island.getByTestId("chart-island-point").first();
      await firstPoint.focus();

      const tooltip = island.getByTestId("chart-island-tooltip");
      await expect(tooltip).toBeVisible();
      // The tooltip is prose a reader reads and a screen reader announces, so
      // it names the period in the prose register — "T1 2019", INE's own
      // vocabulary — never the database's "2019-Q1" storage format.
      await expect(tooltip).toContainText("T1 2019");
      await expect(tooltip).toContainText("Definitivo");
    },
  );

  test(
    "keyboard traversal reaches every point with a visible focus indicator, and Tab leaves the chart with no trap",
    { tag: ["@chart-island", "@a11y"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      const points = island.getByTestId("chart-island-point");
      const count = await points.count();
      expect(count).toBeGreaterThan(1);

      await points.first().focus();
      const seenLabels = new Set<string>();
      seenLabels.add((await points.first().getAttribute("aria-label")) ?? "");

      for (let i = 1; i < count; i++) {
        // Read the TARGET element's id before pressing, then use an
        // auto-retrying assertion (`toBeFocused`) rather than a one-shot
        // `:focus` read — robust against parallel-worker scheduling jitter
        // between the keypress and the synchronous `.focus()` call inside
        // the component's own keydown handler actually landing.
        const target = points.nth(i);
        const targetId = await target.getAttribute("id");
        await page.keyboard.press("ArrowRight");
        await expect(page.locator(`#${targetId}`)).toBeFocused();
        seenLabels.add((await target.getAttribute("aria-label")) ?? "");
        // A visible focus indicator (AA-contrast `outline-accent`, the same
        // already-verified token every other focusable control in this
        // product uses) — a real, non-zero outline width, not merely
        // present in the stylesheet.
        const outlineWidth = await target.evaluate((el) => getComputedStyle(el).outlineWidth);
        expect(outlineWidth).not.toBe("0px");
      }
      // Every point was reachable — no two arrow-key presses landed on the
      // same aria-label (each point's period is unique in this fixture).
      expect(seenLabels.size).toBe(count);

      // ArrowRight at the last point is a clamped no-op (never wraps/traps);
      // Tab leaves the chart into the next focusable control on the page.
      await page.keyboard.press("ArrowRight");
      await page.keyboard.press("Tab");
      const afterTab = page.locator(":focus");
      await expect(afterTab).not.toHaveAttribute("data-testid", "chart-island-point");
    },
  );

  test(
    "selecting a range preset updates the permalink; reloading it reproduces the same range",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      const fullCount = await island.getByTestId("chart-island-point").count();

      const fiveYearButton = island.getByTestId("range-preset-5y");
      await fiveYearButton.click();
      await expect(fiveYearButton).toHaveAttribute("aria-pressed", "true");

      await expect(page).toHaveURL(/range=5y/);
      const narrowedCount = await island.getByTestId("chart-island-point").count();
      expect(narrowedCount).toBeLessThan(fullCount);

      await page.reload();
      const reloadedWorkbench = new WorkbenchPage(page);
      await reloadedWorkbench.waitForChartIslandHydrated();
      const reloadedIsland = reloadedWorkbench.chartIsland;
      await expect(reloadedIsland.getByTestId("range-preset-5y")).toHaveAttribute("aria-pressed", "true");
      const reloadedCount = await reloadedIsland.getByTestId("chart-island-point").count();
      expect(reloadedCount).toBe(narrowedCount);
    },
  );

  // series-transformations spec, "Range presets" — the "personalizado"
  // entry (verify-report WARNING-5). The workbench island's fixture spans
  // 2019-Q1..2026-Q1, so a 2020..2021 window is a genuine narrowing and a
  // 1990..1995 window genuinely holds nothing.
  test(
    "committing a custom range narrows the chart, encodes both bounds in the permalink and survives a reload",
    { tag: ["@chart-island", "@custom-range"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      const fullCount = await island.getByTestId("chart-island-point").count();

      await island.getByTestId("custom-range-from").fill("2020-01-01");
      await island.getByTestId("custom-range-to").fill("2021-12-31");
      await island.getByTestId("custom-range-apply").click();

      const narrowedCount = await island.getByTestId("chart-island-point").count();
      expect(narrowedCount).toBeLessThan(fullCount);
      expect(narrowedCount).toBeGreaterThan(0);

      // Both bounds travel as PERIOD LABELS: the calendar day the reader
      // picked inside 2021-Q4 is not part of the chart's own vocabulary.
      await expect(page).toHaveURL(/range=custom/);
      await expect(page).toHaveURL(/from=2020-Q1/);
      await expect(page).toHaveURL(/to=2021-Q4/);

      // No fixed preset claims to be selected while a custom range is active.
      await expect(island.getByTestId("range-preset-full")).toHaveAttribute("aria-pressed", "false");
      await expect(island.getByTestId("range-preset-5y")).toHaveAttribute("aria-pressed", "false");

      await page.reload();
      const reloaded = new WorkbenchPage(page);
      await reloaded.waitForChartIslandHydrated();
      const reloadedIsland = reloaded.chartIsland;
      expect(await reloadedIsland.getByTestId("chart-island-point").count()).toBe(narrowedCount);
      // The inputs are re-populated from the permalink, so the reader can
      // see and adjust the range they arrived at rather than facing two
      // blank fields under a chart that is already narrowed. Each bound
      // prefills at the FIRST day of its period — a documented, stable
      // round-trip (2021-Q4 -> 2021-10-01 -> 2021-Q4), not the exact day
      // originally typed.
      await expect(reloadedIsland.getByTestId("custom-range-from")).toHaveValue("2020-01-01");
      await expect(reloadedIsland.getByTestId("custom-range-to")).toHaveValue("2021-10-01");
    },
  );

  test(
    "a custom range running past the series' span is clamped and the narrowing is disclosed, never silent",
    { tag: ["@chart-island", "@custom-range"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      // 1990 is 29 years before this series' first observation. The missing
      // years cannot be rendered, so the range is clamped — and the reader
      // is told, in Spanish, that it was.
      await island.getByTestId("custom-range-from").fill("1990-01-01");
      await island.getByTestId("custom-range-to").fill("2020-12-31");
      await island.getByTestId("custom-range-apply").click();

      const status = island.getByTestId("custom-range-status");
      await expect(status).toContainText("más allá del periodo con datos");
      // The disclosed span is the one actually rendered, not the one asked for.
      await expect(status).toContainText("T1 2019");
      await expect(status).toContainText("T4 2020");
      // …and the URL still carries the CANONICAL bounds. This pair of
      // assertions is the boundary in one place: the sentence is for the
      // reader, the query parameter is parsed back by `resolveCustomRange`,
      // and they must not be the same string.
      await expect(page).toHaveURL(/from=2019-Q1/);
      await expect(page).not.toHaveURL(/from=T1/);
    },
  );

  test(
    "a custom range holding no observation is refused and the chart is left exactly as it was",
    { tag: ["@chart-island", "@custom-range"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      const before = await island.getByTestId("chart-island-point").count();

      await island.getByTestId("custom-range-from").fill("1990-01-01");
      await island.getByTestId("custom-range-to").fill("1995-12-31");
      await island.getByTestId("custom-range-apply").click();

      await expect(island.getByTestId("custom-range-status")).toContainText("no tiene ningún dato");
      await expect(island.getByTestId("custom-range-from")).toHaveAttribute("aria-invalid", "true");
      // Refused, not "applied to nothing": the chart still shows what it did.
      expect(await island.getByTestId("chart-island-point").count()).toBe(before);
      await expect(page).not.toHaveURL(/range=custom/);
    },
  );

  test(
    "an inverted custom range is refused with its own message",
    { tag: ["@chart-island", "@custom-range"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      await island.getByTestId("custom-range-from").fill("2022-01-01");
      await island.getByTestId("custom-range-to").fill("2020-01-01");
      await island.getByTestId("custom-range-apply").click();

      await expect(island.getByTestId("custom-range-status")).toContainText("anterior a la de fin");
      await expect(page).not.toHaveURL(/range=custom/);
    },
  );

  test(
    "a hand-edited, out-of-span custom permalink degrades to the full range instead of crashing or rendering nothing",
    { tag: ["@chart-island", "@custom-range"] },
    async ({ page }) => {
      const consoleErrors: string[] = [];
      page.on("pageerror", (error) => consoleErrors.push(error.message));

      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const fullCount = await workbench.chartIsland.getByTestId("chart-island-point").count();

      // Every one of these is something a reader can type into the address
      // bar: bounds far outside the span, an unparseable label, an inverted
      // pair, and a half-specified range.
      for (const search of [
        "?range=custom&from=1900-Q1&to=1901-Q1",
        "?range=custom&from=banana&to=2020-Q1",
        "?range=custom&from=2022-Q1&to=2019-Q1",
        "?range=custom&from=2020-Q1",
      ]) {
        await page.goto(`/workbench${search}`);
        const island = new WorkbenchPage(page).chartIsland;
        await island.getByTestId("chart-island-point").first().waitFor();
        await expect
          .poll(async () => island.getByTestId("chart-island-point").count(), {
            message: `"${search}" did not degrade to the full range`,
          })
          .toBe(fullCount);
        // Degraded silently, by design: there is no reader gesture to
        // attach an error to, and the full series is a truthful view.
        await expect(island.getByTestId("custom-range-status")).toHaveText("");
      }

      expect(consoleErrors, `uncaught page errors: ${consoleErrors.join(", ")}`).toEqual([]);
    },
  );

  test(
    "the custom-range inputs are reachable in order by keyboard and are not a trap",
    { tag: ["@chart-island", "@custom-range", "@a11y"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      // A native `<input type="date">` owns several INTERNAL focusable
      // segments (day / month / year) in Chromium, so Tab steps through the
      // field itself before leaving it. That is standard native behaviour,
      // not a trap — and it is exactly why this test counts BOUNDED presses
      // instead of assuming one Tab per control, which is what a naive
      // version of this assertion would have got wrong.
      await island.getByTestId("custom-range-from").focus();
      const visitedOrder: string[] = [];
      for (let press = 0; press < 15; press++) {
        await page.keyboard.press("Tab");
        const id = await page.evaluate(() => document.activeElement?.getAttribute("data-testid") ?? "");
        if (id && !visitedOrder.includes(id)) visitedOrder.push(id);
      }

      const trail = visitedOrder.join(" -> ");
      const toIndex = visitedOrder.indexOf("custom-range-to");
      const applyIndex = visitedOrder.indexOf("custom-range-apply");
      // Reading order, forwards only: "Desde" -> "Hasta" -> the commit control.
      expect(toIndex, `Tab never reached the "Hasta" field: ${trail}`).toBeGreaterThanOrEqual(0);
      expect(applyIndex, `the commit control is not after the "Hasta" field: ${trail}`).toBeGreaterThan(toIndex);
      // And focus genuinely LEAVES the group afterwards — the definition of
      // "not a trap" is that something outside it comes next.
      const afterGroup = visitedOrder
        .slice(applyIndex + 1)
        .filter((id) => !id.startsWith("custom-range-") && id !== "chart-custom-range");
      expect(afterGroup.length, `focus never left the custom-range group: ${trail}`).toBeGreaterThan(0);

      // Committing from the keyboard alone works (a real `<button>`, not a
      // click-only affordance)...
      await island.getByTestId("custom-range-from").fill("2020-01-01");
      await island.getByTestId("custom-range-to").fill("2021-12-31");
      await island.getByTestId("custom-range-apply").focus();
      await page.keyboard.press("Enter");
      await expect(page).toHaveURL(/range=custom/);

      // ...and Tab moves on out of the group rather than cycling back into it.
      await page.keyboard.press("Tab");
      const afterTab = await page.evaluate(() => document.activeElement?.getAttribute("data-testid") ?? "");
      expect(["custom-range-from", "custom-range-to", "custom-range-apply"]).not.toContain(afterTab);
    },
  );

  test(
    "a break band survives a range change (P4) — mutation-checked",
    { tag: ["@chart-island", "@p4"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      // The default ("full") view already shows the break.
      await expect(island.locator('[data-testid="chart-break-band"]')).toHaveCount(1);

      // Switching to the "5 años" preset narrows the visible window, but
      // this fixture's break (2022-Q2) is deliberately positioned to stay
      // inside BOTH available presets (see workbench/fixtures.ts's own
      // comment) — the band MUST still be present after the range change.
      await island.getByTestId("range-preset-5y").click();
      await expect(island.locator('[data-testid="chart-break-band"]')).toHaveCount(1);
      await expect(island.locator('[data-testid="break-band-tooltip"]')).toHaveCount(1);
    },
  );

  test(
    "activating year-on-year relabels the unit and states the derivation; the legend still distinguishes definitive/provisional by shape",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      await island.getByTestId("transform-toggle-yoy").click();
      await expect(island.getByTestId("transform-toggle-yoy")).toHaveAttribute("aria-pressed", "true");
      await expect(island.getByTestId("chart-derivation-note")).toBeVisible();
      await expect(island.getByTestId("chart-derivation-note")).toContainText("derivada de la serie original");

      const table = island.locator('[data-testid="accessible-data-table"]');
      await expect(table.locator("caption")).toContainText("% de variación");

      // Shape channel unaffected by the active transform: still circle vs
      // diamond, never colour alone (indicator-page spec §12.5).
      await expect(island.locator('[data-testid="chart-legend"]')).toBeVisible();
    },
  );

  test(
    "activating a transformation or range preset issues zero network requests — mutation-checked",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      // `waitForChartIslandHydrated()` flushes `client:idle`'s own
      // chunk-loading requests BEFORE the listener below attaches —
      // otherwise this test would flag the island's own hydration bundle
      // fetch, not a request caused by TOGGLING a transformation, which is
      // the actual contract under test (series-transformations spec).
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      const requests: string[] = [];
      page.on("request", (req) => requests.push(req.url()));

      await island.getByTestId("transform-toggle-yoy").click();
      await island.getByTestId("range-preset-5y").click();
      await island.getByTestId("transform-toggle-perCapita").click();

      expect(requests, `unexpected network requests: ${requests.join(", ")}`).toEqual([]);
    },
  );

  test(
    "the per-capita view discloses its covered span in Spanish",
    { tag: ["@chart-island"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();
      const island = workbench.chartIsland;

      await island.getByTestId("transform-toggle-perCapita").click();
      const disclosure = island.getByTestId("chart-percapita-disclosure");
      await expect(disclosure).toBeVisible();
      await expect(disclosure).toContainText("T1 2019");
      await expect(disclosure).toContainText("T4 2020");
    },
  );
});
