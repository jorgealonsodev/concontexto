// The accessible data table is emitted by TWO independent renderers, and the
// disclosure it now lives inside has to behave identically in both.
//
// The duplication is the same one `break-band-parity.test.ts` guards and it
// exists for the same forced reason: `ChartIsland.svelte` RE-RENDERS the table
// as the range narrows rather than hydrating `AccessibleDataTable.astro`'s DOM,
// because the Astro/Svelte boundary forbids the latter. This project has
// already had one defect from those two copies drifting, which is why the
// break band has a guard at all — the table had none until this file.
//
// WHAT IS UNDER TEST, and why it is asserted on RENDERED output rather than on
// the source text `break-band-parity.test.ts` reads. That file compares class
// attributes because the break band's contract is a class name a shared
// stylesheet matches. This one's contract is what a reader actually receives:
// a disclosure that is CLOSED, whose label states the same count and the same
// span in both renderers, with the table still inside it. Rendering both is
// the only way to compare the label, because the label is computed from the
// points and not written in either file.
//
// The mutation this is built to catch is the obvious one: adding `open` to a
// `<details>` in one renderer, or in both. Forcing either open fails here.
import { describe, expect, it } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import { render } from "svelte/server";
import AccessibleDataTable from "../../src/components/AccessibleDataTable.astro";
import ChartIsland from "../../src/components/ChartIsland.svelte";
import { dataTableSummaryLabel } from "../../src/lib/chart/tableSummary";

/** The same observations through both renderers — the only way the two labels
 * can be compared for equality rather than merely for shape. Long enough that
 * the count is not 0 or 1 (both of which take their own branch) and spans two
 * years, so a truncated span is visible in the assertion. */
const POINTS = [
  { period: "2024-Q1", value: 12.3, status: "D" as const },
  { period: "2024-Q2", value: 11.9, status: "D" as const },
  { period: "2024-Q3", value: 11.4, status: "D" as const },
  { period: "2025-Q1", value: 11.1, status: "P" as const },
];

const DETAILS_TESTID = "accessible-data-table-details";

interface Disclosure {
  /** The `<details>` opening tag, verbatim. */
  openingTag: string;
  /** The `<summary>` opening tag, verbatim. */
  summaryTag: string;
  /** The summary's visible text, with Svelte's SSR comment markers and
   * whitespace normalised away — the two renderers may frame the same text
   * differently, but they may not print different text. */
  summaryText: string;
  /** Whether the table really is inside the disclosure, rather than beside it
   * with the disclosure wrapping nothing. */
  tableIsInside: boolean;
}

function extractDisclosure(html: string, renderer: string): Disclosure {
  const anchor = html.indexOf(`data-testid="${DETAILS_TESTID}"`);
  expect(anchor, `${renderer} renders no element with data-testid="${DETAILS_TESTID}"`).toBeGreaterThan(-1);

  const tagStart = html.lastIndexOf("<", anchor);
  const openingTag = html.slice(tagStart, html.indexOf(">", anchor) + 1);
  const closing = html.indexOf("</details>", tagStart);
  expect(closing, `${renderer} opens a disclosure it never closes`).toBeGreaterThan(-1);
  const block = html.slice(tagStart, closing);

  const summaryStart = block.indexOf("<summary");
  expect(summaryStart, `${renderer} renders a <details> with no <summary> to open it with`).toBeGreaterThan(-1);
  const summaryTag = block.slice(summaryStart, block.indexOf(">", summaryStart) + 1);
  const summaryInner = block.slice(block.indexOf(">", summaryStart) + 1, block.indexOf("</summary>", summaryStart));

  return {
    openingTag,
    summaryTag,
    summaryText: summaryInner.replace(/<!--[\s\S]*?-->/g, "").replace(/\s+/g, " ").trim(),
    tableIsInside: block.includes('data-testid="accessible-data-table"'),
  };
}

async function renderAstro(): Promise<string> {
  const container = await AstroContainer.create();
  return container.renderToString(AccessibleDataTable, {
    props: {
      id: "parity-table",
      caption: "Datos de Tasa de paro",
      unit: "% población activa",
      decimals: 1,
      points: POINTS,
    },
  });
}

function renderSvelte(): string {
  return render(ChartIsland, {
    props: {
      slug: "parity",
      name: "Tasa de paro",
      unit: "% población activa",
      frequency: "Q" as const,
      decimals: 1,
      points: POINTS,
      transforms: { yoy: false, qoq: false, perCapita: false },
      tableId: "parity-table",
    },
  }).body;
}

describe("the accessible data table's disclosure, across its two renderers", () => {
  it("wraps the table in a <details> in BOTH renderers, with the table genuinely inside it", async () => {
    const astro = extractDisclosure(await renderAstro(), "AccessibleDataTable.astro");
    const svelte = extractDisclosure(renderSvelte(), "ChartIsland.svelte");

    expect(astro.tableIsInside, "AccessibleDataTable.astro renders the table outside its own disclosure").toBe(true);
    expect(svelte.tableIsInside, "ChartIsland.svelte renders the table outside its own disclosure").toBe(true);
  });

  // The whole point of the change. A `<details>` with `open` is a table that
  // still pushes the rest of the page off the screen, and a `<details>` open
  // in only one renderer is the divergence this file exists for.
  it("is CLOSED on arrival in both renderers — no `open` attribute anywhere", async () => {
    const astro = extractDisclosure(await renderAstro(), "AccessibleDataTable.astro");
    const svelte = extractDisclosure(renderSvelte(), "ChartIsland.svelte");

    for (const [renderer, disclosure] of [
      ["AccessibleDataTable.astro", astro],
      ["ChartIsland.svelte", svelte],
    ] as const) {
      expect(
        /\sopen(?=[\s=>])/.test(disclosure.openingTag),
        `${renderer} ships the data table's disclosure OPEN: ${disclosure.openingTag}. ` +
          `The table is 98 rows on tasa-de-paro-epa and 294 on ipc-general; an open disclosure pushes the ` +
          `action bar, the methodology sheet and the related indicators below the fold on arrival, which is ` +
          `exactly what collapsing it was for.`,
      ).toBe(false);
    }
  });

  it("labels the disclosure with the SAME count and the SAME span in both renderers", async () => {
    const astro = extractDisclosure(await renderAstro(), "AccessibleDataTable.astro");
    const svelte = extractDisclosure(renderSvelte(), "ChartIsland.svelte");

    expect(
      svelte.summaryText,
      `data-table DIVERGENCE: the disclosure label differs between the two renderers.\n` +
        `  src/components/AccessibleDataTable.astro: ${astro.summaryText}\n` +
        `  src/components/ChartIsland.svelte: ${svelte.summaryText}\n` +
        `Both must print what src/lib/chart/tableSummary.ts returns — computing it twice is how the two ` +
        `copies of this table drift apart.`,
    ).toBe(astro.summaryText);

    // ...and it is the shared function's own output, not merely two identical
    // hand-written strings that happen to agree today.
    expect(astro.summaryText).toBe(dataTableSummaryLabel(POINTS));
    expect(astro.summaryText).toBe("Tabla de datos (4 periodos, de T1 2024 a T1 2025)");
  });

  // The 44 px touch-target sweep measures every `<summary>` on the page (the
  // Page Objects' own `interactiveControls()`), and it measures the
  // rendered box, not the class list. Pinning the classes here is what makes a
  // failure in that sweep point at ONE renderer instead of at both — and what
  // stops the two summaries being styled differently in the first place.
  it("gives the summary the same classes in both renderers, including the 44 px floor", async () => {
    const astro = extractDisclosure(await renderAstro(), "AccessibleDataTable.astro");
    const svelte = extractDisclosure(renderSvelte(), "ChartIsland.svelte");

    const classesOf = (tag: string): string[] => (/class="([^"]*)"/.exec(tag)?.[1] ?? "").trim().split(/\s+/).sort();

    expect(
      classesOf(svelte.summaryTag),
      `data-table DIVERGENCE: the summary's classes differ between the two renderers.\n` +
        `  src/components/AccessibleDataTable.astro: ${astro.summaryTag}\n` +
        `  src/components/ChartIsland.svelte: ${svelte.summaryTag}`,
    ).toEqual(classesOf(astro.summaryTag));

    expect(
      classesOf(astro.summaryTag),
      "the disclosure's summary must carry the same min-h-11 touch-target floor MethodologySheet's own " +
        "summary carries — it is a control a reader taps on a phone.",
    ).toContain("min-h-11");
  });
});
