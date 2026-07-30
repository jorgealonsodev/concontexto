import { test, expect, type Locator } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { WorkbenchPage } from "./workbench-page";
import { CONTRAST_LEVEL, CONTRAST_THRESHOLD, contrastRatio } from "../../../src/lib/design-system/contrast";

// design-system spec, "Interactive controls meet the 44 px touch target" and
// "AA contrast is measured in both themes" — tasks.md 6.5 (RED) / 6.6 (GREEN),
// plus this session's explicit instruction to wire the workbench so contrast
// is measured against real rendered components, not token pairs alone (slice
// 5's `contrast.test.ts` measures the declared tokens; this measures the
// browser's own computed styles on the real DOM the seven components emit).
test.describe("Component workbench", () => {
  test(
    "every interactive control measures at least 44x44 CSS px at a 375px viewport",
    { tag: ["@design-system", "@touch-target"] },
    async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 900 });
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      // The island's custom-range picker exists only once hydrated (it
      // cannot work without JavaScript, so it is not server-rendered). A
      // sweep that runs before hydration would silently skip the two date
      // inputs and the commit button — i.e. it would pass by not looking.
      await workbench.waitForChartIslandHydrated();
      await workbench.chartIsland.getByTestId("custom-range-apply").waitFor();

      const controls = workbench.interactiveControls();
      const count = await controls.count();
      expect(count).toBeGreaterThan(0);

      const undersized: string[] = [];
      for (let i = 0; i < count; i++) {
        const control = controls.nth(i);
        // Only controls actually rendered/visible at this viewport count — a
        // control inside MethodologySheet's native, closed-by-default
        // `<details>` (mobile) is legitimately not on screen until the
        // reader expands the disclosure, matching the spec's own scenario
        // wording ("any interactive control RENDERED at a 375 px viewport").
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

  test(
    "real rendered components meet AA contrast in both themes",
    { tag: ["@design-system", "@a11y", "@contrast"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      // Same reason as the touch-target sweep above: the custom-range
      // picker's own label and inputs are hydration-only markup, so the
      // dark-theme measurement below has nothing to read until the island
      // has hydrated in BOTH workbench sections.
      await workbench.waitForChartIslandHydrated();
      await workbench.darkSection.getByTestId("custom-range-apply").waitFor();

      /** Resolves the DOM's ACTUAL rendered text colour and its nearest fully
       * opaque ancestor background — measured via `getComputedStyle` on the
       * real page, not read from `theme.css`. Disclosed simplification: an
       * element whose own background is translucent (e.g. a badge painted
       * with a `/10`-opacity token) is skipped in favour of its nearest fully
       * opaque ancestor, rather than compositing alpha layers pixel-by-pixel
       * — a conservative approximation given every affected pairing already
       * clears its threshold by a wide margin against the pure opaque token
       * (verified separately in `contrast.test.ts`), not a silent shortcut. */
      async function measure(locator: Locator): Promise<{ fgHex: string; bgHex: string }> {
        const result = await locator.evaluate((el) => {
          function toHex(rgb: string): string {
            const m = /rgba?\(([^)]+)\)/.exec(rgb);
            if (!m) throw new Error(`unexpected computed colour: ${rgb}`);
            const [r, g, b] = m[1].split(",").map((p) => Math.round(parseFloat(p.trim())));
            return `#${[r, g, b].map((c) => c.toString(16).padStart(2, "0")).join("")}`;
          }
          function alpha(rgb: string): number {
            const m = /rgba?\(([^)]+)\)/.exec(rgb);
            if (!m) return 1;
            const parts = m[1].split(",");
            return parts.length > 3 ? parseFloat(parts[3]) : 1;
          }
          const fg = getComputedStyle(el as Element).color;
          let node: Element | null = el as Element;
          let bg = "rgb(255, 255, 255)";
          while (node) {
            const candidate = getComputedStyle(node).backgroundColor;
            if (alpha(candidate) > 0.98) {
              bg = candidate;
              break;
            }
            node = node.parentElement;
          }
          return { fgHex: toHex(fg), bgHex: toHex(bg) };
        });
        return result;
      }

      async function assertPairing(
        label: string,
        locator: Locator,
        theme: "light" | "dark",
        level: (typeof CONTRAST_LEVEL)[keyof typeof CONTRAST_LEVEL],
      ) {
        const { fgHex, bgHex } = await measure(locator);
        const ratio = contrastRatio(fgHex, bgHex);
        const threshold = CONTRAST_THRESHOLD[level];
        expect(
          ratio,
          `"${label}" (${theme}) measured ${ratio.toFixed(2)}:1 (fg ${fgHex} on bg ${bgHex}), below its required ${threshold}:1`,
        ).toBeGreaterThanOrEqual(threshold);
      }

      for (const theme of ["light", "dark"] as const) {
        const section = theme === "light" ? workbench.lightSection : workbench.darkSection;
        await assertPairing(
          "IndicatorCard name (sanity baseline)",
          section.getByTestId("indicator-card-name").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        await assertPairing(
          "FreshnessSemaphore fresh label",
          section.getByTestId("freshness-fresh").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        await assertPairing(
          "FreshnessSemaphore source-pending label",
          section.getByTestId("freshness-source-pending").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        await assertPairing(
          "MethodologySheet body text",
          section.getByTestId("methodology-measures").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        await assertPairing(
          // `.first()`: slice 7's IndicatorChart section now renders a
          // second BreakBand (inside its own break list) alongside this
          // section's own standalone demo — same convention already used
          // above for every other testid this workbench renders more than
          // once.
          "BreakBand tooltip text",
          section.getByTestId("break-band-tooltip").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        await assertPairing(
          "AccessibleDataTable provisional value cell",
          section.getByTestId("table-value-P").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        await assertPairing(
          "AccessibleDataTable definitive value cell",
          section.getByTestId("table-value-D").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        // The custom range picker (verify-report WARNING-5) — its field
        // labels are muted text, and its date inputs paint `text-ink` on an
        // explicit `bg-surface` rather than inheriting whatever the browser's
        // own form defaults would be, which is exactly the pairing measured
        // here in both themes.
        await assertPairing(
          "custom range field label",
          section.getByTestId("custom-range-from-label").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
        await assertPairing(
          "custom range date input",
          section.getByTestId("custom-range-from").first(),
          theme,
          CONTRAST_LEVEL.BODY_TEXT,
        );
      }
    },
  );

  test(
    "has zero automatically-detectable accessibility violations",
    { tag: ["@design-system", "@a11y"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      // Audit the HYDRATED page — the custom-range picker's labelled date
      // inputs and live region are hydration-only markup (see the
      // touch-target sweep above for why).
      await workbench.waitForChartIslandHydrated();
      await workbench.darkSection.getByTestId("custom-range-apply").waitFor();

      const results = await new AxeBuilder({ page }).analyze();
      expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
    },
  );

  // design-system spec, "break band" entry: the tooltip is SUPPLEMENTARY
  // detail revealed on hover/focus — the band itself is what is
  // unconditionally visible (indicator-page spec, P4). A tooltip that is
  // painted at rest is not a disclosure, it is an overlay sitting on top of
  // whatever follows it in the layout.
  //
  // This assertion is page-wide on purpose, not scoped to one component.
  // `break-band__tooltip` markup is emitted by TWO independent renderers —
  // `BreakBand.astro` and `ChartIsland.svelte`'s own break list — because
  // the Astro/Svelte boundary forces the island to re-render rather than
  // hydrate (slice 8's declared risk, carried into slice 9a). Every
  // pre-existing test scoped itself to one of the two, so a rule present in
  // one and absent in the other satisfied all of them: 247 unit tests
  // assert BreakBand's prop types, and the e2e gates assert the band
  // survives a range change, meets contrast, meets 44 px and passes axe.
  // None of them asks whether the thing is painted on top of the page. This
  // one does, over every instance the page emits regardless of origin.
  test(
    "no break-band tooltip is painted at rest, whichever component rendered it",
    { tag: ["@design-system", "@break-band"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();

      const tooltips = page.getByTestId("break-band-tooltip");
      const count = await tooltips.count();
      // Guards against the assertion silently passing over an empty set if
      // the workbench fixtures ever stop declaring a break.
      expect(count, "the workbench renders no break-band tooltip at all").toBeGreaterThan(0);

      const painted: string[] = [];
      for (let i = 0; i < count; i++) {
        const tooltip = tooltips.nth(i);
        const opacity = await tooltip.evaluate((el) => getComputedStyle(el).opacity);
        if (opacity !== "0") {
          const id = (await tooltip.getAttribute("id")) ?? `index ${i}`;
          painted.push(`${id} (opacity ${opacity})`);
        }
      }
      expect(
        painted,
        `break-band tooltips visible with no hover or focus: ${painted.join(", ")}`,
      ).toEqual([]);
    },
  );

  test(
    "focusing a break-band trigger reveals its own tooltip and no other",
    { tag: ["@design-system", "@break-band"] },
    async ({ page }) => {
      const workbench = new WorkbenchPage(page);
      await workbench.goto();
      await workbench.waitForChartIslandHydrated();

      // The island's own break list — the renderer that carried the defect.
      // Asserting the reveal here, not only on the Astro component, is what
      // keeps the fix from regressing into "hidden everywhere, therefore
      // never disclosed".
      const band = workbench.chartIsland.locator(".break-band").first();
      const tooltip = band.getByTestId("break-band-tooltip");
      await expect(tooltip).toHaveCSS("opacity", "0");

      // Selected by its own class rather than `[tabindex="0"]`: the trigger is
      // now a `<button>` (verify-report WARNING-7), which is focusable without
      // an explicit tabindex. Naming the element this test means, instead of
      // the mechanism that happened to make it focusable, is also what keeps
      // this assertion honest if that mechanism changes again.
      await band.locator(".break-band__trigger").first().focus();
      await expect(tooltip).toHaveCSS("opacity", "1");
    },
  );
});
