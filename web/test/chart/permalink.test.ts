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

// The government range (indicator-page spec, "Annotation layers per PRD
// §6.1.1(a)" — the `governments` event group, read together with
// series-transformations spec's "The selected preset MUST be encoded in the
// permalink").
//
// A government selection carries a payload exactly as the custom range does —
// but ONE identifier, not a pair of bounds, and that identifier is the
// editorial registry's own event id (`gobierno-rajoy-2011`). The id is the
// right thing to encode rather than the derived `[from, to]` window, because
// the window is INFERRED from the succession: encoding the window would freeze
// today's inference into every shared link, so a later registry correction (a
// confirmed end date, a government finally recorded) would leave old links
// pointing at a span nobody would derive again. The id survives that
// correction; the window is re-derived on every load.
//
// It is also a machine surface, and stays one: the raw config id, never the
// reader-facing name and never a formatted year range.
describe("encodeChartState / decodeChartState — the government range", () => {
  const AVAILABLE = ["full", "5y", "custom", "government"] as const;

  it("round-trips a government selection by its editorial id", () => {
    const search = encodeChartState({ range: "government", transform: "raw", government: "gobierno-rajoy-2011" });
    expect(search).toContain("range=government");
    expect(search).toContain("government=gobierno-rajoy-2011");
    expect(decodeChartState(search, AVAILABLE, ["raw", "yoy"])).toEqual({
      range: "government",
      transform: "raw",
      government: "gobierno-rajoy-2011",
    });
  });

  it("round-trips a government selection alongside an active transformation", () => {
    const search = encodeChartState({ range: "government", transform: "yoy", government: "gobierno-sanchez-2018" });
    expect(decodeChartState(search, AVAILABLE, ["raw", "yoy"])).toEqual({
      range: "government",
      transform: "yoy",
      government: "gobierno-sanchez-2018",
    });
  });

  it("encodes the id verbatim — a permalink parameter is a machine surface, never display copy", () => {
    const search = encodeChartState({ range: "government", transform: "raw", government: "gobierno-gonzalez-1982" });
    // The reader-facing rendering of this same selection is "Felipe González
    // (1982–1996)". None of that may leak into the parameter that is parsed
    // back, exactly as the custom range encodes "2010-Q1" and not "T1 2010".
    expect(search).not.toContain("Felipe");
    expect(search).not.toContain("1982%E2%80%931996");
    expect(new URLSearchParams(search.slice(1)).get("government")).toBe("gobierno-gonzalez-1982");
  });

  it("never emits a government id for any other range — it would describe nothing", () => {
    const search = encodeChartState({ range: "5y", transform: "raw", government: "gobierno-rajoy-2011" });
    expect(search).toContain("range=5y");
    expect(search).not.toContain("government=");
  });

  it("degrades range=government with no id to the default range, on both sides", () => {
    expect(encodeChartState({ range: "government", transform: "raw", government: null })).toBe("");
    expect(decodeChartState("?range=government", AVAILABLE, ["raw"])).toEqual({ range: "full", transform: "raw" });
  });

  it("ignores range=government entirely when the caller offers no government range", () => {
    // Same gate the fixed presets and the custom range already pass through:
    // a series whose span overlaps no selectable government offers no
    // government control, so a URL cannot select one.
    expect(decodeChartState("?range=government&government=gobierno-rajoy-2011", ["full", "5y"], ["raw"])).toEqual({
      range: "full",
      transform: "raw",
    });
  });

  it("passes an unknown id through verbatim for the island to judge — parsing is not validation", () => {
    // This module has no access to the series, so it cannot know which
    // governments overlap it. `availableGovernmentTerms` is the single place
    // that judges; the island degrades to the full range when it refuses.
    expect(decodeChartState("?range=government&government=banana", AVAILABLE, ["raw"])).toEqual({
      range: "government",
      transform: "raw",
      government: "banana",
    });
  });

  it("keeps the custom range and the government range from contaminating each other", () => {
    const search = encodeChartState({
      range: "government",
      transform: "raw",
      government: "gobierno-rajoy-2011",
      custom: { from: "2010-Q1", to: "2015-Q4" },
    });
    expect(search).not.toContain("from=");
    expect(search).not.toContain("to=");
    expect(decodeChartState("?range=custom&from=2010-Q1&to=2015-Q4&government=gobierno-rajoy-2011", AVAILABLE, ["raw"])).toEqual({
      range: "custom",
      transform: "raw",
      custom: { from: "2010-Q1", to: "2015-Q4" },
    });
  });
});
