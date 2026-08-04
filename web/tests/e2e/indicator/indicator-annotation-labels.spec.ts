import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { IndicatorPage } from "./indicator-page";

// The on-drawing annotation LABELS, and the selection gate that decides whether
// anything is drawn at all — both proven in a real browser, on real built
// pages, with real editorial data.
//
// WHY A BROWSER TEST AND NOT ONLY THE UNIT ONE. `lib/chart/annotationLabels.ts`
// places every label from an ESTIMATE of how wide its text will be — there is
// no text engine in Vitest, and the same conservative glyph ratio the chart's
// own margins are derived from is what it uses. An estimate that stopped being
// conservative would produce a chart whose labels overlap or hang outside the
// viewBox while every unit test still passed. This file is where the real
// `getBBox()` is measured, exactly as `indicator-pages.spec.ts` already does
// for the axis ticks — the browser is the only place the claim can be checked.
//
// WHY THESE TWO PAGES. /indicador/poblacion-residente carries six changes of
// government, two of them closer together than any other pair on the site
// (Calvo-Sotelo, February 1981, and González, December 1982) — the pair that
// made horizontal names impossible and the one every collision claim is really
// about. /indicador/tasa-de-paro-epa is the only page where the two labelled
// layers meet: Rajoy's rule stands inside the 2008-2013 crisis rail, so a
// government name and an event name compete for the same corner of the plot.
const DENSE_SLUG = "poblacion-residente";
const MIXED_SLUG = "tasa-de-paro-epa";

/** The two drawings, each with the viewport at which it is the one on screen.
 *
 * THE RESIZE IS LOAD-BEARING, and finding out why cost a red test: both
 * variants are in the document at once and CSS shows one, and `getBBox()`
 * inside a `display: none` subtree reports zeros in Chromium. Measuring the
 * narrow drawing from a desktop viewport therefore reads every label as a
 * zero-sized box at the origin — which is not a measurement of anything. Each
 * variant is measured at the width a reader really sees it at. */
const VARIANTS = [
  {
    name: "wide",
    viewport: { width: 1280, height: 900 },
    svgId: "indicator-chart-svg",
    governmentLabelId: "chart-government-label",
    eventLabelId: "chart-event-span-label",
  },
  {
    name: "narrow",
    viewport: { width: 375, height: 900 },
    svgId: "indicator-chart-svg-narrow",
    governmentLabelId: "chart-government-label-narrow",
    eventLabelId: "chart-event-span-label-narrow",
  },
] as const;

/** Every label's real rendered box in the drawing's own user-space units, read
 * from the browser rather than computed.
 *
 * `getBBox()` and not `getBoundingClientRect()`, deliberately: the SVG is laid
 * out by `preserveAspectRatio="xMidYMid meet"`, so client rectangles carry the
 * viewport's own scaling and letterboxing, while the question being asked here
 * — does this label overlap that one, does it escape the box — is a question
 * about the drawing's own coordinates. A rotated label's `getBBox()` is
 * reported in its own local system, before its `transform`, so the width and
 * height are swapped back here to give the box as it is actually painted. */
async function measureLabels(page: import("@playwright/test").Page, testId: string) {
  return page.evaluate((id) => {
    return [...document.querySelectorAll<SVGTextElement>(`[data-testid="${id}"]`)].map((node) => {
      const box = node.getBBox();
      const rotated = (node.getAttribute("transform") ?? "").includes("rotate(-90");
      const anchorX = Number(node.getAttribute("x"));
      const anchorY = Number(node.getAttribute("y"));
      return {
        text: node.textContent ?? "",
        // Under `rotate(-90 anchorX anchorY)` a local point (x, y) is painted
        // at (anchorX + (y - anchorY), anchorY - (x - anchorX)).
        x: rotated ? anchorX + (box.y - anchorY) : box.x,
        y: rotated ? anchorY - (box.x - anchorX) - box.width : box.y,
        width: rotated ? box.height : box.width,
        height: rotated ? box.width : box.height,
      };
    });
  }, testId);
}

interface MeasuredLabel {
  text: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

function overlappingPairs(labels: MeasuredLabel[]): string[] {
  const clashes: string[] = [];
  for (let i = 0; i < labels.length; i++) {
    for (let j = i + 1; j < labels.length; j++) {
      const a = labels[i];
      const b = labels[j];
      if (a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height) {
        clashes.push(`"${a.text}" × "${b.text}"`);
      }
    }
  }
  return clashes;
}

/** The viewBox of one drawing, as its own four numbers. */
async function viewBoxOf(page: import("@playwright/test").Page, testId: string) {
  return page.evaluate((id) => {
    const [, , width, height] = document
      .querySelector(`[data-testid="${id}"]`)!
      .getAttribute("viewBox")!
      .split(" ")
      .map(Number);
    return { width, height };
  }, testId);
}

test.describe("Indicator page — the marks carry their own names", () => {
  test(
    "no government marker is drawn until the reader selects the group, and every one is named once it is",
    { tag: ["@indicator-page", "@government-marker", "@annotation-label"] },
    async ({ page }) => {
      // The change this proves, in one gesture. The marker layer used to be
      // drawn on every chart unconditionally — the only annotation layer whose
      // visible toggle the drawing ignored. Now it answers that toggle, and
      // every rule it draws says whose investiture it is.
      const indicator = new IndicatorPage(page, DENSE_SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();

      await expect(indicator.governmentMarkers).toHaveCount(0);
      await expect(indicator.governmentMarkersNarrow).toHaveCount(0);
      await expect(indicator.governmentLabels).toHaveCount(0);
      await expect(indicator.legendGovernment).toHaveCount(0);
      await expect(indicator.governmentNote).toHaveText("");

      await indicator.annotationToggle("governments").click();

      const markers = await indicator.governmentMarkers.count();
      expect(markers, "no marker was drawn even after the group was opened").toBeGreaterThan(0);
      // One label per mark, in BOTH drawings: a second variant is a second
      // chance to lose a layer, and this is the layer whose type size differs
      // between the two boxes.
      await expect(indicator.governmentLabels).toHaveCount(markers);
      await expect(indicator.governmentMarkersNarrow).toHaveCount(markers);
      await expect(indicator.governmentLabelsNarrow).toHaveCount(markers);
      await expect(indicator.legendGovernment).toBeVisible();

      // ...and the name on the drawing is the registry's own, whole.
      const labels = await measureLabels(page, "chart-government-label");
      expect(labels.map((l) => l.text)).toContain("José Luis Rodríguez Zapatero");
      for (const label of labels) {
        expect(label.text, `"${label.text}" was cut short on the drawing`).not.toContain("…");
      }

      // ...and closing the group takes all of it away again.
      await indicator.annotationToggle("governments").click();
      await expect(indicator.governmentMarkers).toHaveCount(0);
      await expect(indicator.governmentLabels).toHaveCount(0);
      await expect(indicator.legendGovernment).toHaveCount(0);
    },
  );

  for (const variant of VARIANTS) {
    test(
      `six government names fit the ${variant.name} drawing without one touching another or leaving the box`,
      { tag: ["@indicator-page", "@government-marker", "@annotation-label"] },
      async ({ page }) => {
        // The worst case on the site, measured rather than argued. Two of these
        // six investitures are twenty-one months apart, which is a handful of
        // user units at this axis' resolution; turning the names onto their
        // side is what makes them fit, and stacking the pair that still cannot
        // is what makes the last of them fit.
        await page.setViewportSize(variant.viewport);
        const indicator = new IndicatorPage(page, DENSE_SLUG);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        await indicator.annotationToggle("governments").click();
        await expect(indicator.governmentLabels.first()).toBeAttached();

        const labels = await measureLabels(page, variant.governmentLabelId);
        expect(labels.length, `${variant.name}: no label was measured`).toBeGreaterThanOrEqual(6);
        // Not vacuous: a hidden drawing reports every box as zero-sized, which
        // would pass every overlap and containment check below.
        for (const label of labels) {
          expect(label.width, `${variant.name}: "${label.text}" measured as nothing`).toBeGreaterThan(0);
          expect(label.height).toBeGreaterThan(0);
        }

        const clashes = overlappingPairs(labels);
        expect(clashes, `${variant.name}: labels drawn through each other: ${clashes.join(", ")}`).toEqual([]);

        const { width, height } = await viewBoxOf(page, variant.svgId);
        const escaped = labels.filter(
          (l) => l.x < 0 || l.y < 0 || l.x + l.width > width || l.y + l.height > height,
        );
        expect(
          escaped.map((l) => l.text),
          `${variant.name}: clipped by the viewBox: ${escaped.map((l) => l.text).join(", ")}`,
        ).toEqual([]);
      },
    );
  }

  test(
    "a government name and an event name never share the same ink, on the one page where they compete",
    { tag: ["@indicator-page", "@event-span", "@annotation-label"] },
    async ({ page }) => {
      // Rajoy's 2011 rule stands inside the 2008-2013 crisis rail on this
      // series, so the two labelled layers are drawn into the same corner of
      // the same plot. Both variants, because the narrow box has a third of the
      // horizontal room and its own smaller type.
      for (const variant of VARIANTS) {
        await page.setViewportSize(variant.viewport);
        const indicator = new IndicatorPage(page, MIXED_SLUG);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        await indicator.annotationToggle("governments").click();
        await indicator.annotationToggle("exogenous").click();
        await expect(indicator.eventSpans).toHaveCount(2);

        const labels = [
          ...(await measureLabels(page, variant.governmentLabelId)),
          ...(await measureLabels(page, variant.eventLabelId)),
        ];
        expect(labels.length, `${variant.name}: no label was measured`).toBeGreaterThan(0);
        for (const label of labels) {
          expect(label.width, `${variant.name}: "${label.text}" measured as nothing`).toBeGreaterThan(0);
        }

        const clashes = overlappingPairs(labels);
        expect(clashes, `${variant.name}: labels drawn through each other: ${clashes.join(", ")}`).toEqual([]);

        const { width, height } = await viewBoxOf(page, variant.svgId);
        const escaped = labels.filter(
          (l) => l.x < 0 || l.y < 0 || l.x + l.width > width || l.y + l.height > height,
        );
        expect(
          escaped.map((l) => l.text),
          `${variant.name}: clipped by the viewBox: ${escaped.map((l) => l.text).join(", ")}`,
        ).toEqual([]);
      }
    },
  );

  test(
    "an event name too long to be drawn whole is withheld, never cut — and the sentence still carries it",
    { tag: ["@indicator-page", "@event-span", "@annotation-label"] },
    async ({ page }) => {
      // "Crisis financiera global y crisis de deuda soberana europea" is 58
      // glyphs. It fits the wide drawing beside its own rail and cannot fit the
      // narrow one at any size this chart sets type in. The honest answer is to
      // draw nothing there rather than an ellipsis that renames the event —
      // and the sentence under the chart, which names every projected event in
      // full, is what keeps that omission disclosed rather than silent.
      const indicator = new IndicatorPage(page, MIXED_SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await indicator.annotationToggle("exogenous").click();
      await expect(indicator.eventSpans).toHaveCount(2);

      const wide = (await measureLabels(page, "chart-event-span-label")).map((l) => l.text);
      expect(wide).toContain("Crisis financiera global y crisis de deuda soberana europea");

      const narrow = (await measureLabels(page, "chart-event-span-label-narrow")).map((l) => l.text);
      expect(narrow).not.toContain("Crisis financiera global y crisis de deuda soberana europea");
      // Whatever the narrow drawing did print, it printed whole.
      for (const text of [...wide, ...narrow]) {
        expect(text, `"${text}" was cut short on the drawing`).not.toContain("…");
      }

      await expect(indicator.eventSpansNote).toContainText(
        "Crisis financiera global y crisis de deuda soberana europea",
      );
    },
  );

  test(
    "the naming sentence is a polite live region that re-narrates as the reader selects",
    { tag: ["@indicator-page", "@a11y", "@government-marker", "@annotation-label"] },
    async ({ page }) => {
      // The labels are painted inside a single `role="img"`, which prunes its
      // own descendants from the accessibility tree — so a screen-reader reader
      // reaches not one word of them. This sentence is their only channel, and
      // because the layer now appears and disappears under the reader's own
      // hand it has to re-narrate rather than change silently.
      const indicator = new IndicatorPage(page, DENSE_SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();

      await expect(indicator.governmentNote).toHaveAttribute("aria-live", "polite");
      await expect(indicator.governmentNote).toHaveText("");

      await indicator.annotationToggle("governments").click();
      await expect(indicator.governmentNote).toContainText("cambios de gobierno registrados");
      await expect(indicator.governmentNote).toContainText("José Luis Rodríguez Zapatero (2004)");

      // ...and the words and the drawing agree: one marker per name listed.
      const listed = ((await indicator.governmentNote.textContent()) ?? "").match(/\(\d{4}\)/g) ?? [];
      expect(listed.length).toBe(await indicator.governmentMarkers.count());

      await indicator.annotationToggle("governments").click();
      await expect(indicator.governmentNote).toHaveText("");
    },
  );

  for (const theme of ["light", "dark"] as const) {
    test(
      `the fully-labelled state has zero automatically-detectable accessibility violations (${theme} theme)`,
      { tag: ["@indicator-page", "@a11y", "@annotation-label"] },
      async ({ page }) => {
        // Every other audit on this page runs over the DEFAULT state, in which
        // none of this markup exists. This is the same gate over the state a
        // reader reaches by opening both annotation groups.
        const indicator = new IndicatorPage(page, MIXED_SLUG);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        await indicator.annotationToggle("governments").click();
        await indicator.annotationToggle("exogenous").click();
        await expect(indicator.governmentLabels.first()).toBeAttached();
        if (theme === "dark") {
          await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
        }
        const results = await new AxeBuilder({ page }).analyze();
        expect(results.violations).toEqual([]);
      },
    );
  }
});
