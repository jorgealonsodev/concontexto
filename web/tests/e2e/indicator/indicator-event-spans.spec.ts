import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { IndicatorPage } from "./indicator-page";

// The editorial event SPAN, proven in a real browser against real full-history
// data — the chart's fourth annotation treatment and the only one about an
// interval.
//
// WHY THIS PAGE. /indicador/tasa-de-paro-epa runs 2002-Q1..2026-Q2 and its
// `exogenous` group is the richest shape the registry produces:
//
//   crisis-financiera-2008-2013   2008-01-01 -> 2013-12-31   a span, inside
//   pandemia-2020-2021            2020-03-14 -> 2021-05-09   a span, inside
//   ngeu-primer-desembolso        2021-08-01 -> (none)       no end recorded
//   shock-energetico-2022         2022-02-24 -> (none)       no end recorded
//
// Four chips, two rails. That difference is the whole design decision about
// open-ended events made visible, and the sentence below the chart is what
// accounts for it.
//
// The pandemic rail is the honest end-to-end proof: this series has its
// visible COVID spike at 2020-Q2, so a rail that landed anywhere else would be
// obvious on screen and is measured here rather than eyeballed.
const SLUG = "tasa-de-paro-epa";

test.describe("Indicator page — editorial event spans", () => {
  test(
    "showing the exogenous group projects a rail per bounded event, in both drawings",
    { tag: ["@indicator-page", "@event-span"] },
    async ({ page }) => {
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();

      // The group is OFF by default (indicator-page spec, "two off by
      // default"), so nothing is projected until the reader asks.
      await expect(indicator.eventSpans).toHaveCount(0);
      await expect(indicator.legendEventSpan).toHaveCount(0);

      await indicator.annotationToggle("exogenous").click();

      // Two rails, not four: the two entries carrying no `date_end` project
      // nothing, because an event with no recorded end has no period to cover.
      await expect(indicator.eventSpans).toHaveCount(2);
      await expect(indicator.eventSpansNarrow).toHaveCount(2);
      // One per drawing, wide and narrow — a second variant is a second chance
      // to lose a layer.
      // Scoped to the RAIL GROUPS by their exact test ids, not to every node
      // carrying the event's id: the rail's own on-drawing label carries that
      // id too, so an unscoped locator would count each mark and its name as
      // two marks and quietly turn this "one per drawing" claim into four.
      await expect(
        indicator.chartSection.locator(
          '[data-testid="chart-event-span"][data-event-id="pandemia-2020-2021"], ' +
            '[data-testid="chart-event-span-narrow"][data-event-id="pandemia-2020-2021"]',
        ),
      ).toHaveCount(2);
      await expect(
        indicator.chartSection.locator('[data-event-id="shock-energetico-2022"]'),
      ).toHaveCount(0);

      // The legend appears with the mark it explains, and disappears with it.
      await expect(indicator.legendEventSpan).toBeVisible();
      await indicator.annotationToggle("exogenous").click();
      await expect(indicator.eventSpans).toHaveCount(0);
      await expect(indicator.legendEventSpan).toHaveCount(0);
    },
  );

  test(
    "the pandemic rail lands on 2020, over the series' own COVID spike",
    { tag: ["@indicator-page", "@event-span"] },
    async ({ page }) => {
      // The claim this whole layer makes is POSITIONAL, so it is measured
      // positionally, in the drawing's own user-space units.
      //
      // The comparison is against the interactive point buttons rather than
      // against a computed constant: each is placed at `left: (x / width)%` of
      // the figure by the island itself, so multiplying that percentage back by
      // the viewBox width recovers the exact x the drawing projected the
      // observation to. If the rail and the point disagree, the rail is
      // pointing at a quarter the reader is not looking at.
      //
      // Deliberately NOT `boundingBox()` on both: the SVG is laid out by
      // `preserveAspectRatio="xMidYMid meet"` while the buttons are laid out in
      // percentages of the figure box, so a sub-pixel rounding of the figure's
      // height letterboxes one against the other by a couple of CSS pixels. The
      // rendered agreement is still checked below, at a tolerance that reflects
      // what that rounding can actually cost.
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await indicator.annotationToggle("exogenous").click();
      // The government rules are gated on their own group, and this test
      // compares the rail against one of them, so both selections are made.
      await indicator.annotationToggle("governments").click();

      const rail = indicator.chartSection.locator(
        '[data-testid="chart-event-span"][data-event-id="pandemia-2020-2021"] .chart-event-span__rail',
      );
      await expect(rail).toHaveCount(1);

      const measured = await page.evaluate(() => {
        const svg = document.querySelector('[data-testid="indicator-chart-svg"]')!;
        const viewBoxWidth = Number(svg.getAttribute("viewBox")!.split(" ")[2]);
        const d = svg
          .querySelector('[data-event-id="pandemia-2020-2021"] .chart-event-span__rail')!
          .getAttribute("d")!;
        const xs = [...d.matchAll(/[ML]([\d.]+),/g)].map((m) => Number(m[1]));
        const pointX = (label: string) => {
          const el = document.querySelector<HTMLElement>(
            `[data-testid="chart-island-point"][aria-label^="${label}:"]`,
          )!;
          return (parseFloat(el.style.left) / 100) * viewBoxWidth;
        };
        return { railStart: Math.min(...xs), railEnd: Math.max(...xs), start: pointX("T1 2020"), end: pointX("T2 2021") };
      });

      // Two decimals, because that is the precision `renderChartSVG` serialises
      // its coordinates at.
      expect(measured.railStart).toBeCloseTo(measured.start, 2);
      expect(measured.railEnd).toBeCloseTo(measured.end, 2);

      // ...and what is actually PAINTED agrees with the point the reader can
      // hover, to within the letterboxing described above.
      const railBox = (await rail.boundingBox())!;
      const startBox = (await indicator.pointFor("T1 2020").boundingBox())!;
      expect(Math.abs(railBox.x - (startBox.x + startBox.width / 2))).toBeLessThan(5);

      // ...and it is a RAIL rather than a second kind of vertical rule. The
      // change-of-government marker crosses this same plot at 2018, so the two
      // are on screen together and the reader has to tell them apart before
      // reading either: the marker spans the FULL plot height, the rail drops a
      // serif a small fraction of it.
      //
      // Measured as a proportion rather than in pixels, because both drawings
      // are scaled viewBoxes and the absolute numbers move with the viewport.
      // A width ratio is deliberately NOT asserted: a one-period span is
      // legitimately almost as wide as it is tall, and that is the axis'
      // resolution rather than a defect.
      const markerBox = (await indicator.governmentMarkers.first().boundingBox())!;
      const plotBox = (await indicator.chartSection
        .locator('[data-testid="indicator-chart-svg"]')
        .boundingBox())!;
      expect(markerBox.height / plotBox.height).toBeGreaterThan(0.8);
      expect(railBox.height / plotBox.height).toBeLessThan(0.15);
    },
  );

  test(
    "the sentence names what is drawn and re-narrates when the selection changes",
    { tag: ["@indicator-page", "@a11y", "@event-span"] },
    async ({ page }) => {
      // The rails live inside a single `role="img"`, which prunes its own
      // descendants — so this sentence is the only route a screen-reader
      // reader has to them, and it has to change under the reader's own hand.
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();

      // The live region exists BEFORE the first toggle (a region created at
      // the moment its content appears is routinely missed), and is empty.
      await expect(indicator.eventSpansNote).toHaveAttribute("aria-live", "polite");
      await expect(indicator.eventSpansNote).toHaveText("");

      await indicator.annotationToggle("exogenous").click();
      await expect(indicator.eventSpansNote).toContainText("Pandemia de COVID-19");
      await expect(indicator.eventSpansNote).toContainText("T1 2020");
      await expect(indicator.eventSpansNote).toContainText("T2 2021");
      // The clause that accounts for the two chips carrying no rail, so the
      // sentence never reads as a complete inventory of the group.
      await expect(indicator.eventSpansNote).toContainText("fecha de inicio y de fin");
      // ...and it never names an event it did not draw.
      await expect(indicator.eventSpansNote).not.toContainText("Shock energético");

      await indicator.annotationToggle("exogenous").click();
      await expect(indicator.eventSpansNote).toHaveText("");
    },
  );

  for (const theme of ["light", "dark"] as const) {
    test(
      `the projected state has zero automatically-detectable accessibility violations (${theme} theme)`,
      { tag: ["@indicator-page", "@a11y", "@event-span"] },
      async ({ page }) => {
        // `indicator-pages.spec.ts` audits the page as it LOADS, with every
        // annotation group in its default state — so the rails, the fourth
        // legend entry and the populated live region are markup that gate has
        // never seen. This is that gate re-run over the state a reader reaches
        // by using the control.
        const indicator = new IndicatorPage(page, SLUG);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        await indicator.annotationToggle("exogenous").click();
        await expect(indicator.eventSpans).toHaveCount(2);
        if (theme === "dark") {
          await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
        }
        // Same builder configuration as `indicator-pages.spec.ts`'s own audit,
        // deliberately — a second, differently-tuned audit would be a second
        // standard rather than the same gate over more of the page.
        const results = await new AxeBuilder({ page }).analyze();
        expect(results.violations).toEqual([]);
      },
    );
  }

  test(
    "narrowing the range re-clamps the rail instead of leaving it claiming a boundary it lost",
    { tag: ["@indicator-page", "@event-span"] },
    async ({ page }) => {
      // "Desde 2018" cuts the 2008-2013 crisis out entirely and leaves the
      // pandemic wholly inside — the two halves of the window rule in one
      // gesture, on real data.
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await indicator.annotationToggle("exogenous").click();
      await expect(indicator.eventSpans).toHaveCount(2);

      await indicator.rangePreset("since-2018").click();
      await expect(indicator.eventSpans).toHaveCount(1);
      await expect(
        indicator.chartSection.locator('[data-event-id="crisis-financiera-2008-2013"]'),
      ).toHaveCount(0);
      await expect(indicator.eventSpansNote).not.toContainText("Crisis financiera");
      await expect(indicator.eventSpansNote).toContainText("Pandemia de COVID-19");
    },
  );

  test(
    "a rail that outruns the window is uncapped, and says so in words too",
    { tag: ["@indicator-page", "@event-span"] },
    async ({ page }) => {
      // "5 años" starts inside the 2020-2021 pandemic on a series ending in
      // 2026, so the pandemic's own start falls outside the window. The rail
      // must not put a serif at the plot's left edge — that would turn the
      // edge of the chart into a claim about the calendar — and the sentence
      // has to make the same distinction for a reader who has no serif to see.
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await indicator.annotationToggle("exogenous").click();
      await indicator.rangePreset("5y").click();

      const rail = indicator.chartSection.locator(
        '[data-testid="chart-event-span"][data-event-id="pandemia-2020-2021"] .chart-event-span__rail',
      );
      await expect(rail).toHaveCount(1);
      // Three vertices, not four: one serif was dropped because that end is
      // the window's, not the event's.
      const d = (await rail.getAttribute("d")) ?? "";
      expect((d.match(/[ML]/g) ?? []).length).toBe(3);

      await expect(indicator.eventSpansNote).toContainText("visible");
      await expect(indicator.eventSpansNote).toContainText("fuera del periodo representado");
    },
  );
});
