// Chart island interaction primitive (web-accessibility-gates spec, "Every
// chart point is reachable by keyboard alone... focus is visible at every
// step" / "No keyboard trap"): pure arrow-key/Home/End focus-index
// arithmetic for a roving-tabindex point list. Deliberately CLAMPS rather
// than wraps at the ends — no interaction the chart itself defines ever
// moves focus outside `[0, count-1]`, so Tab always leaves the chart in the
// natural document order (never trapped by a wrap-around). Pure, no DOM.
import { describe, expect, it } from "vitest";
import { nextFocusIndex } from "../../src/lib/chart/focusIndex";

describe("nextFocusIndex", () => {
  it("returns -1 for zero points regardless of key", () => {
    expect(nextFocusIndex(0, "ArrowRight", 0)).toBe(-1);
  });

  it("ArrowRight advances by one, clamped at the last index", () => {
    expect(nextFocusIndex(0, "ArrowRight", 5)).toBe(1);
    expect(nextFocusIndex(4, "ArrowRight", 5)).toBe(4);
  });

  it("ArrowLeft retreats by one, clamped at zero", () => {
    expect(nextFocusIndex(3, "ArrowLeft", 5)).toBe(2);
    expect(nextFocusIndex(0, "ArrowLeft", 5)).toBe(0);
  });

  it("Home jumps to the first point from anywhere", () => {
    expect(nextFocusIndex(4, "Home", 5)).toBe(0);
  });

  it("End jumps to the last point from anywhere", () => {
    expect(nextFocusIndex(0, "End", 5)).toBe(4);
  });

  it("reaches every index of a 12-point series via repeated ArrowRight from 0", () => {
    let index = 0;
    const seen = new Set<number>([0]);
    for (let i = 0; i < 11; i++) {
      index = nextFocusIndex(index, "ArrowRight", 12);
      seen.add(index);
    }
    expect(seen.size).toBe(12);
    expect(index).toBe(11);
  });
});
