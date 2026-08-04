// web-accessibility-gates spec, "Every chart has a textual description of
// its main pattern" — PRD §12.5's own worked example: "el paro sube de X a
// Y entre A y B, luego desciende…". Pure-function, red-first per this
// project's Strict-TDD convention for pure render functions.
import { describe, expect, it } from "vitest";
import { describeSeries } from "../../src/lib/chart/description";
import type { ChartPoint } from "../../src/lib/chart/geometry";

function points(values: (number | null)[]): ChartPoint[] {
  return values.map((value, i) => ({ period: `2020-Q${(i % 4) + 1}`, value, status: "D" as const }));
}

describe("describeSeries", () => {
  it("returns the no-data message for an entirely empty series", () => {
    expect(describeSeries({ points: [], unit: "%", decimals: 1 })).toContain("No hay datos");
  });

  it("describes a single-point series without inventing a trend", () => {
    const text = describeSeries({ points: points([10]), unit: "%", decimals: 1 });
    // `10,0`, with the Spanish decimal comma: this description is prose a
    // reader reads and a screen reader speaks, so it goes through the same
    // `lib/format/number.ts` every other reader-facing numeral does.
    expect(text).toContain("10,0 %");
    expect(text).not.toContain("10.0 %");
    // And "T1 2020", not "2020-Q1": this description is prose, so its dates
    // go through `lib/format/period.ts`'s PROSE register exactly as its
    // numerals go through `formatNumber`. `Q` is the English abbreviation
    // for *quarter*; INE — the source of every figure here — publishes T.
    expect(text).toContain("T1 2020");
    expect(text).not.toContain("2020-Q1");
  });

  it("names start, end and a monotonic rise", () => {
    const text = describeSeries({ points: points([10, 11, 12, 15]), unit: "%", decimals: 1 });
    expect(text).toContain("sube");
    expect(text).toContain("10,0 %");
    expect(text).toContain("T1 2020");
    expect(text).toContain("15,0 %");
  });

  it("names start, end and a monotonic fall", () => {
    const text = describeSeries({ points: points([20, 18, 15, 10]), unit: "%", decimals: 1 });
    expect(text).toContain("desciende");
  });

  it("describes a genuine reversal with the PRD's own two-segment shape (rise then fall)", () => {
    // 10 -> 20 (rise, peak) -> 12 (fall)
    const text = describeSeries({ points: points([10, 15, 20, 12]), unit: "%", decimals: 1 });
    expect(text).toContain("sube");
    expect(text).toContain("y después desciende");
    expect(text).toContain("20,0 %"); // the peak/turning value
  });

  it("describes a genuine reversal the other way (fall then rise)", () => {
    const text = describeSeries({ points: points([20, 15, 10, 18]), unit: "%", decimals: 1 });
    expect(text).toContain("desciende");
    expect(text).toContain("y después sube");
  });

  it("falls back to the simple two-point form when the interior extremum does not represent a real reversal", () => {
    // A tiny interior wobble that never actually changes the overall
    // monotonic direction from start to end.
    const text = describeSeries({ points: points([10, 10.5, 10.2, 20]), unit: "%", decimals: 1 });
    expect(text).not.toContain("y después");
    expect(text).toContain("sube");
  });

  it("skips null observations without treating them as zero", () => {
    const text = describeSeries({
      points: [
        { period: "2020-Q1", value: 5, status: "D" },
        { period: "2020-Q2", value: null, status: "D" },
        { period: "2020-Q3", value: 8, status: "D" },
      ],
      unit: "%",
      decimals: 1,
    });
    expect(text).toContain("5,0 %");
    expect(text).toContain("8,0 %");
  });

  it("produces a different description for a different series (differs between series)", () => {
    const a = describeSeries({ points: points([10, 11, 12, 15]), unit: "%", decimals: 1 });
    const b = describeSeries({ points: points([100, 90, 80, 70]), unit: "personas", decimals: 0 });
    expect(a).not.toBe(b);
  });
});
