// series-transformations spec, "Range presets" — "A preset whose start
// precedes the series' first observation MUST be absent, not disabled".
// Pure-function, red-first.
import { describe, expect, it } from "vitest";
import {
  CUSTOM_RANGE,
  RANGE_PRESETS,
  availablePresets,
  isPresetAvailable,
  periodStartCalendarDate,
  resolveCustomRange,
  sliceCustomRange,
  sliceRange,
} from "../../src/lib/transform/sliceRange";

describe("isPresetAvailable / availablePresets", () => {
  it("full is always available", () => {
    expect(isPresetAvailable("full", "Q", "2010-Q1", "2026-Q1")).toBe(true);
  });

  it("an absolute preset earlier than the series' first observation is unavailable", () => {
    // Series starts 2010 — "desde 2008" would claim data before it exists.
    expect(isPresetAvailable("since-2008", "Q", "2010-Q1", "2026-Q1")).toBe(false);
  });

  it("an absolute preset at or after the series' start is available", () => {
    expect(isPresetAvailable("since-2018", "Q", "2010-Q1", "2026-Q1")).toBe(true);
  });

  it("a relative preset (5/10 años) unavailable when the series itself is shorter", () => {
    expect(isPresetAvailable("10y", "Q", "2020-Q1", "2021-Q1")).toBe(false);
  });

  it("availablePresets never includes an unavailable preset", () => {
    const presets = availablePresets("Q", "2010-Q1", "2026-Q1");
    expect(presets).not.toContain("since-2008");
    expect(presets).toContain("full");
    expect(presets).toContain("since-2018");
  });

  // verify-report WARNING-5's secondary fault: this test's ORIGINAL title
  // ("RANGE_PRESETS names exactly the five spec-mandated presets") claimed a
  // spec conformance the array did not have. The spec's own list is "the full
  // series as the default range plus these presets: 5 años, 10 años, desde
  // 2008, desde 2018, personalizado" — six selections, of which
  // `RANGE_PRESETS` models five and "personalizado" is deliberately NOT one
  // of them: a custom range carries its own `from`/`to` pair, so it cannot be
  // a bare member of an enum whose every member is fully determined by the
  // series' own span. It is modelled as `CUSTOM_RANGE`, a separate selection
  // alongside the preset enum, and the assertions below now say exactly that.
  it("RANGE_PRESETS names the full-series default plus the four fixed presets, and never the custom selection", () => {
    expect(RANGE_PRESETS).toEqual(["full", "5y", "10y", "since-2008", "since-2018"]);
    expect(RANGE_PRESETS as readonly string[]).not.toContain(CUSTOM_RANGE);
  });

  it("the custom selection is not span-gated the way a fixed preset is — it has no fixed start to compare", () => {
    // `availablePresets` filters by "does this preset's own start precede the
    // series' first observation" (the spec's absence rule). A custom range
    // has no such start until the reader enters one, so it can never appear
    // in — nor be filtered out of — this list. Its own equivalent of the
    // absence rule is `resolveCustomRange` below.
    expect(availablePresets("Q", "1971-Q1", "2026-Q1") as readonly string[]).not.toContain(CUSTOM_RANGE);
  });
});

describe("sliceRange", () => {
  const points = [
    { period: "2015-Q1" },
    { period: "2016-Q1" },
    { period: "2020-Q1" },
    { period: "2025-Q1" },
    { period: "2026-Q1" },
  ];

  it("full returns every point unchanged", () => {
    expect(sliceRange(points, "Q", "full")).toEqual(points);
  });

  it("5y keeps only points within 5 years of the series' own latest point", () => {
    const result = sliceRange(points, "Q", "5y");
    expect(result.map((p) => p.period)).toEqual(["2025-Q1", "2026-Q1"]);
  });

  it("since-2018 keeps only points at or after 2018", () => {
    const result = sliceRange(points, "Q", "since-2018");
    expect(result.map((p) => p.period)).toEqual(["2020-Q1", "2025-Q1", "2026-Q1"]);
  });
});

describe("sliceCustomRange", () => {
  it("keeps only points within the inclusive [from, to] bounds", () => {
    const points = [{ period: "2018-Q1" }, { period: "2020-Q1" }, { period: "2022-Q1" }];
    const result = sliceCustomRange(points, "Q", "2019-Q1", "2021-Q1");
    expect(result.map((p) => p.period)).toEqual(["2020-Q1"]);
  });
});

// series-transformations spec, "Range presets": the "personalizado" entry —
// verify-report WARNING-5. The spec fixes the absence rule for a preset whose
// start precedes the series' first observation ("absent, not disabled"); a
// reader-entered range has no fixed start to check at render time, so the
// equivalent honest rule has to be decided here and is decided as follows:
//
//   - PARTLY outside the span  -> clamp to the span and report `clamped:
//     true`, so the caller can DISCLOSE the narrowing. Clamping is the only
//     renderable outcome (the missing years do not exist), and an
//     undisclosed clamp would be the free-form equivalent of a disabled
//     preset silently lying about what it shows.
//   - WHOLLY outside the span, or overlapping only a gap in the series'
//     own cadence, so that not one observation falls inside it -> rejected.
//     This is the direct analogue of "absent, not disabled": we do not
//     render a view of nothing.
//   - Inverted (from after to) or blank/garbage -> rejected, since there is
//     no defensible view to render at all.
describe("resolveCustomRange", () => {
  // A deliberately GAPPY quarterly series: 2015-Q2 and 2016-Q2..Q4 are
  // missing, mirroring `poblacion-residente`'s real semiannual historical
  // cadence segment (config/series/poblacion-residente.yaml).
  const PERIODS = ["2015-Q1", "2015-Q3", "2015-Q4", "2016-Q1", "2017-Q1", "2018-Q1"];

  it("accepts a range wholly inside the series' span, unclamped", () => {
    expect(resolveCustomRange(PERIODS, "Q", "2015-Q3", "2017-Q1")).toEqual({
      status: "ok",
      from: "2015-Q3",
      to: "2017-Q1",
      clamped: false,
    });
  });

  it("clamps a range that starts before the series' first observation and says so", () => {
    expect(resolveCustomRange(PERIODS, "Q", "2008-Q1", "2016-Q1")).toEqual({
      status: "ok",
      from: "2015-Q1",
      to: "2016-Q1",
      clamped: true,
    });
  });

  it("clamps a range that ends after the series' last observation and says so", () => {
    expect(resolveCustomRange(PERIODS, "Q", "2017-Q1", "2030-Q4")).toEqual({
      status: "ok",
      from: "2017-Q1",
      to: "2018-Q1",
      clamped: true,
    });
  });

  it("rejects a range wholly before the series' span rather than rendering an empty chart", () => {
    expect(resolveCustomRange(PERIODS, "Q", "2000-Q1", "2010-Q4")).toEqual({
      status: "rejected",
      reason: "no-observations",
    });
  });

  it("rejects a range wholly after the series' span", () => {
    expect(resolveCustomRange(PERIODS, "Q", "2020-Q1", "2026-Q4")).toEqual({
      status: "rejected",
      reason: "no-observations",
    });
  });

  it("rejects a range that falls entirely inside a gap in the series' own cadence", () => {
    // 2016-Q2..2016-Q4 are inside [first, last] but hold no observation at
    // all — clamping cannot help, and slicing would yield an empty chart.
    expect(resolveCustomRange(PERIODS, "Q", "2016-Q2", "2016-Q4")).toEqual({
      status: "rejected",
      reason: "no-observations",
    });
  });

  it("rejects an inverted range", () => {
    expect(resolveCustomRange(PERIODS, "Q", "2017-Q1", "2015-Q1")).toEqual({
      status: "rejected",
      reason: "inverted",
    });
  });

  it("rejects a blank bound as incomplete rather than as garbage", () => {
    expect(resolveCustomRange(PERIODS, "Q", "", "2017-Q1")).toEqual({ status: "rejected", reason: "incomplete" });
    expect(resolveCustomRange(PERIODS, "Q", "2015-Q1", "")).toEqual({ status: "rejected", reason: "incomplete" });
  });

  it("rejects an unparseable bound instead of throwing (a hand-edited permalink is untrusted input)", () => {
    // `parsePeriod` throws by design on a label that does not match the
    // frequency — correct for internal data, wrong for a query string a
    // reader can type anything into.
    expect(() => resolveCustomRange(PERIODS, "Q", "banana", "2017-Q1")).not.toThrow();
    expect(resolveCustomRange(PERIODS, "Q", "banana", "2017-Q1")).toEqual({
      status: "rejected",
      reason: "unparseable",
    });
    expect(resolveCustomRange(PERIODS, "Q", "2015-01", "2017-Q1")).toEqual({
      status: "rejected",
      reason: "unparseable",
    });
  });

  it("rejects any range against an empty series", () => {
    expect(resolveCustomRange([], "Q", "2015-Q1", "2017-Q1")).toEqual({
      status: "rejected",
      reason: "no-observations",
    });
  });

  it("a single-period range naming a real observation is accepted", () => {
    expect(resolveCustomRange(PERIODS, "Q", "2016-Q1", "2016-Q1")).toEqual({
      status: "ok",
      from: "2016-Q1",
      to: "2016-Q1",
      clamped: false,
    });
  });

  it("an accepted resolution always slices to at least one point", () => {
    // The contract the whole rejection branch exists to guarantee: a caller
    // that feeds an "ok" resolution straight into `sliceCustomRange` can
    // never end up with an empty chart.
    const points = PERIODS.map((period) => ({ period }));
    for (const [from, to] of [
      ["2008-Q1", "2030-Q4"],
      ["2015-Q3", "2017-Q1"],
      ["2016-Q1", "2016-Q1"],
    ] as const) {
      const resolution = resolveCustomRange(PERIODS, "Q", from, to);
      expect(resolution.status).toBe("ok");
      if (resolution.status !== "ok") continue;
      expect(sliceCustomRange(points, "Q", resolution.from, resolution.to).length).toBeGreaterThan(0);
    }
  });

  it("works for monthly and annual series, not only quarterly", () => {
    expect(resolveCustomRange(["2020-01", "2020-06", "2021-01"], "M", "2019-01", "2020-06")).toEqual({
      status: "ok",
      from: "2020-01",
      to: "2020-06",
      clamped: true,
    });
    expect(resolveCustomRange(["2019", "2020", "2021"], "A", "2020", "2021")).toEqual({
      status: "ok",
      from: "2020",
      to: "2021",
      clamped: false,
    });
  });
});

// The bridge between the reader's two native date inputs (which speak
// calendar dates) and the series' own period labels (which everything else
// in this module speaks). `periodFromCalendarDate` in `lib/chart/periods`
// already owns the date -> period direction; this is the inverse, needed to
// PREFILL the inputs when a custom permalink is reopened.
describe("periodStartCalendarDate", () => {
  it("maps a quarterly period label to the first calendar day of that quarter", () => {
    expect(periodStartCalendarDate("2015-Q1", "Q")).toBe("2015-01-01");
    expect(periodStartCalendarDate("2015-Q2", "Q")).toBe("2015-04-01");
    expect(periodStartCalendarDate("2015-Q4", "Q")).toBe("2015-10-01");
  });

  it("maps a monthly period label to the first calendar day of that month", () => {
    expect(periodStartCalendarDate("2015-03", "M")).toBe("2015-03-01");
    expect(periodStartCalendarDate("2015-12", "M")).toBe("2015-12-01");
  });

  it("maps an annual period label to 1 January", () => {
    expect(periodStartCalendarDate("2015", "A")).toBe("2015-01-01");
  });
});
