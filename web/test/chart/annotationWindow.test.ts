// The ONE window rule, tested where it now lives.
//
// Before this module the rule was written out once inside
// `selectGovernmentChanges` and again inside `selectEventSpans`, and a third
// consumer (the annotation chips below the chart) had none — which is exactly
// how the chips came to list a 2008-2013 crisis under a chart narrowed to
// 2018 onwards, and to list three pre-2002 investitures on a series beginning
// in 2002. These tests hold the extracted rule to the behaviour those
// selectors already had, so the refactor is provably a move rather than a
// rewrite; `governmentMarkers.test.ts` and `eventSpans.test.ts` continue to
// hold each selector to it from the other side.
import { describe, expect, it } from "vitest";
import {
  annotationInWindow,
  annotationsInWindow,
  instantOrdinalInWindow,
  intervalOrdinalsInWindow,
  periodWindow,
} from "../../src/lib/chart/annotationWindow";

const QUARTERS_2018_2021 = ["2018-Q1", "2018-Q2", "2019-Q1", "2020-Q1", "2021-Q1"];

describe("periodWindow", () => {
  it("spans the first and last period handed in, whatever lies between", () => {
    // Deliberately a NON-contiguous list: the window is the two ends of the
    // series on screen, not a count of the observations inside it. A gap in
    // the cadence must not narrow it.
    const w = periodWindow(QUARTERS_2018_2021, "Q");
    expect(w).not.toBeNull();
    expect(w!.firstOrdinal).toBeLessThan(w!.lastOrdinal);
  });

  it("is null for an empty series, so no annotation can be judged in range", () => {
    // The three selectors all return `[]` for no periods rather than
    // comparing against `undefined`; a null window is how that same refusal
    // travels to a caller that filters instead of mapping.
    expect(periodWindow([], "Q")).toBeNull();
  });
});

describe("instantOrdinalInWindow", () => {
  const w = { firstOrdinal: 10, lastOrdinal: 20 };

  it("includes both edges", () => {
    // Inclusive at both ends because the edge case is the POINT rather than a
    // rounding artefact: under the government filter the selected term begins
    // at its own investiture, and that left-edge marker is what shows the
    // reader where the window they chose came from
    // (`governmentMarkers.ts`'s own header).
    expect(instantOrdinalInWindow(10, w)).toBe(true);
    expect(instantOrdinalInWindow(20, w)).toBe(true);
  });

  it("excludes an instant on either side of the window", () => {
    expect(instantOrdinalInWindow(9, w)).toBe(false);
    expect(instantOrdinalInWindow(21, w)).toBe(false);
  });
});

describe("intervalOrdinalsInWindow", () => {
  const w = { firstOrdinal: 10, lastOrdinal: 20 };

  it("admits an interval that merely INTERSECTS, never requiring containment", () => {
    // `eventSpans.ts`'s own rule, and the reason it differs from the instant's:
    // the 2008-2013 crisis genuinely covers 2010-2013 of a series beginning in
    // 2010, so refusing to draw it would hide a real overlap.
    expect(intervalOrdinalsInWindow(0, 12, w)).toBe(true); // overlaps the left edge
    expect(intervalOrdinalsInWindow(18, 99, w)).toBe(true); // overlaps the right edge
    expect(intervalOrdinalsInWindow(0, 99, w)).toBe(true); // swallows the window whole
    expect(intervalOrdinalsInWindow(12, 14, w)).toBe(true); // contained
  });

  it("touching one edge is intersecting", () => {
    expect(intervalOrdinalsInWindow(0, 10, w)).toBe(true);
    expect(intervalOrdinalsInWindow(20, 99, w)).toBe(true);
  });

  it("rejects an interval that ends before or begins after the window", () => {
    expect(intervalOrdinalsInWindow(0, 9, w)).toBe(false);
    expect(intervalOrdinalsInWindow(21, 99, w)).toBe(false);
  });

  it("fails closed on an interval whose end predates its start", () => {
    // A registry entry with the bounds the wrong way round is refused rather
    // than silently swapped — swapping would admit a span nobody configured.
    expect(intervalOrdinalsInWindow(15, 12, w)).toBe(false);
  });
});

describe("annotationInWindow", () => {
  it("judges an entry with a start AND an end as an interval", () => {
    const crisis = { dateStart: "2008-01-01", dateEnd: "2013-12-31" };
    expect(annotationInWindow(crisis, ["2010-Q1", "2012-Q1"], "Q")).toBe(true);
    expect(annotationInWindow(crisis, QUARTERS_2018_2021, "Q")).toBe(false);
  });

  it("judges an entry with NO end date as an instant", () => {
    // `shock-energetico-2022` and `ngeu-primer-desembolso` are shaped like
    // this. The registry says the event began and says nothing about when it
    // stopped, so there is no interval to intersect — only a date that either
    // falls inside the span on screen or does not. `selectEventSpans` drops
    // these entirely (no end, no rail); the chip is the whole statement, and
    // it must still answer the window it is printed under.
    const shock = { dateStart: "2022-02-24", dateEnd: null };
    expect(annotationInWindow(shock, ["2022-Q1", "2023-Q1"], "Q")).toBe(true);
    expect(annotationInWindow(shock, QUARTERS_2018_2021, "Q")).toBe(false);
  });

  it("treats an absent dateEnd and an explicit null identically", () => {
    const withUndefined = { dateStart: "2021-12-30" };
    const withNull = { dateStart: "2021-12-30", dateEnd: null };
    expect(annotationInWindow(withUndefined, QUARTERS_2018_2021, "Q")).toBe(
      annotationInWindow(withNull, QUARTERS_2018_2021, "Q"),
    );
  });

  it("admits nothing at all when the series has no periods", () => {
    expect(annotationInWindow({ dateStart: "2020-01-01" }, [], "Q")).toBe(false);
  });
});

describe("annotationsInWindow", () => {
  it("keeps the entries the window covers, in the order handed in", () => {
    // Order is preserved rather than sorted: the chip row is rendered from
    // this list and the artifact's own order is what the group already
    // printed. Selecting is this function's whole job.
    const entries = [
      { id: "reforma-2012", group: "milestones", dateStart: "2012-02-10", dateEnd: null },
      { id: "covid-2020", group: "milestones", dateStart: "2020-03-17", dateEnd: null },
      { id: "reforma-2021", group: "milestones", dateStart: "2021-12-30", dateEnd: null },
    ];
    const kept = annotationsInWindow(entries, ["2018-Q1", "2020-Q1", "2021-Q4", "2026-Q2"], "Q");
    expect(kept.map((e) => e.id)).toEqual(["covid-2020", "reforma-2021"]);
  });

  it("returns nothing for an empty series rather than everything", () => {
    const entries = [{ id: "a", group: "milestones", dateStart: "2020-01-01", dateEnd: null }];
    expect(annotationsInWindow(entries, [], "Q")).toEqual([]);
  });
});
