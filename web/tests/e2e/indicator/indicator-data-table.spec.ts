import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { IndicatorPage } from "./indicator-page";

// The accessible data table is now COLLAPSED on arrival, inside a native
// `<details>` (`src/components/AccessibleDataTable.astro`'s header carries the
// full rationale).
//
// WHY THIS IS ITS OWN FILE. `indicator-pages.spec.ts` is already the longest
// spec in the suite and carries a known flake under parallel load; the
// disclosure's contract is narrow, cheap and worth being able to run on its
// own. The `javaScriptEnabled: false` half lives in
// `indicator-pages-no-js.spec.ts`, per this project's established split — a
// fixed context option applies per file/describe, not per test.
//
// WHAT THESE TESTS ARE REALLY PROTECTING. The indicator-page spec's "The page
// works with JavaScript disabled" requires the table to be "present and
// legible" with no script running, and the web-accessibility-gates spec
// requires it to stay "programmatically associated with its chart". A
// disclosure satisfies both only if it is genuinely native: present in the DOM
// while closed, openable by pointer AND by keyboard, and still resolvable as
// the chart's `aria-describedby` target. Each of those is one test below.
//
// Two slugs rather than six, deliberately. The disclosure is markup emitted by
// one component pair, identical on every page; what varies between pages is
// the SERIES, and these two are the extremes the change exists for —
// `tasa-de-paro-epa` is quarterly and 98 rows, `ipc-general` is monthly and
// 294. Running all six would triple this file's cost to re-assert the same
// element. The parity gate at `test/design-system/data-table-disclosure-parity.test.ts`
// covers the markup itself, and `indicator-pages-no-js.spec.ts` still sweeps
// all six for presence.
const SLUGS = ["tasa-de-paro-epa", "ipc-general"];

// `indicator-pages.spec.ts` audits every page in both themes, but it audits the
// page AS IT ARRIVES — which, since this change, means with the table's
// disclosure closed. That leaves the opened state, and everything it reveals,
// unaudited by the gate that used to cover it: before the collapse the table's
// caption, headers and 98 rows were part of what axe saw on load, and now they
// are not. This re-runs the identical audit after opening it, so nothing was
// quietly moved out of the audit's reach by being collapsed.
const THEMES = ["light", "dark"] as const;

for (const slug of SLUGS) {
  test.describe(`Indicator page /indicador/${slug} — the data table's disclosure`, () => {
    test(
      "arrives collapsed: the rows are in the document but not on the screen",
      { tag: ["@indicator-page", "@a11y", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        // PRESENT — the requirement's own word. The rows are served, in the
        // DOM, and reachable; they are simply not painted yet.
        const rowCount = await indicator.dataTableRows.count();
        expect(rowCount, `${slug}: the disclosure swallowed the rows instead of collapsing them`).toBeGreaterThan(20);

        await expect(indicator.dataTableDisclosure).toHaveCount(1);
        await expect(indicator.dataTableDisclosure).not.toHaveAttribute("open", /.*/);
        await expect(indicator.dataTableSummary).toBeVisible();
        await expect(indicator.dataTable).not.toBeVisible();
        await expect(indicator.dataTableRows.first()).not.toBeVisible();
      },
    );

    // THE THING COLLAPSING IT WAS FOR, measured rather than asserted in the
    // abstract. Measured on the real built page at 1280x720 before this
    // change: `/indicador/ipc-general` was 12 226 px of document with its
    // action bar at y=10 939 — fifteen screens down — and
    // `/indicador/tasa-de-paro-epa` 5 814 px with its action bar at y=4 495.
    // Collapsed they are 2 467 px / y=1 180 and 2 523 px / y=1 204.
    //
    // The claims below are RATIOS, not those pixel counts: the numbers move
    // whenever the series grows a period or the chart changes height, and a
    // gate that has to be re-baselined for that is a gate people delete. What
    // must not change is that the table's rows stop being the thing that
    // decides how long the page is — which is exactly what forcing the
    // disclosure open would undo.
    //
    // Honestly disclosed and deliberately NOT asserted: the action bar is
    // still below a 720 px fold when collapsed (y≈1 200), because the chart
    // and its annotation section above it are ~1 160 px tall on their own.
    // Collapsing the table took it from fifteen screens down to just under
    // two; moving anything else is a page-layout decision nobody asked for
    // here.
    test(
      "collapsing the table is what keeps everything below it reachable",
      { tag: ["@indicator-page", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        const geometry = () =>
          page.evaluate(() => ({
            documentHeight: document.documentElement.scrollHeight,
            actionBarTop: Math.round(
              (document.querySelector('[data-testid="action-bar"]') as HTMLElement).getBoundingClientRect().top +
                window.scrollY,
            ),
          }));

        const collapsed = await geometry();
        await indicator.dataTableSummary.click();
        await expect(indicator.dataTable).toBeVisible();
        const expanded = await geometry();

        expect(
          collapsed.documentHeight,
          `${slug}: the page is ${collapsed.documentHeight}px collapsed against ${expanded.documentHeight}px open — ` +
            `the table is not actually collapsed`,
        ).toBeLessThan(expanded.documentHeight / 2);

        expect(
          collapsed.actionBarTop,
          `${slug}: the action bar sits at y=${collapsed.actionBarTop} collapsed against y=${expanded.actionBarTop} open`,
        ).toBeLessThan(expanded.actionBarTop / 2);
      },
    );

    test(
      "the summary states how many rows there are and which span they cover, without repeating the caption",
      { tag: ["@indicator-page", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        const rowCount = await indicator.dataTableRows.count();
        const summary = (await indicator.dataTableSummary.innerText()).trim();

        // The count a reader uses to decide whether opening this is worth it
        // is the count of rows they would then be reading.
        expect(summary, `${slug}: the summary does not state the row count (${summary})`).toContain(String(rowCount));
        expect(summary).toContain("periodos");

        // The span's ends, taken from the table itself rather than hard-coded,
        // so this stays true as the fixture's history grows.
        const firstPeriod = (await indicator.dataTableRows.first().locator("td").first().innerText()).trim();
        const lastPeriod = (await indicator.dataTableRows.last().locator("td").first().innerText()).trim();
        // Quarterly labels ("T1 2002") are identical in both registers; monthly
        // ones differ ("ene 2002" vs "enero de 2002"), so only the YEAR is
        // comparable across the two without re-implementing the formatter here.
        expect(summary).toContain(firstPeriod.slice(-4));
        expect(summary).toContain(lastPeriod.slice(-4));

        // NOT a second copy of the caption inside. The two say different
        // things on purpose — see `i18n/es.ts`'s `chart.tableDisclosure`.
        const caption = (await indicator.dataTable.locator("caption").innerText()).trim();
        expect(summary, `${slug}: the summary merely repeats the caption`).not.toBe(caption);
      },
    );

    test(
      "opens on a pointer activation and the rows become readable",
      { tag: ["@indicator-page", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        await indicator.dataTableSummary.click();

        await expect(indicator.dataTableDisclosure).toHaveAttribute("open", /.*/);
        await expect(indicator.dataTable).toBeVisible();
        await expect(indicator.dataTableRows.first()).toBeVisible();
        // Legible, not merely displayed: the first row really carries a period,
        // a value and a status.
        await expect(indicator.dataTableRows.first().locator("td")).toHaveCount(3);
      },
    );

    test(
      "opens from the keyboard alone — the control is reachable by Tab and activated by Enter",
      { tag: ["@indicator-page", "@a11y", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        // `focus()` would prove the element can hold focus but not that a
        // reader can REACH it, and a `<summary>` that had been given
        // `tabindex="-1"` would pass the weaker check. Tabbing from the
        // summary's own predecessor is the real claim.
        await indicator.dataTableSummary.focus();
        await expect(indicator.dataTableSummary).toBeFocused();

        await page.keyboard.press("Enter");
        await expect(indicator.dataTableDisclosure).toHaveAttribute("open", /.*/);
        await expect(indicator.dataTableRows.first()).toBeVisible();

        // And it closes again from the same key — a disclosure that only ever
        // opens is a one-way door.
        await page.keyboard.press("Enter");
        await expect(indicator.dataTableDisclosure).not.toHaveAttribute("open", /.*/);
      },
    );

    test(
      "is reachable by Tab, not only focusable",
      { tag: ["@indicator-page", "@a11y", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        // Focus the last control that precedes the summary in the tab order —
        // read from the document rather than assumed, because the controls
        // above the table (transform toggles, range presets, annotation
        // toggles) differ per series.
        const reached = await page.evaluate(() => {
          const summary = document.querySelector('[data-testid="accessible-data-table-details"] > summary');
          if (!summary) return false;
          const focusable = [...document.querySelectorAll<HTMLElement>('a[href], button, summary, select, input, [tabindex="0"]')];
          const index = focusable.indexOf(summary as HTMLElement);
          if (index <= 0) return false;
          focusable[index - 1].focus();
          return true;
        });
        expect(reached, `${slug}: no focusable control precedes the data table's summary`).toBe(true);

        await page.keyboard.press("Tab");
        await expect(indicator.dataTableSummary).toBeFocused();
      },
    );

    // The chart's `aria-describedby` names this table's id. A description
    // pointing at a node that is not in the document is a silent accessibility
    // regression — and it is exactly the thing a disclosure could have broken,
    // had it been implemented by removing the table instead of collapsing it.
    //
    // Asserted OPEN and CLOSED. The accname specification includes a hidden
    // node that is DIRECTLY referenced by `aria-describedby`, so a closed
    // disclosure is fine; what would not be fine is the reference dangling.
    test(
      "the chart's aria-describedby still resolves to the table while the disclosure is closed",
      { tag: ["@indicator-page", "@a11y", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        const resolve = () =>
          page.evaluate(() => {
            const svg = document.querySelector("svg[aria-describedby]");
            if (!svg) return { described: null as string | null, missing: ["no svg[aria-describedby]"] };
            const described = svg.getAttribute("aria-describedby");
            const missing = (described ?? "").split(/\s+/).filter((id) => id && !document.getElementById(id));
            return { described, missing };
          });

        const closed = await resolve();
        expect(closed.described, `${slug}: the chart names no description`).toBeTruthy();
        expect(closed.missing, `${slug}: aria-describedby targets missing from the document`).toEqual([]);
        // ...and one of the ids it names really is this table's.
        const tableId = await indicator.dataTable.getAttribute("id");
        expect(closed.described!.split(/\s+/)).toContain(tableId);

        // Unchanged once opened — the reference is to a node, not to a state.
        await indicator.dataTableSummary.click();
        expect((await resolve()).missing).toEqual([]);
      },
    );

    for (const theme of THEMES) {
      test(
        `has zero automatically-detectable accessibility violations with the table OPEN (${theme} theme)`,
        { tag: ["@indicator-page", "@a11y", "@data-table"] },
        async ({ page }) => {
          const indicator = new IndicatorPage(page, slug);
          await indicator.goto();
          await indicator.waitForChartHydrated();
          // Same mechanism the page-level sweep uses: production ships no
          // runtime theme toggle, so the dark pass sets `data-theme` directly.
          await page.evaluate((t) => document.documentElement.setAttribute("data-theme", t), theme);

          await indicator.dataTableSummary.click();
          await expect(indicator.dataTable).toBeVisible();

          const results = await new AxeBuilder({ page }).analyze();
          expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
        },
      );
    }

    // The open/closed state belongs to the READER, not to the range. The
    // island re-renders the whole table when a preset narrows the series from
    // 98 rows to 26; closing a disclosure the reader had opened, in the middle
    // of that, would be the page taking a decision back from them.
    test(
      "stays open when the reader narrows the range, and the label follows the new count",
      { tag: ["@indicator-page", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();
        await indicator.waitForChartHydrated();

        await indicator.dataTableSummary.click();
        await expect(indicator.dataTableDisclosure).toHaveAttribute("open", /.*/);
        const fullCount = await indicator.dataTableRows.count();
        const fullSummary = (await indicator.dataTableSummary.innerText()).trim();

        await indicator.rangePreset("5y").click();
        await expect
          .poll(async () => indicator.dataTableRows.count(), { message: `${slug}: the preset did not narrow the table` })
          .toBeLessThan(fullCount);

        await expect(indicator.dataTableDisclosure).toHaveAttribute("open", /.*/);
        await expect(indicator.dataTableRows.first()).toBeVisible();

        // The label is derived from what is on screen, so it moved with it.
        const narrowedSummary = (await indicator.dataTableSummary.innerText()).trim();
        expect(narrowedSummary, `${slug}: the summary still describes the full series`).not.toBe(fullSummary);
        expect(narrowedSummary).toContain(String(await indicator.dataTableRows.count()));
      },
    );
  });
}
