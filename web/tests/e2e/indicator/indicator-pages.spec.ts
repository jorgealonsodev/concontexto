import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { IndicatorPage } from "./indicator-page";

// Task 9a.13: "Scaffold Playwright + axe-core for these 3 pages as
// acceptance gates written alongside this slice (not red-first, per the
// stated Strict-TDD boundary): keyboard point navigation, 44 px targets,
// a javaScriptEnabled: false context." This file covers the JS-enabled
// gates (axe, keyboard navigation, 44px); the javaScriptEnabled: false
// context is its own file (`indicator-pages-no-js.spec.ts`), matching this
// project's established `chart-island.spec.ts` / `chart-no-js.spec.ts`
// split (a fixed context option applies per file/describe, not per test).
//
// Slice 9b (tasks.md 9b.4-9b.6) widens this to all six frozen slugs, now
// that ipc-general, ipc-subyacente and pib all have real built pages.
const SLUGS = [
  "tasa-de-paro-epa",
  "ocupados-epa",
  "poblacion-residente",
  "ipc-general",
  "ipc-subyacente",
  "pib",
];

// web-accessibility-gates spec, "WCAG 2.1 AA is verified by automated audit
// on every indicator page", Scenario "the audit runs over all six indicator
// pages in both themes" (task 9b.4). Production pages ship no runtime
// theme-toggle (the page always renders `data-theme="light"` server-side —
// `[slug].astro`), so the dark-theme pass sets `data-theme="dark"` directly
// on `<html>` after load, exactly re-stating every token the dark
// `[data-theme="dark"]` block already declares (design.md D-6), then
// re-runs the identical audit.
const THEMES = ["light", "dark"] as const;

for (const slug of SLUGS) {
  test.describe(`Indicator page /indicador/${slug}`, () => {
    for (const theme of THEMES) {
      test(
        `renders and has zero automatically-detectable accessibility violations (${theme} theme)`,
        { tag: ["@indicator-page", "@a11y"] },
        async ({ page }) => {
          const indicator = new IndicatorPage(page, slug);
          await indicator.goto();
          // Audit the HYDRATED page: the custom-range picker (two labelled
          // date inputs, a commit button and a polite live region) exists
          // only after hydration, so an audit that ran before it would
          // report zero violations over markup that was never there.
          await indicator.waitForChartHydrated();
          await indicator.customRangeApply.waitFor();
          if (theme === "dark") {
            await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
          }

          await expect(indicator.title).toBeVisible();
          await expect(indicator.chartSection).toBeVisible();
          await expect(indicator.methodologySheet).toBeVisible();
          await expect(indicator.actionBar).toBeVisible();
          await expect(indicator.relatedIndicators).toBeVisible();

          const results = await new AxeBuilder({ page }).analyze();
          expect(results.violations, `${slug} (${theme}): ${JSON.stringify(results.violations, null, 2)}`).toEqual([]);
        },
      );
    }

    test(
      "keyboard traversal reaches every chart point with a visible focus indicator, no keyboard trap",
      { tag: ["@indicator-page", "@a11y"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        const points = indicator.points;
        const count = await points.count();
        expect(count).toBeGreaterThan(0);

        await points.first().focus();
        const seenLabels = new Set<string>();
        seenLabels.add((await points.first().getAttribute("aria-label")) ?? "");

        for (let i = 1; i < count; i++) {
          const target = points.nth(i);
          const targetId = await target.getAttribute("id");
          await page.keyboard.press("ArrowRight");
          await expect(page.locator(`#${targetId}`)).toBeFocused();
          seenLabels.add((await target.getAttribute("aria-label")) ?? "");
          const outlineWidth = await target.evaluate((el) => getComputedStyle(el).outlineWidth);
          expect(outlineWidth).not.toBe("0px");
        }
        expect(seenLabels.size).toBe(count);

        // No trap: Tab leaves the chart's point group into the next
        // focusable control on the page.
        await page.keyboard.press("Tab");
        const afterTab = page.locator(":focus");
        await expect(afterTab).not.toHaveAttribute("data-testid", "chart-island-point");
      },
    );

    // verify-report WARNING-6. On the build the report inspected,
    // `grep -c 'data-testid="range-preset'` returned 0 on all six pages and
    // the header year-on-year figure rendered the not-available dash on all
    // six — not because the code was wrong, but because the export fixture
    // carried three observations per series. Both are now real, reachable
    // production behaviour, so both are gated here on the REAL BUILT page
    // rather than left to a container test alone.
    test(
      "all five range presets are offered, selecting one really narrows the chart, and the year-on-year figure is a real number",
      { tag: ["@indicator-page", "@range-preset"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        await expect(indicator.rangeControls).toBeVisible();
        for (const preset of ["full", "5y", "10y", "since-2008", "since-2018"]) {
          await expect(indicator.rangePreset(preset), `${slug}: missing the "${preset}" preset`).toBeVisible();
        }

        // The header figure the report found rendering "\u2014" on every page.
        // Punctuated the Spanish way \u2014 decimal COMMA, grouping point \u2014 because
        // every reader-facing numeral on this site now goes through
        // `lib/format/number.ts`. Spelled out rather than relaxed to `[.,]`,
        // so a regression back to locale-blind `toFixed` fails here too.
        await expect(indicator.yoyVariation).toHaveText(/^[+-]\d+(?:\.\d{3})*(?:,\d+)?%$/);

        // A preset that does not actually slice anything is a decorative
        // control: assert the rendered point count really drops.
        const fullCount = await indicator.points.count();
        expect(fullCount).toBeGreaterThan(3);
        await indicator.rangePreset("5y").click();
        await expect
          .poll(async () => indicator.points.count(), { message: `${slug}: the 5y preset did not narrow the chart` })
          .toBeLessThan(fullCount);
      },
    );

    // series-transformations spec, "Range presets" — the "personalizado"
    // entry (verify-report WARNING-5). Exercised on the REAL built page, over
    // the real full-length series (98-294 observations spanning 1995-2026
    // depending on the slug), not a synthetic short fixture: a custom range
    // is only a meaningful control when the series it narrows is long enough
    // for the narrowing to matter.
    //
    // 2010-01-01..2015-12-31 is inside every one of the six series' spans
    // (the earliest starts 1971-Q1, the latest 2002-01), so one window works
    // for all six regardless of frequency.
    test(
      "a custom range narrows the chart, encodes both bounds in the permalink and reloads to the same view",
      { tag: ["@indicator-page", "@custom-range"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        await expect(indicator.customRangeFrom).toBeVisible();
        const fullCount = await indicator.points.count();
        expect(fullCount).toBeGreaterThan(20);

        await indicator.customRangeFrom.fill("2010-01-01");
        await indicator.customRangeTo.fill("2015-12-31");
        await indicator.customRangeApply.click();

        await expect
          .poll(async () => indicator.points.count(), { message: `${slug}: the custom range did not narrow the chart` })
          .toBeLessThan(fullCount);
        const narrowedCount = await indicator.points.count();
        expect(narrowedCount, `${slug}: the custom range emptied the chart`).toBeGreaterThan(0);

        await expect(page).toHaveURL(/range=custom&from=2010-/);
        await expect(page).toHaveURL(/to=2015-/);

        await page.reload();
        const reloaded = new IndicatorPage(page, slug);
        await reloaded.waitForChartHydrated();
        await expect
          .poll(async () => reloaded.points.count(), { message: `${slug}: the custom permalink did not reproduce` })
          .toBe(narrowedCount);
        // The chart really is showing the requested window, not merely the
        // right NUMBER of points: the first and last rendered periods both
        // fall inside it.
        const periods = await reloaded.points.evaluateAll((nodes) =>
          nodes.map((n) => n.getAttribute("aria-label")?.split(":")[0] ?? ""),
        );
        expect(periods[0].startsWith("2010")).toBe(true);
        expect(periods[periods.length - 1].startsWith("2015")).toBe(true);
      },
    );

    test(
      "every interactive control measures at least 44x44 CSS px at a 375px viewport",
      { tag: ["@indicator-page", "@touch-target"] },
      async ({ page }) => {
        await page.setViewportSize({ width: 375, height: 900 });
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        // The custom-range picker is hydration-only markup (it cannot work
        // without JavaScript), so the sweep must wait for it or it would
        // pass by not looking at the two date inputs and the commit button.
        await indicator.waitForChartHydrated();
        await indicator.customRangeApply.waitFor();

        const controls = indicator.interactiveControls();
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

    // The chart is the centrepiece of every indicator page, and a 375 px
    // phone is where most readers meet it. Measured in a real browser at
    // that width before this change:
    //
    //   svg box 312.3 x 117.1 CSS px, scale 0.325, tick labels 3.25 CSS px
    //
    // The unit tests pin the geometry; only a browser can prove what the
    // geometry actually RENDERS at, because the answer depends on the page's
    // own column width, the scrollbar and the media query all agreeing.
    test(
      "the chart's axis labels are legible at a 375px viewport",
      { tag: ["@indicator-page", "@a11y", "@chart"] },
      async ({ page }) => {
        await page.setViewportSize({ width: 375, height: 900 });
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();

        // The narrow drawing is the one CSS shows at this width; the wide
        // one must be hidden, or both are in the accessibility tree and the
        // reader sees two charts.
        const narrow = indicator.chartSection.locator('[data-testid="indicator-chart-svg-narrow"]');
        const wide = indicator.chartSection.locator('[data-testid="indicator-chart-svg"]');
        await expect(narrow).toBeVisible();
        await expect(wide).toBeHidden();

        const measured = await narrow.evaluate((svg) => {
          const box = svg.getBoundingClientRect();
          const viewBox = (svg.getAttribute("viewBox") ?? "0 0 1 1").split(" ").map(Number);
          const scale = box.width / viewBox[2];
          const ticks = [...svg.querySelectorAll(".chart-tick")];
          return {
            height: box.height,
            renderedTickPx: ticks.map((t) => parseFloat(getComputedStyle(t).fontSize) * scale),
            // Anything drawn outside the viewBox is clipped by the SVG's own
            // overflow, so a label that starts left of 0 or ends past the
            // width is a label the reader cannot fully read.
            clipped: ticks
              .map((t) => {
                const b = (t as SVGGraphicsElement).getBBox();
                return { text: t.textContent ?? "", left: b.x, right: b.x + b.width };
              })
              .filter((t) => t.left < 0 || t.right > viewBox[2])
              .map((t) => t.text),
          };
        });

        // Small print, not a smudge. 10 CSS px is the floor; the measured
        // value here is about 11.
        const smallest = Math.min(...measured.renderedTickPx);
        expect(smallest, `smallest rendered tick label is ${smallest.toFixed(2)} CSS px`).toBeGreaterThanOrEqual(10);

        // ...and tall enough that the series' vertical movement is readable
        // rather than compressed into a band.
        expect(measured.height).toBeGreaterThan(180);

        // No axis label is cut off by the viewBox edge — the failure the
        // derived margins exist to prevent, and one the wide drawing still
        // exhibits for ten-glyph labels.
        expect(measured.clipped, `axis labels clipped by the viewBox: ${measured.clipped.join(", ")}`).toEqual([]);
      },
    );
  });
}
