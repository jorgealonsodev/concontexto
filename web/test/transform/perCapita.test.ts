// series-transformations spec, "Per capita uses the resident population at
// each observation's own date" / "Per capita exists only where the
// denominator genuinely exists". Pure-function, red-first.
import { describe, expect, it } from "vitest";
import { computePerCapita } from "../../src/lib/transform/perCapita";

describe("computePerCapita", () => {
  it("divides each observation by the population value at that SAME period", () => {
    const points = [{ period: "2020-Q1", value: 1000 }];
    const population = [{ period: "2020-Q1", value: 100 }];
    const result = computePerCapita(points, population);
    expect(result.points).toEqual([{ period: "2020-Q1", value: 10 }]);
  });

  it("never uses the latest population value for an earlier period (no retroprojection)", () => {
    const points = [
      { period: "2020-Q1", value: 1000 },
      { period: "2021-Q1", value: 1100 },
    ];
    const population = [
      { period: "2020-Q1", value: 100 },
      { period: "2021-Q1", value: 110 },
    ];
    const result = computePerCapita(points, population);
    // 2020-Q1 must divide by 100 (its own period), never by 110 (the
    // "latest" population value).
    const q1 = result.points.find((p) => p.period === "2020-Q1");
    expect(q1?.value).toBeCloseTo(10, 5);
  });

  it("produces no point for a period with no population observation, never interpolated", () => {
    const points = [
      { period: "2020-Q1", value: 1000 },
      { period: "2020-Q2", value: 1050 }, // no matching population period below
      { period: "2020-Q3", value: 1100 },
    ];
    const population = [
      { period: "2020-Q1", value: 100 },
      { period: "2020-Q3", value: 105 },
    ];
    const result = computePerCapita(points, population);
    expect(result.points.map((p) => p.period)).not.toContain("2020-Q2");
  });

  it("reports coverage as null and no points when the denominator covers nothing", () => {
    const points = [{ period: "2020-Q1", value: 1000 }];
    const population = [{ period: "2019-Q1", value: 100 }];
    const result = computePerCapita(points, population);
    expect(result.coverage).toBeNull();
    expect(result.points).toEqual([]);
  });

  it("restricts coverage to the maximal contiguous span, not the full series", () => {
    // Population covers Q1-Q2 only; indicator has Q1-Q4.
    const points = [
      { period: "2020-Q1", value: 1000 },
      { period: "2020-Q2", value: 1010 },
      { period: "2020-Q3", value: 1020 },
      { period: "2020-Q4", value: 1030 },
    ];
    const population = [
      { period: "2020-Q1", value: 100 },
      { period: "2020-Q2", value: 101 },
    ];
    const result = computePerCapita(points, population);
    expect(result.coverage).toEqual({ from: "2020-Q1", to: "2020-Q2" });
    expect(result.points.map((p) => p.period)).toEqual(["2020-Q1", "2020-Q2"]);
  });

  it("picks the LONGER of two disjoint covered runs", () => {
    const points = [
      { period: "2020-Q1", value: 10 },
      { period: "2020-Q2", value: 11 },
      { period: "2020-Q3", value: 12 }, // no population here
      { period: "2020-Q4", value: 13 },
      { period: "2021-Q1", value: 14 },
      { period: "2021-Q2", value: 15 },
    ];
    const population = [
      { period: "2020-Q1", value: 100 }, // run of 2
      { period: "2020-Q2", value: 100 },
      { period: "2020-Q4", value: 100 }, // run of 3
      { period: "2021-Q1", value: 100 },
      { period: "2021-Q2", value: 100 },
    ];
    const result = computePerCapita(points, population);
    expect(result.coverage).toEqual({ from: "2020-Q4", to: "2021-Q2" });
  });

  it("skips a zero-valued population observation rather than dividing by zero", () => {
    const points = [{ period: "2020-Q1", value: 1000 }];
    const population = [{ period: "2020-Q1", value: 0 }];
    const result = computePerCapita(points, population);
    expect(result.coverage).toBeNull();
  });
});
