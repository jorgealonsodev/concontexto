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

        // PRESENT, which is the requirement's own word, and legible on demand
        // through a control the browser operates by itself. The table now
        // arrives inside a native closed `<details>` — same instrument, same
        // `min-h-11` summary, that `MethodologySheet.astro` already meets this
        // requirement with on mobile. It is asserted here as "in the document
        // with its rows" plus "a visible control that opens it", which is
        // strictly more than the bare `toBeVisible()` this line used to make:
        // that assertion could not tell a rendered table from a rendered
        // EMPTY one, and this one can. The opening itself is its own test
        // below, because "the control exists" and "the control works with no
        // script at all" are two different claims.
        const table = indicator.chartSection.locator('[data-testid="accessible-data-table"]');
        await expect(table).toHaveCount(1);
        expect(await table.locator("tbody tr").count()).toBeGreaterThan(0);
        await expect(indicator.dataTableDisclosure).toHaveCount(1);
        await expect(indicator.dataTableSummary).toBeVisible();

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

    // THE LOAD-BEARING TEST FOR COLLAPSING THE TABLE AT ALL.
    //
    // The indicator-page spec requires the accessible data table to be
    // "present and legible" with JavaScript disabled. Collapsing it is only
    // compatible with that because `<details>` is native: the browser opens it
    // by itself, so "legible on demand" costs a reader with no script exactly
    // one activation and no bytes. A collapse that needed the island to
    // hydrate would take the table away from precisely the readers this
    // requirement exists for, and this test is what would catch that being
    // done — it runs in a context where no script executes at all, so nothing
    // but the browser's own disclosure behaviour can make it pass.
    //
    // Both gestures, because a reader without JavaScript is not necessarily a
    // reader with a mouse: activating by pointer and activating by Enter are
    // two separate native behaviours of the same element.
    test(
      `/indicador/${slug}: the data table's disclosure opens — by pointer and by keyboard — with JavaScript disabled`,
      { tag: ["@indicator-page", "@a11y", "@no-js", "@data-table"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();

        await expect(indicator.dataTableDisclosure).not.toHaveAttribute("open", /.*/);
        await expect(indicator.dataTable).not.toBeVisible();

        await indicator.dataTableSummary.click();
        await expect(indicator.dataTableDisclosure).toHaveAttribute("open", /.*/);
        await expect(indicator.dataTable).toBeVisible();

        // LEGIBLE, not merely displayed: the first row carries a period, a
        // value and a status, and every one of them is on the screen.
        const firstRowCells = indicator.dataTableRows.first().locator("td");
        await expect(firstRowCells).toHaveCount(3);
        for (let i = 0; i < 3; i++) {
          await expect(firstRowCells.nth(i)).toBeVisible();
          await expect(firstRowCells.nth(i)).not.toBeEmpty();
        }
        await expect(indicator.dataTable.locator("caption")).toBeVisible();

        // Closes again, then opens from the keyboard alone.
        await indicator.dataTableSummary.click();
        await expect(indicator.dataTableDisclosure).not.toHaveAttribute("open", /.*/);
        // `Locator.press` focuses the element and then sends the key through
        // the browser's own input pipeline — deliberately not `page.evaluate`,
        // which is exactly the thing a `javaScriptEnabled: false` context does
        // not have.
        await indicator.dataTableSummary.press("Enter");
        await expect(indicator.dataTableDisclosure).toHaveAttribute("open", /.*/);
        await expect(indicator.dataTableRows.last()).toBeVisible();
      },
    );

    // The two hydration-only range controls, held to the spec's "absent, not
    // disabled" discipline rather than merely to "does not crash".
    //
    // The government select cannot work on a statically built page with no
    // JavaScript — there is no server to ask for a re-slice — so rendering it
    // would put a control in front of a reader that looks exactly like the
    // working ones beside it and silently does nothing when used. The custom
    // range picker is asserted in the same test because it is the same rule
    // and the same failure.
    //
    // DISCLOSED, unchanged from `chart-no-js.spec.ts`: the five fixed preset
    // BUTTONS remain server-rendered and equally inert without JavaScript.
    // That predates both controls and is recorded rather than swept in.
    test(
      `/indicador/${slug}: the government range control is absent — not present-but-dead — with JavaScript disabled`,
      { tag: ["@indicator-page", "@a11y", "@no-js", "@government-range"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, slug);
        await indicator.goto();

        // The island's server-rendered markup really is here (the table is
        // inside its own native disclosure now, so presence is asserted as
        // presence — see the previous test)...
        await expect(indicator.chartSection.locator('[data-testid="accessible-data-table"]')).toHaveCount(1);
        await expect(indicator.dataTableSummary).toBeVisible();
        // ...and neither hydration-only control is.
        await expect(indicator.governmentRange).toHaveCount(0);
        await expect(indicator.governmentSelect).toHaveCount(0);
        await expect(indicator.chartSection.locator("select")).toHaveCount(0);
        await expect(indicator.customRangeApply).toHaveCount(0);

        // The governments themselves are NOT withheld from a no-JavaScript
        // reader: the editorial annotation layer is static markup and stays.
        // Only the interactive filter over it is absent.
        await expect(indicator.chartSection.getByTestId("annotation-group-governments")).toBeVisible();

        // WHAT A NO-JAVASCRIPT READER DOES NOT GET, stated as an assertion
        // rather than left to be discovered.
        //
        // The on-chart marker is now gated on the `governments` annotation
        // group, which starts CLOSED and cannot be opened without JavaScript —
        // so it is never drawn here. That is a real loss against what this page
        // used to render, and it is the honest reading of the two rules that
        // produce it: the indicator-page spec fixes two of the three annotation
        // groups OFF by default, and the owner's rule is that nothing is drawn
        // unless it is selected. A reader who cannot select cannot be shown a
        // selection they did not make, and the alternative — keeping this ONE
        // group's marks unconditional — is exactly the inconsistency being
        // removed: `exogenous` has behaved this way since it shipped, and the
        // `<select>` above is absent on the same principle.
        //
        // Nothing about the layer is hidden from that reader, only undrawn: the
        // group's own chips are server-rendered inside the disclosure below,
        // which a CSS-only checkbox can open with no script at all, and the
        // accessible data table carries every observation regardless.
        expect(await indicator.governmentMarkers.count()).toBe(0);
        expect(await indicator.governmentMarkersNarrow.count()).toBe(0);
        expect(await indicator.governmentLabels.count()).toBe(0);
        await expect(indicator.legendGovernment).toHaveCount(0);
        await expect(indicator.governmentNote).toHaveText("");
      },
    );
  }
});
