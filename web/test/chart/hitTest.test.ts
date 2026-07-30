// Chart island interaction primitive (web-accessibility-gates spec, "Every
// chart point is reachable by keyboard"; indicator-page spec tooltips):
// resolves a pointer's x coordinate (in SVG viewBox units, already scaled by
// the caller from the pointer event's client coordinates) to the nearest
// plotted point index — the tooltip trigger's hit-testing contract. Pure, no
// DOM — task 8.x RED (slice 8).
import { describe, expect, it } from "vitest";
import { nearestPointIndexForX } from "../../src/lib/chart/hitTest";
import { DEFAULT_DIMENSIONS, plotArea, xForIndex } from "../../src/lib/chart/geometry";

describe("nearestPointIndexForX", () => {
  it("returns -1 for zero points", () => {
    expect(nearestPointIndexForX(100, 0, DEFAULT_DIMENSIONS)).toBe(-1);
  });

  it("centers a single point regardless of pointer position", () => {
    expect(nearestPointIndexForX(0, 1, DEFAULT_DIMENSIONS)).toBe(0);
    expect(nearestPointIndexForX(9999, 1, DEFAULT_DIMENSIONS)).toBe(0);
  });

  it("resolves an exact match to its own index", () => {
    const count = 12;
    for (let i = 0; i < count; i++) {
      const x = xForIndex(i, count, DEFAULT_DIMENSIONS);
      expect(nearestPointIndexForX(x, count, DEFAULT_DIMENSIONS)).toBe(i);
    }
  });

  it("resolves a pointer left of the plot area to the first point", () => {
    const area = plotArea(DEFAULT_DIMENSIONS);
    expect(nearestPointIndexForX(area.x0 - 500, 12, DEFAULT_DIMENSIONS)).toBe(0);
  });

  it("resolves a pointer right of the plot area to the last point", () => {
    const area = plotArea(DEFAULT_DIMENSIONS);
    expect(nearestPointIndexForX(area.x1 + 500, 12, DEFAULT_DIMENSIONS)).toBe(11);
  });

  it("resolves a pointer exactly midway between two points to the earlier one (tie-break)", () => {
    const count = 3;
    const x0 = xForIndex(0, count, DEFAULT_DIMENSIONS);
    const x1 = xForIndex(1, count, DEFAULT_DIMENSIONS);
    const midpoint = (x0 + x1) / 2;
    expect(nearestPointIndexForX(midpoint, count, DEFAULT_DIMENSIONS)).toBe(0);
  });
});
