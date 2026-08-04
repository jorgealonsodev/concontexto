// Positioning of the policy-measure marks — the `measures` annotation group,
// drawn ON the chart rather than only listed beside it.
//
// THE THREE FACTS THIS FILE PINS, and every one of them is a way the mark
// could otherwise make a claim nobody authored:
//
//   1. IT NEVER ENTERS THE PLOT AREA. A vertical rule crossing the data at
//      the exact period the curve turns asserts an effect by adjacency, with
//      no author and no citable source. The mark therefore lives entirely in
//      the bottom margin, under the x-axis tick labels — a statement about the
//      calendar, which is what a date of entry into force is, and about
//      nothing else. Move it into the plot and these tests fail.
//   2. IT IS SEPARATED FROM THE AXIS BY THE WHOLE ROW OF TICK LABELS. A
//      change-of-government rule ENDS at the axis line; a measure mark starts
//      a row below it. Without that separation, a measure and an investiture
//      at the same period would read as one continuous rule and the two codes
//      would collapse into one.
//   3. IT IS ONLY DRAWN INSIDE THE PLOTTED WINDOW — the same rule
//      `selectGovernmentChanges` applies, for the same reason:
//      `nearestPeriodIndex` snaps ANY date, so a 2012 measure over a
//      2020-2026 chart would otherwise be pinned to the left edge and assert
//      a date that is not there.
import { describe, expect, it } from "vitest";
import {
  buildPolicyMeasureMarks,
  selectPolicyMeasures,
  type PolicyMeasureAnnotation,
} from "../../src/lib/chart/measureMarks";
import { DEFAULT_DIMENSIONS, narrowDimensions, plotArea, xForIndex } from "../../src/lib/chart/geometry";

/** The real seeded registry, EPA half — the measures whose scope puts them on
 * /indicador/tasa-de-paro-epa and /indicador/ocupados-epa. Real dates, because
 * the window rule below is only meaningful against the entries this product
 * actually carries. */
const EPA_MEASURES: PolicyMeasureAnnotation[] = [
  { id: "rdl-3-2012-reforma-mercado-laboral", group: "measures", name: "Real Decreto-ley 3/2012", dateStart: "2012-02-12" },
  { id: "rdl-8-2020-covid-medidas-extraordinarias", group: "measures", name: "Real Decreto-ley 8/2020", dateStart: "2020-03-18" },
  { id: "rdl-32-2021-reforma-laboral", group: "measures", name: "Real Decreto-ley 32/2021", dateStart: "2021-12-31" },
];

function quarters(fromYear: number, toYear: number): string[] {
  const periods: string[] = [];
  for (let year = fromYear; year <= toYear; year++) {
    for (let q = 1; q <= 4; q++) periods.push(`${year}-Q${q}`);
  }
  return periods;
}

const WIDE_TICK_FONT_SIZE = 10;
const WIDE_X_TICK_LABEL_OFFSET = 16;

describe("selectPolicyMeasures", () => {
  it("keeps only the `measures` group, so a government or a shock can never be marked as one", () => {
    const mixed: PolicyMeasureAnnotation[] = [
      ...EPA_MEASURES,
      { id: "gobierno-sanchez-2018", group: "governments", name: "Pedro Sánchez", dateStart: "2018-06-02" },
      { id: "pandemia-2020-2021", group: "exogenous", name: "Pandemia de COVID-19", dateStart: "2020-03-14" },
    ];
    const got = selectPolicyMeasures(mixed, quarters(2010, 2026), "Q");
    expect(got.map((m) => m.id)).toEqual(EPA_MEASURES.map((m) => m.id));
  });

  it("orders by date, oldest first, whatever order the artifact delivered", () => {
    const shuffled = [EPA_MEASURES[2], EPA_MEASURES[0], EPA_MEASURES[1]];
    const got = selectPolicyMeasures(shuffled, quarters(2010, 2026), "Q");
    expect(got.map((m) => m.dateStart)).toEqual(["2012-02-12", "2020-03-18", "2021-12-31"]);
  });

  it("refuses to mark a measure whose date falls outside the plotted window", () => {
    // 2012 over a 2020-2026 chart. Unfiltered, `nearestPeriodIndex` would
    // snap it onto 2020-Q1 and put a mark under a date at which nothing was
    // enacted.
    const got = selectPolicyMeasures(EPA_MEASURES, quarters(2020, 2026), "Q");
    expect(got.map((m) => m.id)).toEqual([
      "rdl-8-2020-covid-medidas-extraordinarias",
      "rdl-32-2021-reforma-laboral",
    ]);
  });

  it("carries the registry's own date verbatim alongside the period it snapped onto", () => {
    const [covid] = selectPolicyMeasures([EPA_MEASURES[1]], quarters(2019, 2022), "Q");
    // The snapped period is where the mark is DRAWN, at the axis' own
    // resolution. The date is what the registry recorded, and the two are
    // kept apart because the sentence beside the chart states the real date
    // while the mark can only stand on a period the series observed.
    expect(covid.dateStart).toBe("2020-03-18");
    expect(covid.period).toBe("2020-Q1");
  });

  it("is empty for a series with no periods at all", () => {
    expect(selectPolicyMeasures(EPA_MEASURES, [], "Q")).toEqual([]);
  });
});

describe("buildPolicyMeasureMarks", () => {
  const periods = quarters(2019, 2022);
  const area = plotArea(DEFAULT_DIMENSIONS);
  const marks = buildPolicyMeasureMarks(
    EPA_MEASURES,
    periods,
    "Q",
    DEFAULT_DIMENSIONS,
    WIDE_TICK_FONT_SIZE,
    WIDE_X_TICK_LABEL_OFFSET,
  );

  it("stands at the x of the period it snapped onto, never at an interpolated one", () => {
    expect(marks).toHaveLength(2);
    const covid = marks[0];
    expect(covid.x).toBeCloseTo(xForIndex(periods.indexOf("2020-Q1"), periods.length, DEFAULT_DIMENSIONS), 6);
  });

  it("is drawn ENTIRELY below the plot area, so it never crosses the data", () => {
    for (const mark of marks) {
      expect(mark.y1).toBeGreaterThan(area.y1);
      expect(mark.y2).toBeGreaterThan(mark.y1);
    }
  });

  it("is separated from the axis by the whole row of tick labels, so it can never read as the continuation of a government rule", () => {
    // A change-of-government rule ENDS on the axis line. If a measure mark
    // started there too, the two would draw as one continuous line whenever
    // an investiture and an instrument landed on the same period.
    const labelBaseline = area.y1 + WIDE_X_TICK_LABEL_OFFSET;
    for (const mark of marks) {
      expect(mark.y1).toBeGreaterThan(labelBaseline);
    }
  });

  it("clears the x-axis tick labels' descenders rather than being drawn through them", () => {
    const labelInkBottom = area.y1 + WIDE_X_TICK_LABEL_OFFSET + WIDE_TICK_FONT_SIZE * 0.45;
    for (const mark of marks) {
      expect(mark.y1).toBeGreaterThanOrEqual(labelInkBottom);
    }
  });

  it("stays inside the viewBox, so nothing is clipped by the drawing's own edge", () => {
    for (const mark of marks) {
      expect(mark.y2).toBeLessThan(DEFAULT_DIMENSIONS.height);
    }
  });

  it("scales with the narrow box's own type and margins, so a phone is not served a hairline", () => {
    // The NARROW box, not the wide one at a narrow type size: that box
    // reserves a deeper bottom margin precisely because its labels are twice
    // the size, and the band this mark lives in is derived from both together.
    const narrowPoints = periods.map((period) => ({ period, value: 10, status: "D" as const }));
    const narrowDims = narrowDimensions(periods, narrowPoints, 1);
    const narrow = buildPolicyMeasureMarks(EPA_MEASURES, periods, "Q", narrowDims, 20, 28);
    expect(narrow[0].strokeWidth).toBeGreaterThan(marks[0].strokeWidth);
    expect(narrow[0].y2 - narrow[0].y1).toBeGreaterThan(marks[0].y2 - marks[0].y1);
    expect(narrow[0].y2).toBeLessThan(narrowDims.height);
  });
});
