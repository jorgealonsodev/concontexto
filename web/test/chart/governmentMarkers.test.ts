// Positioning of the government-change markers (PRD §6.1.1(a)'s `governments`
// event group, drawn ON the chart rather than only listed beside it).
//
// The two facts this file exists to pin, because both are ways the marker can
// LIE about the data:
//
//   1. A marker is drawn only when the investiture genuinely falls inside the
//      series' own observed span. `nearestPeriodIndex` snaps ANY date onto the
//      nearest plotted period, including a date decades outside the series —
//      so an unfiltered Aznar (1996) over a 2002-2026 series would draw a line
//      at 2002-Q1 asserting a change of government that did not happen there.
//   2. Inside the span, the marker sits on the nearest ACTUALLY PLOTTED period
//      — the same snap `buildBreakBands` already uses — never on an
//      interpolated x for a period the series never observed.
import { describe, expect, it } from "vitest";
import {
  buildGovernmentMarkers,
  selectGovernmentChanges,
  type GovernmentChangeAnnotation,
} from "../../src/lib/chart/governmentMarkers";
import { DEFAULT_DIMENSIONS, xForIndex } from "../../src/lib/chart/geometry";

/** The real registry, minus Suárez — whose `date_status: unconfirmed` means he
 * is never projected to the artifact and therefore never reaches this module.
 * Deliberately the real dates: the positioning rules below are only meaningful
 * against the succession this product actually carries. */
const REGISTRY: GovernmentChangeAnnotation[] = [
  { id: "gobierno-calvo-sotelo-1981", group: "governments", name: "Leopoldo Calvo-Sotelo", dateStart: "1981-02-25" },
  { id: "gobierno-gonzalez-1982", group: "governments", name: "Felipe González", dateStart: "1982-12-02" },
  { id: "gobierno-aznar-1996", group: "governments", name: "José María Aznar", dateStart: "1996-05-05" },
  { id: "gobierno-zapatero-2004", group: "governments", name: "José Luis Rodríguez Zapatero", dateStart: "2004-04-17" },
  { id: "gobierno-rajoy-2011", group: "governments", name: "Mariano Rajoy", dateStart: "2011-12-21" },
  { id: "gobierno-sanchez-2018", group: "governments", name: "Pedro Sánchez", dateStart: "2018-06-02" },
];

/** Consecutive quarterly labels, oldest first — the cadence four of the six
 * shipped series use. */
function quarters(fromYear: number, toYear: number): string[] {
  const periods: string[] = [];
  for (let year = fromYear; year <= toYear; year++) {
    for (let q = 1; q <= 4; q++) periods.push(`${year}-Q${q}`);
  }
  return periods;
}

describe("selectGovernmentChanges — which investitures the chart may mark", () => {
  it("marks every investiture that falls inside the series' own span", () => {
    // `tasa-de-paro-epa`'s real span. Four governments OVERLAP it (Aznar's
    // term was still running in 2002), but only three CHANGES happen inside
    // it.
    const changes = selectGovernmentChanges(REGISTRY, quarters(2002, 2026), "Q");
    expect(changes.map((c) => c.id)).toEqual([
      "gobierno-zapatero-2004",
      "gobierno-rajoy-2011",
      "gobierno-sanchez-2018",
    ]);
  });

  it("never marks an investiture that predates the first observation, however close it is", () => {
    // THE LIE THIS RULE PREVENTS. Aznar took office in 1996; the series starts
    // in 2002. `nearestPeriodIndex` would happily snap 1996-Q2 onto 2002-Q1 —
    // drawing a change of government at the left edge of a chart on which no
    // change of government happened.
    const changes = selectGovernmentChanges(REGISTRY, quarters(2002, 2003), "Q");
    expect(changes.map((c) => c.id)).toEqual([]);
  });

  it("never marks an investiture that postdates the last observation", () => {
    const changes = selectGovernmentChanges(REGISTRY, quarters(2012, 2017), "Q");
    expect(changes.map((c) => c.id)).toEqual([]);
  });

  it("marks nothing at all for a series that predates every government in the registry", () => {
    // `poblacion-residente` begins in 1971 and the earliest PROJECTED
    // government is Calvo-Sotelo (1981). Suárez (1976) carries
    // `date_status: unconfirmed`, is never projected, and must never be
    // invented here — so the 1971-1980 stretch is honestly unmarked rather
    // than given a marker for a date this product does not claim to know.
    expect(selectGovernmentChanges(REGISTRY, quarters(1971, 1980), "Q")).toEqual([]);
  });

  it("marks exactly one change for a series overlapping exactly one investiture", () => {
    const changes = selectGovernmentChanges(REGISTRY, quarters(2003, 2006), "Q");
    expect(changes.map((c) => c.id)).toEqual(["gobierno-zapatero-2004"]);
    expect(changes[0].period).toBe("2004-Q2");
    expect(changes[0].year).toBe("2004");
    expect(changes[0].name).toBe("José Luis Rodríguez Zapatero");
  });

  it("marks every one of the six for a series that spans the whole registry", () => {
    const changes = selectGovernmentChanges(REGISTRY, quarters(1971, 2026), "Q");
    expect(changes).toHaveLength(6);
  });

  it("snaps onto the nearest ACTUALLY PLOTTED period when the investiture's own period was never observed", () => {
    // `poblacion-residente`'s historical cadence is semiannual: Q1 and Q3
    // only. Sánchez's investiture lands in 2018-Q2, which the series never
    // observed, so the marker belongs on the nearest period it did — the same
    // snap a break band already gets, never an invented x between two points.
    const semiannual = ["2017-Q1", "2017-Q3", "2018-Q1", "2018-Q3", "2019-Q1"];
    const changes = selectGovernmentChanges(REGISTRY, semiannual, "Q");
    expect(changes.map((c) => c.period)).toEqual(["2018-Q1"]);
    expect(changes[0].index).toBe(2);
  });

  it("orders the changes chronologically however the annotations arrive", () => {
    // The artifact's `events` array carries no ordering contract, and the
    // description sentence below the chart reads left to right — an unsorted
    // list would name the markers in an order the drawing does not use.
    const shuffled = [REGISTRY[5], REGISTRY[3], REGISTRY[4]];
    const changes = selectGovernmentChanges(shuffled, quarters(2002, 2026), "Q");
    expect(changes.map((c) => c.year)).toEqual(["2004", "2011", "2018"]);
  });

  it("ignores every annotation that is not a government", () => {
    // A shock or a milestone falling between two investitures must never be
    // drawn as a change of government.
    const mixed: GovernmentChangeAnnotation[] = [
      { id: "crisis-2008", group: "exogenous", name: "Crisis financiera", dateStart: "2008-01-01" },
      { id: "reforma-2012", group: "milestones", name: "Reforma laboral", dateStart: "2012-02-01" },
    ];
    expect(selectGovernmentChanges(mixed, quarters(2002, 2026), "Q")).toEqual([]);
  });

  it("returns nothing for a series with no observations at all", () => {
    expect(selectGovernmentChanges(REGISTRY, [], "Q")).toEqual([]);
  });

  it("marks an investiture landing exactly on the first or the last observation", () => {
    // Inclusive at both edges, and deliberately so: under the government
    // filter the selected term's own window BEGINS at the investiture, so the
    // marker at the left edge is exactly what tells the reader where the
    // window they chose came from.
    const changes = selectGovernmentChanges(REGISTRY, quarters(2018, 2020), "Q");
    expect(changes.map((c) => c.id)).toEqual(["gobierno-sanchez-2018"]);
    expect(changes[0].index).toBe(1); // 2018-Q2, the second plotted quarter
  });
});

describe("buildGovernmentMarkers — where the rule is drawn", () => {
  it("puts each marker on the x of its own plotted period, exactly as the axis places it", () => {
    const periods = quarters(2002, 2026);
    const markers = buildGovernmentMarkers(REGISTRY, periods, "Q", DEFAULT_DIMENSIONS);
    expect(markers).toHaveLength(3);
    for (const marker of markers) {
      expect(marker.x).toBeCloseTo(xForIndex(marker.index, periods.length, DEFAULT_DIMENSIONS), 10);
    }
  });

  it("orders the markers left to right, so the description below reads in drawing order", () => {
    const markers = buildGovernmentMarkers(REGISTRY, quarters(2002, 2026), "Q", DEFAULT_DIMENSIONS);
    const xs = markers.map((m) => m.x);
    expect([...xs].sort((a, b) => a - b)).toEqual(xs);
  });

  it("draws nothing for a series with no observations", () => {
    expect(buildGovernmentMarkers(REGISTRY, [], "Q", DEFAULT_DIMENSIONS)).toEqual([]);
  });
});
