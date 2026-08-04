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
          // date inputs, a commit button and a polite live region) and the
          // government range control (a labelled select and a second live
          // region) exist only after hydration, so an audit that ran before
          // them would report zero violations over markup that was never
          // there.
          await indicator.waitForChartHydrated();
          await indicator.customRangeApply.waitFor();
          await indicator.governmentSelect.waitFor();
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
        //
        // The YEAR is asserted at the END of the label, not at the start.
        // These announcements are reader-facing text and now carry the prose
        // period register ("T1 2010", "enero de 2010" — see
        // `src/lib/format/period.ts`), where the year comes last; the two
        // `toHaveURL` assertions above are what still hold the CANONICAL
        // "2010-Q1" bounds, in the one place that is parsed back.
        const periods = await reloaded.points.evaluateAll((nodes) =>
          nodes.map((n) => n.getAttribute("aria-label")?.split(":")[0] ?? ""),
        );
        expect(periods[0].endsWith("2010"), `${slug}: first rendered period is "${periods[0]}"`).toBe(true);
        const last = periods[periods.length - 1];
        expect(last.endsWith("2015"), `${slug}: last rendered period is "${last}"`).toBe(true);
      },
    );

    // The change-of-government markers, on the real built pages (PRD
    // §6.1.1(a)'s `governments` group, drawn ON the chart).
    //
    // Four things a narrower test would miss, and all four are ways the marker
    // could be a lie rather than an aid:
    //   1. it is not drawn until the reader asks for it — the `governments`
    //      group starts closed, and a layer that ignored its own toggle was
    //      exactly what this used to be;
    //   2. it is SOLID — the dashed stroke is the reserved provisional
    //      semantic and must not be borrowed here;
    //   3. both drawings carry it, so a phone reader is not shown a different
    //      chart from a desktop reader;
    //   4. the drawing and the generated sentence agree, because that sentence
    //      is the only route a screen-reader reader has to this layer (the
    //      drawing is one `role="img"`, which prunes its own children).
    test(
      "draws a solid change-of-government marker in both drawings once the group is opened, and the sentence names exactly what is drawn",
      { tag: ["@indicator-page", "@government-marker"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        // Nothing at all until the reader selects it.
        expect(await indicator.governmentMarkers.count(), `${slug}: a marker was drawn unasked`).toBe(0);
        await indicator.annotationToggle("governments").click();

        const wide = await indicator.governmentMarkers.count();
        const narrow = await indicator.governmentMarkersNarrow.count();
        // Every one of the six series spans 2018, so every one of them must
        // carry at least the sitting government's investiture. A page with no
        // marker at all would mean the layer silently did not ship.
        expect(wide, `${slug}: no change-of-government marker was drawn`).toBeGreaterThan(0);
        expect(narrow, `${slug}: the narrow drawing lost the marker the wide one has`).toBe(wide);

        // SOLID. `getAttribute` rather than a CSS check on purpose: the dash
        // would be a presentational attribute in the markup, exactly where the
        // provisional line puts its own.
        const dashes = await indicator.governmentMarkers
          .locator(".chart-government-marker__rule")
          .evaluateAll((nodes) => nodes.map((n) => n.getAttribute("stroke-dasharray")));
        expect(dashes.every((d) => d === null), `${slug}: a marker borrowed the provisional dash`).toBe(true);

        // The legend teaches the code, and the sentence names the events. The
        // sentence lives in its own polite live region rather than in the
        // chart's static description, because it now appears and disappears
        // under the reader's own hand.
        await expect(indicator.legendGovernment).toBeVisible();
        await expect(indicator.governmentNote).toContainText("cambios de gobierno registrados");

        // Agreement between the drawing and the words: one marker per name in
        // the sentence. The sentence lists "Nombre (AAAA)" items, so the years
        // in parentheses are countable.
        const text = (await indicator.governmentNote.textContent()) ?? "";
        const listed = text.match(/\(\d{4}\)/g) ?? [];
        expect(listed.length, `${slug}: the sentence names ${listed.length} changes but the chart draws ${wide}`).toBe(wide);

        // ...and every one of them says whose investiture it is, ON the
        // drawing. `indicator-annotation-labels.spec.ts` measures where.
        await expect(indicator.governmentLabels).toHaveCount(wide);
      },
    );

    // The range interaction, which is the question this layer had to answer
    // for itself: what should a marker do when the reader narrows the chart?
    //
    // The answer implemented is the break bands' own rule — a marker exists
    // while its date falls inside the window ON SCREEN — and this proves the
    // two consequences of it. Narrowing to one government's term can leave at
    // most that term's own opening boundary (attribution is half-open, so the
    // successor's investiture is outside the window); and it leaves NONE when
    // the investiture itself falls in a gap in the series' cadence just before
    // the first observation the term selects, because the change genuinely did
    // not happen inside the span that is drawn.
    test(
      "narrowing to one government leaves at most that term's own opening marker, and the sentence follows it",
      { tag: ["@indicator-page", "@government-marker"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        await indicator.governmentSelect.waitFor();
        // The markers are gated on the group toggle, so this range interaction
        // has to be asked for before it can be observed.
        await indicator.annotationToggle("governments").click();

        const fullMarkers = await indicator.governmentMarkers.count();
        const optionValues = await indicator.governmentSelect
          .locator("option")
          .evaluateAll((nodes) => nodes.map((n) => (n as HTMLOptionElement).value));
        const sitting = optionValues[optionValues.length - 1];
        await indicator.governmentSelect.selectOption(sitting);
        await expect(indicator.governmentStatus).toContainText("Gobierno de");

        const narrowedMarkers = await indicator.governmentMarkers.count();
        expect(narrowedMarkers, `${slug}: narrowing to one term did not reduce the markers`).toBeLessThanOrEqual(
          Math.min(1, fullMarkers),
        );

        // Whatever survived, the words and the drawing still agree — the
        // invariant that makes the sentence a description rather than a
        // caption written once and left behind.
        if (narrowedMarkers === 0) {
          await expect(indicator.governmentNote).not.toContainText("cambios de gobierno registrados");
          await expect(indicator.legendGovernment).toHaveCount(0);
        } else {
          await expect(indicator.governmentNote).toContainText("cambios de gobierno registrados");
          await expect(indicator.legendGovernment).toBeVisible();
          // ...and it sits on the term's own first observation, which is the
          // plot area's left edge — the same x the axis line starts at.
          const [markerX, axisX] = await indicator.chartSection.evaluate(() => {
            const svg = document.querySelector('[data-testid="indicator-chart-svg"]') as SVGSVGElement;
            const rule = svg.querySelector(".chart-government-marker__rule") as SVGLineElement;
            const axis = svg.querySelector(".chart-axis") as SVGLineElement;
            return [Number(rule.getAttribute("x1")), Number(axis.getAttribute("x1"))];
          });
          expect(markerX, `${slug}: the surviving marker is not on the term's first observation`).toBeCloseTo(axisX, 1);
        }
      },
    );

    // The government range (indicator-page spec, "Annotation layers per PRD
    // §6.1.1(a)" — the `governments` event group; series-transformations
    // spec, "The selected preset MUST be encoded in the permalink").
    //
    // This is the one range control whose bounds are NOT on the page: they
    // come from `config/gobiernos.yaml` via the artifact's `events`, and the
    // end of every closed term is INFERRED from the next investiture, since no
    // government in that registry carries an end date. So this test checks
    // three things a narrower one would miss: that the slice is real, that the
    // inference is DISCLOSED to the reader, and that the chart, the data table
    // and the generated description all describe the same slice.
    test(
      "selecting a government narrows the chart to its term, discloses the derived end, and the table and description agree",
      { tag: ["@indicator-page", "@government-range"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        await indicator.governmentSelect.waitFor();

        const fullCount = await indicator.points.count();
        const optionValues = await indicator.governmentSelect
          .locator("option")
          .evaluateAll((nodes) => nodes.map((n) => (n as HTMLOptionElement).value));
        // The neutral option plus at least one government. All six series
        // span 2018, so the sitting government is always among them.
        expect(optionValues[0], `${slug}: the first option must be the neutral one`).toBe("");
        expect(optionValues.length, `${slug}: no government terms offered`).toBeGreaterThan(1);

        // The LAST option is the sitting government — the one whose term is
        // open-ended, and therefore the one whose disclosure is the "no end
        // recorded" sentence rather than the "derived from the successor" one.
        const sitting = optionValues[optionValues.length - 1];
        await indicator.governmentSelect.selectOption(sitting);

        await expect
          .poll(async () => indicator.points.count(), { message: `${slug}: "${sitting}" did not narrow the chart` })
          .toBeLessThan(fullCount);
        const narrowedCount = await indicator.points.count();
        expect(narrowedCount, `${slug}: the government range emptied the chart`).toBeGreaterThan(0);

        // The ID travels in the permalink, never the president's name and
        // never the derived window — the parameter is parsed back.
        await expect(page).toHaveURL(new RegExp(`range=government&government=${sitting}`));
        await expect(page).not.toHaveURL(/from=/);

        // The disclosure. The registry records no end date for a sitting
        // government, and the page says so rather than implying the chart's
        // last observation is a term boundary.
        await expect(indicator.governmentStatus).toContainText("Gobierno de");
        await expect(indicator.governmentStatus).toContainText("no recoge la fecha de fin");

        // Agreement, in all three renderings of the same slice.
        const periods = await indicator.points.evaluateAll((nodes) =>
          nodes.map((n) => n.getAttribute("aria-label")?.split(":")[0] ?? ""),
        );
        const first = periods[0];
        const last = periods[periods.length - 1];
        const description = indicator.chartSection.getByTestId("chart-description");
        await expect(description).toContainText(first);
        await expect(description).toContainText(last);
        // The status line names the SAME rendered span the chart draws.
        await expect(indicator.governmentStatus).toContainText(first);
        await expect(indicator.governmentStatus).toContainText(last);
        // ...and the table holds one row per rendered observation. A row can
        // exist for a null value with no point button beside it, so the table
        // is allowed to be longer — never shorter, and never the full series.
        const rows = indicator.chartSection.locator('[data-testid="accessible-data-table"] tbody tr');
        const rowCount = await rows.count();
        expect(rowCount, `${slug}: the table did not follow the chart`).toBeGreaterThanOrEqual(narrowedCount);
        expect(rowCount, `${slug}: the table still shows the full series`).toBeLessThan(fullCount);

        // The permalink round-trip: the same view, the same selection.
        await page.reload();
        const reloaded = new IndicatorPage(page, slug);
        await reloaded.waitForChartHydrated();
        await reloaded.governmentSelect.waitFor();
        await expect
          .poll(async () => reloaded.points.count(), { message: `${slug}: the government permalink did not reproduce` })
          .toBe(narrowedCount);
        await expect(reloaded.governmentSelect).toHaveValue(sitting);
        await expect(reloaded.governmentStatus).toContainText("Gobierno de");
      },
    );

    // "Absent, not present-and-empty" — mutation-checked by exercising EVERY
    // option the control offers rather than by asserting the rule in the
    // abstract. Two ways to fail: an offered government that renders an empty
    // chart (the rule the fixed presets state), and an offered government that
    // renders the whole series (a duplicate of "Todo el periodo", which is the
    // REASON the presets give for absence). A term that fell outside this
    // series entirely would trip the first; the sitting government over a
    // three-observation series would trip the second.
    test(
      "every offered government genuinely narrows the chart, and no government that cannot is offered",
      { tag: ["@indicator-page", "@government-range"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        await indicator.governmentSelect.waitFor();

        const fullCount = await indicator.points.count();
        const values = (
          await indicator.governmentSelect
            .locator("option")
            .evaluateAll((nodes) => nodes.map((n) => (n as HTMLOptionElement).value))
        ).filter((value) => value !== "");

        const broken: string[] = [];
        for (const value of values) {
          await indicator.governmentSelect.selectOption(value);
          await expect(page).toHaveURL(new RegExp(`government=${value}`));
          const count = await indicator.points.count();
          if (count === 0) broken.push(`${value} (empty)`);
          if (count === fullCount) broken.push(`${value} (identical to the full series)`);
        }
        expect(broken, `${slug}: offered governments that cannot narrow anything: ${broken.join(", ")}`).toEqual([]);

        // And the neutral option really does restore the full series, so the
        // filter can be undone without reloading the page.
        await indicator.governmentSelect.selectOption("");
        await expect
          .poll(async () => indicator.points.count(), { message: `${slug}: clearing the filter did not restore the series` })
          .toBe(fullCount);
        await expect(page).not.toHaveURL(/government=/);
        await expect(indicator.governmentStatus).toHaveText("");
      },
    );

    test(
      "every interactive control measures at least 44x44 CSS px at a 375px viewport",
      { tag: ["@indicator-page", "@touch-target"] },
      async ({ page }) => {
        await page.setViewportSize({ width: 375, height: 900 });
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        // The custom-range picker and the government select are both
        // hydration-only markup (neither can work without JavaScript), so the
        // sweep must wait for them or it would pass by not looking at the two
        // date inputs, the commit button and the select.
        await indicator.waitForChartHydrated();
        await indicator.customRangeApply.waitFor();
        await indicator.governmentSelect.waitFor();

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
        // derived margins exist to prevent.
        expect(measured.clipped, `axis labels clipped by the viewBox: ${measured.clipped.join(", ")}`).toEqual([]);
      },
    );

    // The same gate on the WIDE drawing, which had exactly the same defect
    // and no test looking for it. Measured with `getBBox()` in Chromium at a
    // 1280 px viewport on the built /indicador/poblacion-residente, against
    // its `viewBox="0 0 960 360"`:
    //
    //   49.477.903  left  -5.46      49.630.061  left  -4.60
    //   49.553.982  left  -6.49      49.706.140  left  -3.61
    //   2026-Q2     right 965.50
    //
    // Five labels clipped on one published page; the last x tick clipped on
    // all six. A width ESTIMATE is what the fixed margins were implicitly
    // betting on and getting wrong, so this measures the real rendered box
    // rather than estimating it again — the unit tests pin the geometry, and
    // only a browser can say what that geometry actually draws in the
    // shipped typeface.
    test(
      "no axis label on the wide chart is clipped by the viewBox",
      { tag: ["@indicator-page", "@chart"] },
      async ({ page }) => {
        await page.setViewportSize({ width: 1280, height: 1000 });
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();

        const wide = indicator.chartSection.locator('[data-testid="indicator-chart-svg"]');
        const narrow = indicator.chartSection.locator('[data-testid="indicator-chart-svg-narrow"]');
        await expect(wide).toBeVisible();
        await expect(narrow).toBeHidden();

        const clipped = await wide.evaluate((svg) => {
          const viewBox = (svg.getAttribute("viewBox") ?? "0 0 1 1").split(" ").map(Number);
          return [...svg.querySelectorAll(".chart-tick")]
            .map((t) => {
              const b = (t as SVGGraphicsElement).getBBox();
              return { text: t.textContent ?? "", left: b.x, right: b.x + b.width };
            })
            // Both edges: a y label overruns the origin on the left, a
            // centred x tick overruns the width on the right, and the fixed
            // margins produced one of each on the same page.
            .filter((t) => t.left < 0 || t.right > viewBox[2])
            .map((t) => `${t.text} [${t.left.toFixed(2)}, ${t.right.toFixed(2)}]`);
        });

        expect(clipped, `axis labels clipped by the wide viewBox: ${clipped.join(", ")}`).toEqual([]);
      },
    );

    // The period a reader sees, proved on the REAL built page rather than in
    // a container render.
    //
    // THE DEFECT: `/indicador/tasa-de-paro-epa` printed `2026-Q2` twenty-two
    // times — the database's canonical storage format (migration 0001's own
    // comment), carrying `Q`, the English abbreviation for *quarter*. INE's
    // API returns `T3_Periodo` with "T1"–"T4" and INE writes "el segundo
    // trimestre de 2020": every figure here comes from a source that says T.
    //
    // This walks TEXT NODES, not markup, so it says something a `toContain`
    // over the HTML cannot: no reader-facing position — header, table cell,
    // axis tick, generated prose, break band, tooltip label — still shows the
    // storage form, while attributes (`datetime`, `?from=`, `href`) are
    // deliberately out of scope and are asserted to keep it.
    test(
      "no period reaches the reader in the database's storage format, and the machine values keep it",
      { tag: ["@indicator-page", "@i18n"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        const leaked = await page.evaluate(() => {
          const STORAGE_FORMAT = /\b\d{4}-(Q[1-4]|0[1-9]|1[0-2])\b/;
          const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
          const found: string[] = [];
          for (let node = walker.nextNode(); node; node = walker.nextNode()) {
            const text = (node.textContent ?? "").trim();
            if (text && STORAGE_FORMAT.test(text)) found.push(text.slice(0, 60));
          }
          // SVG tick labels are text nodes too and are already covered by the
          // walk above; the accessible names of the point buttons are not, so
          // they are swept explicitly — that string is what a screen-reader
          // reader actually receives.
          for (const button of document.querySelectorAll('[data-testid="chart-island-point"]')) {
            const label = button.getAttribute("aria-label") ?? "";
            if (STORAGE_FORMAT.test(label)) found.push(label.slice(0, 60));
          }
          return found;
        });
        expect(leaked, `${slug}: storage-format periods still visible: ${leaked.slice(0, 5).join(" | ")}`).toEqual([]);

        // Quarterly pages say T, monthly pages say a Spanish month name —
        // the positive half, so this test cannot pass by rendering nothing.
        const tickText = await page
          .locator('[data-testid="indicator-chart-svg"] .chart-tick--x')
          .allTextContents();
        expect(tickText.length).toBeGreaterThan(0);
        for (const label of tickText) {
          expect(label, `${slug}: unexpected x tick "${label}"`).toMatch(
            /^(T[1-4] \d{4}|[a-záéíóú]{3,10} \d{4}|\d{4})$/,
          );
        }

        // And the machine surfaces, unmoved. `<time datetime>` exists exactly
        // so the prose beside it can be reformatted without the machine value
        // being lost, and the export links must still name real files.
        const datetimes = await page.locator("time[datetime]").evaluateAll((nodes) =>
          nodes.map((n) => n.getAttribute("datetime") ?? ""),
        );
        expect(datetimes.length).toBeGreaterThan(0);
        for (const value of datetimes) {
          expect(value, `${slug}: <time datetime> is no longer machine-readable`).toMatch(
            /^\d{4}-\d{2}-\d{2}(T[\d:.]+Z?)?$/,
          );
        }
      },
    );

    // `FreshnessSemaphore` is a non-interactive `<span>` styled as a pill
    // (`inline-flex … rounded-pill border px-2.5 py-1`). Every container it
    // is dropped into here is a flex COLUMN — the page header
    // (`flex flex-col`) and `IndicatorCard`'s root — where the default
    // `align-items: stretch` widened it to the container's full width.
    // Measured on the built page at 1280 px before this change: the header
    // badge rendered 848.0 x 26.0 CSS px inside an 848.0 px header, i.e.
    // 100% of it, and every related card's badge rendered 238.0 px inside a
    // 238.0 px content box. A status pill drawn as a full-width bordered
    // box reads as a button and invites a click that does nothing.
    //
    // The same 70% share the homepage gate uses, for the same reason
    // (`tests/e2e/home/home.spec.ts`): 100% fails it by a mile, and a pill
    // sized to its own text has far more headroom than 30%.
    const MAX_BADGE_SHARE = 0.7;

    test(
      "the freshness badge is sized to its own text in the header and in every related card",
      { tag: ["@indicator-page", "@layout"] },
      async ({ page }) => {
        await page.setViewportSize({ width: 1280, height: 1000 });
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();

        const headerBox = await indicator.header.boundingBox();
        const headerBadgeBox = await indicator.headerFreshness.boundingBox();
        if (!headerBox || !headerBadgeBox) throw new Error("the page header or its freshness badge has no rendered box");
        const headerShare = headerBadgeBox.width / headerBox.width;
        expect(
          headerShare,
          `header badge is ${headerBadgeBox.width.toFixed(1)} px of an ${headerBox.width.toFixed(1)} px header (${(headerShare * 100).toFixed(1)}%)`,
        ).toBeLessThanOrEqual(MAX_BADGE_SHARE);

        const cards = indicator.relatedCards;
        const count = await cards.count();
        expect(count, "the related-indicators strip must render cards to measure").toBeGreaterThan(0);

        const overwide: string[] = [];
        for (let i = 0; i < count; i++) {
          const card = cards.nth(i);
          const badgeBox = await indicator.badgeIn(card).boundingBox();
          if (!badgeBox) throw new Error(`related card ${i}'s freshness badge has no rendered box`);
          const contentWidth = await card.evaluate((el) => {
            const style = getComputedStyle(el);
            return el.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
          });
          const share = badgeBox.width / contentWidth;
          if (share > MAX_BADGE_SHARE) {
            overwide.push(
              `${await card.getAttribute("href")} (badge ${badgeBox.width.toFixed(1)} px of ${contentWidth.toFixed(1)} px content = ${(share * 100).toFixed(1)}%)`,
            );
          }
        }
        expect(
          overwide,
          `related-card badges wider than ${MAX_BADGE_SHARE * 100}% of their card's content width: ${overwide.join(", ")}`,
        ).toEqual([]);
      },
    );

    // The related strip is `grid … lg:grid-cols-3` of cards that are
    // themselves the grid items, so they were already stretched to their
    // row's height — but their badges were not pushed to the bottom, so
    // they zigzagged. Measured at 1280 px before this change, the three
    // cards of the first row on `/indicador/ipc-general` carried badge
    // bottoms 88 px apart inside rows of identical height.
    test(
      "related cards in one row render at one height, and their freshness badges share one bottom edge",
      { tag: ["@indicator-page", "@layout"] },
      async ({ page }) => {
        await page.setViewportSize({ width: 1280, height: 1000 });
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        // The chart island reflows the page as it hydrates, and every box
        // below is read from the live layout — measuring before it settles
        // reads coordinates that no longer exist a frame later.
        await indicator.waitForChartHydrated();

        const cards = indicator.relatedCards;
        const count = await cards.count();
        const measured: { href: string; top: number; height: number; badgeBottom: number }[] = [];
        for (let i = 0; i < count; i++) {
          const card = cards.nth(i);
          const box = await card.boundingBox();
          const badgeBox = await indicator.badgeIn(card).boundingBox();
          if (!box || !badgeBox) throw new Error(`related card ${i} or its badge has no rendered box`);
          measured.push({
            href: (await card.getAttribute("href")) ?? `card ${i}`,
            top: box.y,
            height: box.height,
            badgeBottom: badgeBox.y + badgeBox.height,
          });
        }

        const rows = new Map<number, typeof measured>();
        for (const card of measured) rows.set(Math.round(card.top), [...(rows.get(Math.round(card.top)) ?? []), card]);
        const multiCardRows = [...rows.values()].filter((row) => row.length > 1);
        expect(multiCardRows.length, "1280px must place related cards beside each other").toBeGreaterThan(0);

        for (const row of multiCardRows) {
          const heights = row.map((c) => c.height);
          expect(
            Math.max(...heights) - Math.min(...heights),
            `related cards in the row at y=${row[0].top.toFixed(1)} render at different heights: ${row.map((c) => `${c.href}=${c.height.toFixed(1)}`).join(", ")}`,
          ).toBeLessThanOrEqual(1);

          const bottoms = row.map((c) => c.badgeBottom);
          expect(
            Math.max(...bottoms) - Math.min(...bottoms),
            `related-card badges in the row at y=${row[0].top.toFixed(1)} do not share a bottom edge: ${row.map((c) => `${c.href}=${c.badgeBottom.toFixed(1)}`).join(", ")}`,
          ).toBeLessThanOrEqual(1);
        }
      },
    );
  });
}
