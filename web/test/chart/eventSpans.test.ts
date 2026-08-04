// The FOURTH visual treatment the chart carries, and the first one that is
// about an INTERVAL rather than about an instant or about one observation.
//
// What the four now say, and why a reader can still tell them apart:
//
//   dotted grey ALONG THE DATA PATH + diamond  "this observation is
//                                               provisional" (RESERVED)
//   filled translucent VERTICAL COLUMN         "the series changed
//                                               methodology here"
//   thin solid VERTICAL rule + triangle flag   "a different government took
//                                               office here"
//   solid HORIZONTAL rail + end serifs         "this editorial event covers
//                                               these periods" (this module)
//
// The separations are structural, never chromatic: the rail is a STROKE where
// the band is a FILL, it is HORIZONTAL where the government rule is VERTICAL,
// and it is drawn ACROSS THE TOP of the plot where the provisional dash is ON
// the data path. Discard colour entirely and all four still read apart.
//
// This file pins the positioning rules; `svg.test.ts` pins what is drawn.
import { describe, expect, it } from "vitest";
import { DEFAULT_DIMENSIONS, plotArea, xForIndex } from "../../src/lib/chart/geometry";
import {
  buildEventSpanRails,
  selectEventSpans,
  type EventSpanAnnotation,
} from "../../src/lib/chart/eventSpans";

/** A contiguous quarterly window, 2010-Q1..2019-Q4 — long enough that a span
 * can sit wholly inside it, cross either edge, or miss it entirely. */
const PERIODS = Array.from({ length: 40 }, (_, i) => `${2010 + Math.floor(i / 4)}-Q${(i % 4) + 1}`);

function annotation(overrides: Partial<EventSpanAnnotation> & { id: string }): EventSpanAnnotation {
  return {
    group: "exogenous",
    name: overrides.id,
    dateStart: "2012-01-01",
    dateEnd: "2013-12-31",
    ...overrides,
  };
}

describe("selectEventSpans: which editorial events a chart may honestly project", () => {
  it("projects an event that falls wholly inside the plotted window, on its own first and last periods", () => {
    const [span] = selectEventSpans(
      [annotation({ id: "crisis", dateStart: "2012-01-01", dateEnd: "2013-12-31" })],
      PERIODS,
      "Q",
    );
    expect(span).toBeDefined();
    expect(span.startPeriod).toBe("2012-Q1");
    expect(span.endPeriod).toBe("2013-Q4");
    // Nothing was clamped, so BOTH ends are genuine boundaries the drawing may
    // cap with a serif.
    expect(span.clampedStart).toBe(false);
    expect(span.clampedEnd).toBe(false);
  });

  it("clamps an event that begins before the window to the first plotted period, and records that it did", () => {
    // The honest reading of the 2008-2013 financial crisis on a series that
    // begins in 2010: the overlap is real and must be drawn, but the drawing
    // must not claim the event began in 2010. `clampedStart` is what the
    // renderer reads to leave that end UNCAPPED.
    const [span] = selectEventSpans(
      [annotation({ id: "crisis", dateStart: "2008-01-01", dateEnd: "2013-12-31" })],
      PERIODS,
      "Q",
    );
    expect(span.startPeriod).toBe("2010-Q1");
    expect(span.endPeriod).toBe("2013-Q4");
    expect(span.clampedStart).toBe(true);
    expect(span.clampedEnd).toBe(false);
  });

  it("clamps an event that outlives the window to the last plotted period, and records that it did", () => {
    const [span] = selectEventSpans(
      [annotation({ id: "late", dateStart: "2018-01-01", dateEnd: "2024-12-31" })],
      PERIODS,
      "Q",
    );
    expect(span.startPeriod).toBe("2018-Q1");
    expect(span.endPeriod).toBe("2019-Q4");
    expect(span.clampedStart).toBe(false);
    expect(span.clampedEnd).toBe(true);
  });

  it("never projects an event that falls entirely outside the plotted window", () => {
    // The rule the change-of-government markers established, applied to an
    // interval: `nearestPeriodIndex` would happily snap a 2004 event onto
    // 2010-Q1 and assert a shock that this chart never saw.
    expect(
      selectEventSpans([annotation({ id: "old", dateStart: "2004-01-01", dateEnd: "2006-12-31" })], PERIODS, "Q"),
    ).toEqual([]);
    expect(
      selectEventSpans([annotation({ id: "future", dateStart: "2030-01-01", dateEnd: "2031-12-31" })], PERIODS, "Q"),
    ).toEqual([]);
  });

  it("never projects an event the registry gives no end date, because it has no period to cover", () => {
    // `shock-energetico-2022` and `ngeu-primer-desembolso` are the real cases.
    // Running the rail to the series' last observation would invent an end the
    // registry does not record; capping it at the start period would assert the
    // event lasted one quarter. Both are claims the data does not support, so
    // nothing is drawn and the chip below the chart — which prints a single
    // year rather than a range — remains the whole statement.
    expect(
      selectEventSpans([annotation({ id: "open", dateStart: "2015-02-24", dateEnd: null })], PERIODS, "Q"),
    ).toEqual([]);
    expect(
      selectEventSpans([annotation({ id: "absent", dateStart: "2015-02-24", dateEnd: undefined })], PERIODS, "Q"),
    ).toEqual([]);
  });

  it("stacks two overlapping events into separate lanes, so neither hides the other", () => {
    const spans = selectEventSpans(
      [
        annotation({ id: "b", dateStart: "2013-01-01", dateEnd: "2016-12-31" }),
        annotation({ id: "a", dateStart: "2012-01-01", dateEnd: "2014-12-31" }),
      ],
      PERIODS,
      "Q",
    );
    // Oldest first, whatever order the artifact delivered them in.
    expect(spans.map((s) => s.id)).toEqual(["a", "b"]);
    expect(spans.map((s) => s.lane)).toEqual([0, 1]);
  });

  it("keeps two events that do NOT overlap in the same lane, so the common case stays one rail deep", () => {
    // The real shape of /indicador/tasa-de-paro-epa's exogenous group: the
    // 2008-2013 crisis and the 2020-2021 pandemic never touch, so stacking them
    // would spend vertical space on a collision that does not exist.
    const spans = selectEventSpans(
      [
        annotation({ id: "first", dateStart: "2011-01-01", dateEnd: "2012-12-31" }),
        annotation({ id: "second", dateStart: "2015-01-01", dateEnd: "2016-12-31" }),
      ],
      PERIODS,
      "Q",
    );
    expect(spans.map((s) => s.lane)).toEqual([0, 0]);
  });

  it("treats two spans meeting on one period as overlapping, because they share a plotted point", () => {
    const spans = selectEventSpans(
      [
        annotation({ id: "first", dateStart: "2011-01-01", dateEnd: "2013-03-31" }),
        annotation({ id: "second", dateStart: "2013-01-01", dateEnd: "2015-12-31" }),
      ],
      PERIODS,
      "Q",
    );
    expect(spans.map((s) => s.lane)).toEqual([0, 1]);
  });

  it("refuses an event whose end predates its start rather than drawing it backwards", () => {
    // A data defect, not a rendering decision: fail closed (P4) rather than
    // silently swap the bounds and present a span nobody configured.
    expect(
      selectEventSpans([annotation({ id: "inverted", dateStart: "2015-01-01", dateEnd: "2012-12-31" })], PERIODS, "Q"),
    ).toEqual([]);
  });

  it("projects nothing at all onto a chart with no plotted periods", () => {
    expect(selectEventSpans([annotation({ id: "any" })], [], "Q")).toEqual([]);
  });

  it("carries the entry's own group through, so the drawing can be styled and queried per group", () => {
    const [span] = selectEventSpans([annotation({ id: "hito", group: "milestones" })], PERIODS, "Q");
    expect(span.group).toBe("milestones");
    expect(span.name).toBe("hito");
  });
});

describe("buildEventSpanRails: where the rail is drawn", () => {
  const area = plotArea(DEFAULT_DIMENSIONS);

  it("runs from the x of its first covered period to the x of its last", () => {
    const [rail] = buildEventSpanRails(
      [annotation({ id: "crisis", dateStart: "2012-01-01", dateEnd: "2013-12-31" })],
      PERIODS,
      "Q",
      DEFAULT_DIMENSIONS,
      10,
    );
    expect(rail.x1).toBeCloseTo(xForIndex(PERIODS.indexOf("2012-Q1"), PERIODS.length, DEFAULT_DIMENSIONS), 5);
    expect(rail.x2).toBeCloseTo(xForIndex(PERIODS.indexOf("2013-Q4"), PERIODS.length, DEFAULT_DIMENSIONS), 5);
  });

  it("is HORIZONTAL and sits inside the plot area, near its top edge", () => {
    // The whole separation from the change-of-government rule is orientation:
    // that one is vertical and full height, this one is horizontal and clear of
    // the data it annotates.
    const [rail] = buildEventSpanRails([annotation({ id: "crisis" })], PERIODS, "Q", DEFAULT_DIMENSIONS, 10);
    expect(rail.y).toBeGreaterThan(area.y0);
    expect(rail.y).toBeLessThan(area.y0 + area.height / 4);
    expect(rail.x2).toBeGreaterThan(rail.x1);
  });

  it("gives a stacked lane its own y, so overlapping rails never coincide", () => {
    const rails = buildEventSpanRails(
      [
        annotation({ id: "a", dateStart: "2012-01-01", dateEnd: "2014-12-31" }),
        annotation({ id: "b", dateStart: "2013-01-01", dateEnd: "2016-12-31" }),
      ],
      PERIODS,
      "Q",
      DEFAULT_DIMENSIONS,
      10,
    );
    expect(rails[1].y).toBeGreaterThan(rails[0].y);
  });

  it("gives an event that snaps onto a single period one period step of width, never zero", () => {
    // Otherwise the rail collapses to a point and the reader sees nothing at
    // all. One step is the width of the period the event falls in, at this
    // axis' own resolution — not an invented extent.
    const [rail] = buildEventSpanRails(
      [annotation({ id: "brief", dateStart: "2012-02-01", dateEnd: "2012-03-15" })],
      PERIODS,
      "Q",
      DEFAULT_DIMENSIONS,
      10,
    );
    const step = area.width / (PERIODS.length - 1);
    expect(rail.x2 - rail.x1).toBeCloseTo(step, 5);
  });

  it("never draws outside the plot area, even for a span clamped at both edges", () => {
    const [rail] = buildEventSpanRails(
      [annotation({ id: "everything", dateStart: "1999-01-01", dateEnd: "2099-12-31" })],
      PERIODS,
      "Q",
      DEFAULT_DIMENSIONS,
      10,
    );
    expect(rail.x1).toBeGreaterThanOrEqual(area.x0);
    expect(rail.x2).toBeLessThanOrEqual(area.x1);
  });

  it("scales with the tick type size, so the phone-shaped box gets a proportionate rail", () => {
    const wide = buildEventSpanRails([annotation({ id: "a" })], PERIODS, "Q", DEFAULT_DIMENSIONS, 10)[0];
    const narrow = buildEventSpanRails([annotation({ id: "a" })], PERIODS, "Q", DEFAULT_DIMENSIONS, 20)[0];
    expect(narrow.strokeWidth).toBeGreaterThan(wide.strokeWidth);
    expect(narrow.serifLength).toBeGreaterThan(wide.serifLength);
  });
});
