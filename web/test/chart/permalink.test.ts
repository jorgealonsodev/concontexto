// series-transformations spec, "Range presets": "Selecting a range preset
// updates the permalink" / "loading that permalink reproduces the same
// range". Pure query-string encode/decode — no DOM, no `window` (the island
// itself is the only caller that touches `window.location`; this module
// only serializes plain state to/from a query string). Round-trips both the
// active range preset AND the active transform, so a shared link reproduces
// the exact view a reader was looking at, not only the range half.
import { describe, expect, it } from "vitest";
import { decodeChartState, encodeChartState } from "../../src/lib/chart/permalink";

describe("encodeChartState / decodeChartState", () => {
  it("round-trips a non-default range and transform", () => {
    const search = encodeChartState({ range: "10y", transform: "yoy" });
    expect(search).toContain("range=10y");
    expect(search).toContain("transform=yoy");
    const decoded = decodeChartState(search, ["full", "5y", "10y"], ["raw", "yoy", "qoq"]);
    expect(decoded).toEqual({ range: "10y", transform: "yoy" });
  });

  it("encodes the default state as an empty query string (a clean permalink for the common case)", () => {
    const search = encodeChartState({ range: "full", transform: "raw" });
    expect(search).toBe("");
  });

  it("falls back to defaults when the query string is empty", () => {
    const decoded = decodeChartState("", ["full", "5y"], ["raw", "yoy"]);
    expect(decoded).toEqual({ range: "full", transform: "raw" });
  });

  it("falls back to defaults when a value is present but not in the available set (e.g. an unavailable preset for this series)", () => {
    const search = encodeChartState({ range: "since-2008", transform: "qoq" });
    const decoded = decodeChartState(search, ["full", "5y"], ["raw", "yoy"]);
    expect(decoded).toEqual({ range: "full", transform: "raw" });
  });

  it("falls back to defaults on a garbage/unrecognised value", () => {
    const decoded = decodeChartState("?range=nonsense&transform=nonsense", ["full", "5y"], ["raw", "yoy"]);
    expect(decoded).toEqual({ range: "full", transform: "raw" });
  });
});

// series-transformations spec, "Range presets" — the "personalizado" entry
// (verify-report WARNING-5). "The selected preset MUST be encoded in the
// permalink" applies to the custom selection exactly as it does to the five
// fixed ones, except that the custom selection must carry its own bounds:
// `?range=custom&from=2010-Q1&to=2015-Q4`.
//
// The bounds travel as PERIOD LABELS, not as the calendar dates the reader's
// date inputs speak. Period labels are the unit the whole product computes
// in, they round-trip through `periodOrdinalIndex` exactly, and they make the
// permalink state the same thing the chart is actually showing rather than a
// date that has to be re-snapped to a period on every read.
//
// This module stays a pure SERIALIZER: it never checks the bounds against the
// series' own span (it has no access to the series). `resolveCustomRange`
// does that, in the island, after decoding — so a hand-edited, out-of-span or
// garbage pair is a rendering decision, not a parsing one.
describe("encodeChartState / decodeChartState — the custom range", () => {
  const AVAILABLE = ["full", "5y", "custom"] as const;

  it("round-trips a custom range's own from/to bounds", () => {
    const search = encodeChartState({ range: "custom", transform: "raw", custom: { from: "2010-Q1", to: "2015-Q4" } });
    expect(search).toContain("range=custom");
    expect(search).toContain("from=2010-Q1");
    expect(search).toContain("to=2015-Q4");
    expect(decodeChartState(search, AVAILABLE, ["raw", "yoy"])).toEqual({
      range: "custom",
      transform: "raw",
      custom: { from: "2010-Q1", to: "2015-Q4" },
    });
  });

  it("round-trips a custom range alongside an active transformation", () => {
    const search = encodeChartState({ range: "custom", transform: "yoy", custom: { from: "2010-Q1", to: "2015-Q4" } });
    expect(decodeChartState(search, AVAILABLE, ["raw", "yoy"])).toEqual({
      range: "custom",
      transform: "yoy",
      custom: { from: "2010-Q1", to: "2015-Q4" },
    });
  });

  it("never emits from/to for a fixed preset — they would describe nothing", () => {
    const search = encodeChartState({ range: "5y", transform: "raw", custom: { from: "2010-Q1", to: "2015-Q4" } });
    expect(search).toContain("range=5y");
    expect(search).not.toContain("from=");
    expect(search).not.toContain("to=");
  });

  it("degrades a custom range with no bounds to the default range rather than emitting an unusable one", () => {
    expect(encodeChartState({ range: "custom", transform: "raw", custom: null })).toBe("");
  });

  it("degrades range=custom with a missing bound to the default range on decode", () => {
    expect(decodeChartState("?range=custom&from=2010-Q1", AVAILABLE, ["raw"])).toEqual({
      range: "full",
      transform: "raw",
    });
    expect(decodeChartState("?range=custom", AVAILABLE, ["raw"])).toEqual({ range: "full", transform: "raw" });
  });

  it("ignores range=custom entirely when the caller does not offer a custom range", () => {
    // Same rule the fixed presets already follow: a selection the component
    // is not currently offering falls back to the default rather than
    // producing a view no control on the page can explain.
    expect(decodeChartState("?range=custom&from=2010-Q1&to=2015-Q4", ["full", "5y"], ["raw"])).toEqual({
      range: "full",
      transform: "raw",
    });
  });

  it("passes garbage bounds through verbatim for the island to reject — parsing is not validation", () => {
    // Deliberate: this module cannot tell an out-of-span pair from an
    // in-span one, so it must not pretend to. It hands back what it read;
    // `resolveCustomRange` is the single place that judges it.
    expect(decodeChartState("?range=custom&from=banana&to=2015-Q4", AVAILABLE, ["raw"])).toEqual({
      range: "custom",
      transform: "raw",
      custom: { from: "banana", to: "2015-Q4" },
    });
  });

  it("leaves the default-state permalink clean — no custom params on a default view", () => {
    expect(encodeChartState({ range: "full", transform: "raw", custom: null })).toBe("");
  });
});
