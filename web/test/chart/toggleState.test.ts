// Chart island interaction primitive: reduces the single-active-transform
// toggle state (series-transformations spec — YoY/QoQ/per-capita are
// mutually exclusive views over one chart, never combined; selecting the
// currently-active one returns to the raw series, exactly like an ordinary
// pressed toggle button — not a radio group that always keeps one option
// selected). Pure, no DOM.
import { describe, expect, it } from "vitest";
import { reduceTransform } from "../../src/lib/chart/toggleState";

describe("reduceTransform", () => {
  it("activates a different transform", () => {
    expect(reduceTransform("raw", "yoy")).toBe("yoy");
    expect(reduceTransform("yoy", "perCapita")).toBe("perCapita");
  });

  it("re-selecting the already-active transform returns to raw", () => {
    expect(reduceTransform("yoy", "yoy")).toBe("raw");
    expect(reduceTransform("qoq", "qoq")).toBe("raw");
    expect(reduceTransform("perCapita", "perCapita")).toBe("raw");
  });

  it("re-selecting raw while raw is active stays raw", () => {
    expect(reduceTransform("raw", "raw")).toBe("raw");
  });
});
