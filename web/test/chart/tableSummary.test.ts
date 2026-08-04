// The accessible data table's disclosure label, which BOTH renderers of that
// table have to print identically.
//
// It is a pure module for the same reason `lib/chart/svg.ts` is one (design.md
// D-5, "never two renderers that must agree"): `AccessibleDataTable.astro` and
// `ChartIsland.svelte` each hand-write the table's markup, and a label
// computed twice is a label that can disagree twice. The parity gate in
// `test/design-system/data-table-disclosure-parity.test.ts` proves the two
// renderers really do route through this one function; this file proves the
// function itself is right.
import { describe, expect, it } from "vitest";
import { dataTableSummaryLabel } from "../../src/lib/chart/tableSummary";

/** Only the period matters to this label, so the fixtures carry nothing else —
 * the function's parameter type is deliberately the narrowest thing that can
 * answer the question, not `ChartPoint`. */
function periods(...labels: string[]): { period: string }[] {
  return labels.map((period) => ({ period }));
}

describe("dataTableSummaryLabel", () => {
  it("states the row count and the span a reader would use to decide whether to open the table", () => {
    expect(dataTableSummaryLabel(periods("2002-Q1", "2010-Q3", "2026-Q2"))).toBe(
      "Tabla de datos (3 periodos, de T1 2002 a T2 2026)",
    );
  });

  // The PROSE register, not the compact one `formatPeriodCompact` gives the
  // Periodo column. `lib/format/period.ts` states the rule: compact where a
  // column width or an axis tick is load-bearing, prose where the label sits
  // in a sentence. This label is a sentence, and it has a whole line to
  // itself.
  it("spells the month out, because a disclosure label is a sentence and not a column", () => {
    expect(dataTableSummaryLabel(periods("2002-01", "2026-06"))).toBe(
      "Tabla de datos (2 periodos, de enero de 2002 a junio de 2026)",
    );
  });

  it("prints an annual span with the bare years both registers already agree on", () => {
    expect(dataTableSummaryLabel(periods("2019", "2020", "2021"))).toBe(
      "Tabla de datos (3 periodos, de 2019 a 2021)",
    );
  });

  // Grammar, not decoration: "1 periodos" and "de X a X" are both wrong, and
  // both are reachable. A custom range can be narrowed onto a single
  // observation, and the workbench renders an explicitly empty variant.
  it("does not say 'de X a X' when the range holds a single observation", () => {
    expect(dataTableSummaryLabel(periods("2020-Q3"))).toBe("Tabla de datos (1 periodo, T3 2020)");
  });

  it("says there is nothing to open when the range holds no observation", () => {
    expect(dataTableSummaryLabel([])).toBe("Tabla de datos (sin datos)");
  });

  // The count is a reader-facing number like every other on this site, so it
  // goes through `formatNumber` and inherits the ONE grouping rule
  // `lib/format/number.ts` states — `useGrouping: "min2"`, the Spanish rule of
  // grouping from five digits up and leaving four alone. A five-digit count is
  // the only length at which that rule and bare string interpolation disagree,
  // which is what makes this the assertion that proves the routing rather than
  // merely describing it. No series here reaches four digits today
  // (`ipc-general` is the longest at 294); the rule still has to be the site's
  // single one.
  it("counts the rows through the site's own number formatter, not string interpolation", () => {
    const fourDigits = Array.from({ length: 1234 }, (_, i) => ({ period: `${1000 + i}` }));
    expect(dataTableSummaryLabel(fourDigits)).toBe("Tabla de datos (1234 periodos, de 1000 a 2233)");

    const fiveDigits = Array.from({ length: 12345 }, (_, i) => ({ period: `${1000 + i}` }));
    expect(dataTableSummaryLabel(fiveDigits)).toBe("Tabla de datos (12.345 periodos, de 1000 a 13344)");
  });

  // The label is DISPLAY text and the periods it names are keys the island
  // sorts, slices and joins on. `lib/format/period.ts`'s boundary rule —
  // "format at the render site, never in the data" — is what keeps a
  // chronological axis from becoming an alphabetical one.
  it("does not mutate the points it was handed", () => {
    const points = periods("2002-Q1", "2026-Q2");
    dataTableSummaryLabel(points);
    expect(points).toEqual([{ period: "2002-Q1" }, { period: "2026-Q2" }]);
  });
});
