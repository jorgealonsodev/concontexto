// Attaches each derived rate/per-capita point's provisional/definitive
// status by looking it up from the RAW observation at the same period
// (indicator-page spec, "Every point discloses provisional or definitive" —
// this must still hold once a transformation is active, series-
// transformations spec's own "two encodings... distinguished by a legend
// and a non-colour channel"). A derived period with no matching raw status
// is impossible by construction (every rate/per-capita point's period is
// drawn from the raw series' own periods) but is defensively treated as
// definitive rather than throwing, since a missing status must never block
// rendering. Pure, no DOM.
import { describe, expect, it } from "vitest";
import { toChartPoints } from "../../src/lib/chart/chartPoints";

describe("toChartPoints", () => {
  it("attaches each period's raw status to the derived point", () => {
    const statusByPeriod = new Map([
      ["2020-Q1", "D" as const],
      ["2020-Q2", "P" as const],
    ]);
    const result = toChartPoints([{ period: "2020-Q1", value: 1.5 }, { period: "2020-Q2", value: 2.1 }], statusByPeriod);
    expect(result).toEqual([
      { period: "2020-Q1", value: 1.5, status: "D" },
      { period: "2020-Q2", value: 2.1, status: "P" },
    ]);
  });

  it("defaults to definitive for a period absent from the status map (defensive, never blocks rendering)", () => {
    const result = toChartPoints([{ period: "2099-Q1", value: 1 }], new Map());
    expect(result).toEqual([{ period: "2099-Q1", value: 1, status: "D" }]);
  });

  it("returns an empty array for empty input", () => {
    expect(toChartPoints([], new Map())).toEqual([]);
  });
});
