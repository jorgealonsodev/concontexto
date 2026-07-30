// design-system spec, "Tabular figures on all numerals" — tasks.md 5.11
// (RED) / 5.12 (GREEN).
//
// Scope note: the spec's own scenario ("computed font-variant-numeric of
// each numeric element" on "a rendered indicator page") describes a
// browser-measured assertion over real numeral-bearing components, which
// do not exist until slice 6+ (IndicatorCard etc.) and real pages until
// slice 9. This slice proves the MECHANISM at the level that exists today:
// the compiled CSS for the utility numeric elements will use resolves
// `font-variant-numeric` to tabular figures, and the `--font-numeric`
// token survives the `--font-*` zeroing. The full rendered-page scenario
// is re-proven once slice 9's Playwright suite has real numeral-bearing
// pages to measure — disclosed here rather than silently narrowed.
import { describe, expect, it } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import { buildTheme } from "./compile-theme";
import AccessibleDataTable from "../../src/components/AccessibleDataTable.astro";

describe("tabular figures on numerals (PRD §12.1)", () => {
  it("the tabular-nums utility resolves font-variant-numeric to tabular figures", async () => {
    const css = await buildTheme(["tabular-nums"]);
    expect(css).toContain(".tabular-nums {");
    expect(css).toContain("--tw-numeric-spacing: tabular-nums");
    expect(css).toMatch(/font-variant-numeric:\s*var\(--tw-ordinal,?\)/);
  });

  it("--font-numeric survives the --font-* zeroing as a real project token", async () => {
    const css = await buildTheme(["font-numeric"]);
    expect(css).toContain(".font-numeric {");
    expect(css).toContain("font-family: var(--font-numeric)");
  });
});

describe("tabular figures survive locale grouping separators", () => {
  // The open question when values stopped being `toFixed` output: does
  // `22.779,0` beside `1234,0` still line up? `tabular-nums` equalises the
  // advance width of DIGITS, and says nothing about `.` or `,` — so a column
  // of values with differing separator counts cannot be made to align by the
  // font alone, and never could, since the digit counts already differed.
  //
  // What actually holds the column is RIGHT ALIGNMENT, which is why this
  // asserts the two properties together on the same cell: the numerals are
  // tabular AND the cell is right-aligned. Losing either one is what would
  // break alignment, and losing either one is silent.
  it("keeps every value cell both tabular and right-aligned, whatever its magnitude", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(AccessibleDataTable, {
      props: {
        id: "alignment-probe",
        caption: "Alineación",
        unit: "personas",
        decimals: 1,
        points: [
          // Deliberately spanning the grouping threshold: no separator, one,
          // and two — the three shapes a real series produces as it grows.
          { period: "2024-Q1", value: 1234, status: "D" as const },
          { period: "2024-Q2", value: 22779, status: "D" as const },
          { period: "2024-Q3", value: 49687120, status: "P" as const },
        ],
      },
    });

    // The three formatted values really are on the page in the three shapes
    // above — otherwise the class assertion below would hold vacuously.
    expect(html).toContain("1234,0");
    expect(html).toContain("22.779,0");
    expect(html).toContain("49.687.120,0");

    const valueCells = [...html.matchAll(/<td[^>]*data-testid="table-value-[PD]"[^>]*>/g)].map((m) => m[0]);
    expect(valueCells).toHaveLength(3);
    for (const cell of valueCells) {
      expect(cell, `value cell lost its tabular figures: ${cell}`).toContain("tabular-nums");
      expect(cell, `value cell lost its right alignment: ${cell}`).toContain("text-right");
    }
  });
});
