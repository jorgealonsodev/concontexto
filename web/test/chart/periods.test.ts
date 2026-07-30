import { describe, expect, it } from "vitest";
import {
  addYears,
  formatPeriod,
  nearestPeriodIndex,
  parsePeriod,
  periodFromCalendarDate,
  periodOrdinalIndex,
  previousPeriod,
  stepsPerYear,
} from "../../src/lib/chart/periods";

describe("stepsPerYear", () => {
  it("returns the right cadence for each frequency", () => {
    expect(stepsPerYear("Q")).toBe(4);
    expect(stepsPerYear("M")).toBe(12);
    expect(stepsPerYear("A")).toBe(1);
  });
});

describe("parsePeriod / formatPeriod round-trip", () => {
  it.each([
    ["2026-Q1", "Q"],
    ["2026-07", "M"],
    ["2026", "A"],
  ] as const)("round-trips %s under frequency %s", (label, frequency) => {
    const parsed = parsePeriod(label, frequency);
    expect(formatPeriod(parsed.year, parsed.ordinal, frequency)).toBe(label);
  });

  it("throws on a label that does not match the declared frequency's shape", () => {
    expect(() => parsePeriod("2026-Q1", "M")).toThrow();
    expect(() => parsePeriod("2026-07", "A")).toThrow();
  });
});

describe("periodOrdinalIndex", () => {
  it("is strictly increasing across consecutive quarterly periods", () => {
    const q4 = periodOrdinalIndex("2025-Q4", "Q");
    const q1 = periodOrdinalIndex("2026-Q1", "Q");
    expect(q1).toBe(q4 + 1);
  });

  it("is strictly increasing across consecutive monthly periods, including a year rollover", () => {
    const dec = periodOrdinalIndex("2025-12", "M");
    const jan = periodOrdinalIndex("2026-01", "M");
    expect(jan).toBe(dec + 1);
  });
});

describe("addYears", () => {
  it("moves a quarterly label back one year, same quarter", () => {
    expect(addYears("2026-Q2", "Q", -1)).toBe("2025-Q2");
  });

  it("moves a monthly label forward two years, same month", () => {
    expect(addYears("2024-03", "M", 2)).toBe("2026-03");
  });
});

describe("previousPeriod", () => {
  it("steps back within a year for quarterly", () => {
    expect(previousPeriod("2026-Q2", "Q")).toBe("2026-Q1");
  });

  it("rolls back across a year boundary for quarterly", () => {
    expect(previousPeriod("2026-Q1", "Q")).toBe("2025-Q4");
  });

  it("rolls back across a year boundary for monthly", () => {
    expect(previousPeriod("2026-01", "M")).toBe("2025-12");
  });
});

describe("periodFromCalendarDate", () => {
  it("maps a calendar date to its quarter", () => {
    expect(periodFromCalendarDate("2020-06-15", "Q")).toBe("2020-Q2");
    expect(periodFromCalendarDate("2020-01-01", "Q")).toBe("2020-Q1");
    expect(periodFromCalendarDate("2020-12-31", "Q")).toBe("2020-Q4");
  });

  it("maps a calendar date to its month", () => {
    expect(periodFromCalendarDate("2020-06-15", "M")).toBe("2020-06");
  });

  it("maps a calendar date to its year", () => {
    expect(periodFromCalendarDate("2020-06-15", "A")).toBe("2020");
  });
});

describe("nearestPeriodIndex", () => {
  it("finds the exact match when present", () => {
    const periods = ["2020-Q1", "2020-Q2", "2020-Q3", "2020-Q4"];
    expect(nearestPeriodIndex(periods, "Q", "2020-Q3")).toBe(2);
  });

  it("snaps to the nearest available period across a semiannual gap", () => {
    // poblacion-residente's historical segment: only Q1/Q3 exist.
    const periods = ["2019-Q1", "2019-Q3", "2020-Q1", "2020-Q3"];
    expect(nearestPeriodIndex(periods, "Q", "2019-Q2")).toBe(0); // tie -> earlier index
    expect(nearestPeriodIndex(periods, "Q", "2020-Q2")).toBe(2);
  });

  it("returns -1 for an empty periods list", () => {
    expect(nearestPeriodIndex([], "Q", "2020-Q1")).toBe(-1);
  });
});
