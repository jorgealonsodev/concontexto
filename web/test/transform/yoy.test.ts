// series-transformations spec, "Transformations never invent data" — "A
// year-on-year value MUST NOT be emitted for periods with no
// same-period-prior-year observation" / "the first year of a series yields
// no year-on-year points". Pure-function, red-first.
import { describe, expect, it } from "vitest";
import { computeIntraPeriodRate, computeYoY } from "../../src/lib/transform/yoy";

describe("computeYoY", () => {
  it("yields no point for the first year of a quarterly series (no prior-year observation)", () => {
    const points = [
      { period: "2020-Q1", value: 100 },
      { period: "2020-Q2", value: 101 },
      { period: "2020-Q3", value: 102 },
      { period: "2020-Q4", value: 103 },
    ];
    expect(computeYoY(points, "Q")).toEqual([]);
  });

  it("emits a rate once a same-period prior-year observation exists", () => {
    const points = [
      { period: "2020-Q1", value: 100 },
      { period: "2021-Q1", value: 110 },
    ];
    const result = computeYoY(points, "Q");
    expect(result).toHaveLength(1);
    expect(result[0].period).toBe("2021-Q1");
    expect(result[0].value).toBeCloseTo(10, 5); // +10%
  });

  it("never invents a point across a gap where the prior-year period is missing", () => {
    const points = [
      { period: "2020-Q1", value: 100 },
      // 2020-Q2 missing entirely
      { period: "2021-Q2", value: 90 },
    ];
    expect(computeYoY(points, "Q")).toEqual([]);
  });

  it("skips a null-valued point entirely, never treating it as zero", () => {
    const points = [
      { period: "2020-Q1", value: 100 },
      { period: "2021-Q1", value: null },
    ];
    expect(computeYoY(points, "Q")).toEqual([]);
  });

  it("works across a monthly frequency", () => {
    const points = [
      { period: "2025-01", value: 105.2 },
      { period: "2026-01", value: 107.5 },
    ];
    const result = computeYoY(points, "M");
    expect(result).toHaveLength(1);
    expect(result[0].value).toBeCloseTo(((107.5 / 105.2) - 1) * 100, 5);
  });
});

describe("computeIntraPeriodRate (PIB's mandatory quarter-on-quarter)", () => {
  it("compares each period against the immediately preceding one", () => {
    const points = [
      { period: "2026-Q1", value: 100 },
      { period: "2026-Q2", value: 102 },
    ];
    const result = computeIntraPeriodRate(points, "Q");
    expect(result).toHaveLength(1);
    expect(result[0].period).toBe("2026-Q2");
    expect(result[0].value).toBeCloseTo(2, 5);
  });

  it("emits nothing for the series' very first period", () => {
    const points = [{ period: "2026-Q1", value: 100 }];
    expect(computeIntraPeriodRate(points, "Q")).toEqual([]);
  });

  it("rolls correctly across a year boundary", () => {
    const points = [
      { period: "2025-Q4", value: 100 },
      { period: "2026-Q1", value: 105 },
    ];
    const result = computeIntraPeriodRate(points, "Q");
    expect(result).toHaveLength(1);
    expect(result[0].value).toBeCloseTo(5, 5);
  });
});
