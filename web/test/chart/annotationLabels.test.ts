// The on-drawing annotation LABELS — the layer that makes a mark say what it
// is without the reader leaving the chart.
//
// WHY THIS FILE IS MOSTLY ARITHMETIC. Everything this layer can get wrong is
// geometric: a label that lands on top of another label, or one that runs out
// of the box and is clipped by the SVG's own overflow. Both are invisible to
// any assertion that merely checks the text is present, and both are exactly
// what happened to this chart's axis labels before `wideDimensions` derived
// its margins. So the assertions here are collision and containment, over the
// REAL worst case measured in Chromium on the built pages, and never over a
// convenient synthetic one.
//
// THE REAL WORST CASE, measured on /indicador/poblacion-residente (294 points,
// six changes of government) with `getBBox()` in Chromium against each
// drawing's own viewBox:
//
//   government   wide x    narrow x   name (glyphs)
//   Calvo-Sotelo  217.65    201.32    Leopoldo Calvo-Sotelo (21)
//   González      238.90    210.52    Felipe González (15)
//   Aznar         430.13    293.31    José María Aznar (16)
//   Zapatero      543.45    342.36    José Luis Rodríguez Zapatero (28)
//   Rajoy         649.69    388.36    Mariano Rajoy (13)
//   Sánchez       741.77    428.21    Pedro Sánchez (13)
//
//   wide plot area   x 76..933,  y 16..328
//   narrow plot area x 140..511, y 20..364
//
// Calvo-Sotelo and González are 21.25 units apart in the wide drawing and 9.20
// apart in the narrow one — the pair that made printing horizontal names on
// this chart impossible, and the pair every collision assertion below is
// really about.
import { describe, expect, it } from "vitest";
import {
  NARROW_ANNOTATION_LABEL_FONT_SIZE,
  WIDE_ANNOTATION_LABEL_FONT_SIZE,
  placeAnnotationLabels,
  type PlacedAnnotationLabel,
} from "../../src/lib/chart/annotationLabels";
import type { PlotArea } from "../../src/lib/chart/geometry";

const WIDE_AREA: PlotArea = { x0: 76, y0: 16, x1: 933, y1: 328, width: 857, height: 312 };
const NARROW_AREA: PlotArea = { x0: 140, y0: 20, x1: 511, y1: 364, width: 371, height: 344 };

/** The six real investitures of /indicador/poblacion-residente, at the x each
 * drawing really projects them to. */
const WIDE_GOVERNMENTS = [
  { id: "gobierno-calvo-sotelo-1981", name: "Leopoldo Calvo-Sotelo", x: 217.65 },
  { id: "gobierno-gonzalez-1982", name: "Felipe González", x: 238.9 },
  { id: "gobierno-aznar-1996", name: "José María Aznar", x: 430.13 },
  { id: "gobierno-zapatero-2004", name: "José Luis Rodríguez Zapatero", x: 543.45 },
  { id: "gobierno-rajoy-2011", name: "Mariano Rajoy", x: 649.69 },
  { id: "gobierno-sanchez-2018", name: "Pedro Sánchez", x: 741.77 },
];

const NARROW_GOVERNMENTS = WIDE_GOVERNMENTS.map((g, i) => ({
  ...g,
  x: [201.32, 210.52, 293.31, 342.36, 388.36, 428.21][i],
}));

/** The two rails /indicador/tasa-de-paro-epa really draws with its `exogenous`
 * group open, at the coordinates `buildEventSpanRails` produces for them. The
 * crisis name is 58 glyphs — far longer than any president's — which is the
 * whole reason the rail label needs its own fitting rule. */
const WIDE_RAILS = [
  {
    id: "crisis-financiera-2008-2013",
    name: "Crisis financiera global y crisis de deuda soberana europea",
    x1: 272.99,
    x2: 480.94,
    y: 28,
    serifLength: 6,
  },
  { id: "pandemia-2020-2021", name: "Pandemia de COVID-19", x1: 706.97, x2: 752.18, y: 28, serifLength: 6 },
];

const NARROW_RAILS = [
  { ...WIDE_RAILS[0], x1: 183.63, x2: 286.77, y: 44, serifLength: 12 },
  { ...WIDE_RAILS[1], x1: 398.89, x2: 421.31, y: 44, serifLength: 12 },
];

/** Every pair of placed labels whose rectangles intersect, named. Two labels
 * that overlap by a fraction of a unit are as broken as two that sit on top of
 * one another: both produce glyphs drawn through glyphs. */
function overlappingPairs(labels: PlacedAnnotationLabel[]): string[] {
  const clashes: string[] = [];
  for (let i = 0; i < labels.length; i++) {
    for (let j = i + 1; j < labels.length; j++) {
      const a = labels[i];
      const b = labels[j];
      const overlapsX = a.x < b.x + b.width && b.x < a.x + a.width;
      const overlapsY = a.y < b.y + b.height && b.y < a.y + a.height;
      if (overlapsX && overlapsY) clashes.push(`${a.id} × ${b.id}`);
    }
  }
  return clashes;
}

/** Every placed label with any part of its rectangle outside the plot area —
 * the clipping defect this chart already shipped once, on its axis labels. */
function outsidePlotArea(labels: PlacedAnnotationLabel[], area: PlotArea): string[] {
  return labels
    .filter((l) => l.x < area.x0 || l.x + l.width > area.x1 || l.y < area.y0 || l.y + l.height > area.y1)
    .map((l) => l.id);
}

function allLabels(result: { governments: PlacedAnnotationLabel[]; eventSpans: PlacedAnnotationLabel[] }) {
  return [...result.governments, ...result.eventSpans];
}

describe("placeAnnotationLabels — the wide drawing, six real changes of government", () => {
  const result = placeAnnotationLabels({
    governments: WIDE_GOVERNMENTS,
    eventSpans: [],
    area: WIDE_AREA,
    fontSize: WIDE_ANNOTATION_LABEL_FONT_SIZE,
  });

  it("labels every one of the six, because a marked change with no label is the defect this layer exists to fix", () => {
    expect(result.governments.map((l) => l.id)).toEqual(WIDE_GOVERNMENTS.map((g) => g.id));
  });

  it("prints the registry's own name verbatim, never a surname derived from it", () => {
    // "José Luis Rodríguez Zapatero" has two surnames and "Leopoldo
    // Calvo-Sotelo" has a compound one; any last-token rule would misname at
    // least one future entry, and the registry carries no surname field to
    // read instead. So the whole name is what is drawn.
    expect(result.governments.map((l) => l.text)).toEqual(WIDE_GOVERNMENTS.map((g) => g.name));
  });

  it("draws no label through another label", () => {
    const clashes = overlappingPairs(result.governments);
    expect(clashes, `overlapping labels: ${clashes.join(", ")}`).toEqual([]);
  });

  it("draws every label inside the plot area, so none is clipped by the viewBox", () => {
    const outside = outsidePlotArea(result.governments, WIDE_AREA);
    expect(outside, `outside the plot area: ${outside.join(", ")}`).toEqual([]);
  });

  it("keeps the 21-unit pair apart by placing each beside its own rule, never by moving one off its rule", () => {
    // The label identifies the rule it stands on. A solver that resolved the
    // collision by sliding a label sideways would produce a chart where the
    // name over one rule belongs to another — worse than no label at all.
    for (const label of result.governments) {
      const marker = WIDE_GOVERNMENTS.find((g) => g.id === label.id)!;
      expect(label.x, `${label.id} was moved off its own rule`).toBeCloseTo(marker.x, 5);
    }
  });
});

describe("placeAnnotationLabels — the narrow drawing, where the same pair is 9.20 units apart", () => {
  const result = placeAnnotationLabels({
    governments: NARROW_GOVERNMENTS,
    eventSpans: [],
    area: NARROW_AREA,
    fontSize: NARROW_ANNOTATION_LABEL_FONT_SIZE,
  });

  it("still labels all six on the drawing a phone reader actually receives", () => {
    expect(result.governments.map((l) => l.id)).toEqual(NARROW_GOVERNMENTS.map((g) => g.id));
  });

  it("draws no label through another label", () => {
    const clashes = overlappingPairs(result.governments);
    expect(clashes, `overlapping labels: ${clashes.join(", ")}`).toEqual([]);
  });

  it("draws every label inside the plot area", () => {
    const outside = outsidePlotArea(result.governments, NARROW_AREA);
    expect(outside, `outside the plot area: ${outside.join(", ")}`).toEqual([]);
  });

  it("resolves the collision by dropping the second label DOWN the plot, never by shrinking or truncating it", () => {
    // Vertical text needs a line-height of horizontal room instead of a
    // string-length of it, which is what makes six names fit on a 371-unit
    // plot at all. The 9.20-unit pair is closer than one line height even so,
    // and the only honest answer left is to start the second label below the
    // first — the same lane discipline `selectEventSpans` already applies to
    // overlapping rails.
    const first = result.governments.find((l) => l.id === "gobierno-calvo-sotelo-1981")!;
    const second = result.governments.find((l) => l.id === "gobierno-gonzalez-1982")!;
    expect(second.y).toBeGreaterThanOrEqual(first.y + first.height);
    expect(second.text).toBe("Felipe González");
  });
});

describe("placeAnnotationLabels — the editorial event rails", () => {
  it("labels a rail whose name fits between its own start and the plot's right edge", () => {
    const result = placeAnnotationLabels({
      governments: [],
      eventSpans: WIDE_RAILS,
      area: WIDE_AREA,
      fontSize: WIDE_ANNOTATION_LABEL_FONT_SIZE,
    });
    expect(result.eventSpans.map((l) => l.id)).toEqual([
      "crisis-financiera-2008-2013",
      "pandemia-2020-2021",
    ]);
    expect(result.eventSpans[0].text).toBe(
      "Crisis financiera global y crisis de deuda soberana europea",
    );
    expect(outsidePlotArea(result.eventSpans, WIDE_AREA)).toEqual([]);
  });

  it("withholds a rail label that cannot be drawn whole, rather than truncating the registry's own words", () => {
    // 58 glyphs at the narrow label size need about 520 units; the narrow plot
    // area is 371 wide. There is no honest way to print it: an ellipsis would
    // rename the event on screen, and shrinking the type below the size the
    // rest of this drawing uses would trade one unreadable label for another.
    // The sentence under the chart names it in full, in the same left-to-right
    // order the rails appear in, so the omission is disclosed rather than
    // silent.
    const result = placeAnnotationLabels({
      governments: [],
      eventSpans: NARROW_RAILS,
      area: NARROW_AREA,
      fontSize: NARROW_ANNOTATION_LABEL_FONT_SIZE,
    });
    expect(result.eventSpans.map((l) => l.id)).toEqual(["pandemia-2020-2021"]);
    expect(result.eventSpans[0].text).toBe("Pandemia de COVID-19");
    expect(outsidePlotArea(result.eventSpans, NARROW_AREA)).toEqual([]);
  });

  it("centres a rail label on the rail it names, so it can never be read as belonging to the next one", () => {
    // Found in a real browser rather than reasoned about: an earlier rule
    // anchored the label on the event's first period and, when that overran the
    // box, on its last. On the narrow /indicador/tasa-de-paro-epa drawing that
    // put "Pandemia de COVID-19" entirely to the LEFT of the pandemic rail and
    // partly under the 2008-2013 crisis rail above it — a name attached to the
    // wrong mark, which is worse than no name. Centring keeps the words over
    // the interval they describe.
    for (const [rails, area, fontSize] of [
      [WIDE_RAILS, WIDE_AREA, WIDE_ANNOTATION_LABEL_FONT_SIZE],
      [NARROW_RAILS, NARROW_AREA, NARROW_ANNOTATION_LABEL_FONT_SIZE],
    ] as const) {
      const result = placeAnnotationLabels({ governments: [], eventSpans: rails, area, fontSize });
      for (const label of result.eventSpans) {
        const rail = rails.find((r) => r.id === label.id)!;
        expect(label.x + label.width / 2, `${label.id} drifted off its own rail`).toBeCloseTo(
          (rail.x1 + rail.x2) / 2,
          5,
        );
      }
    }
  });

  it("never lets a clamped rail label lose contact with the span it names", () => {
    // Centring can still be pushed sideways by the plot's own edge, and a label
    // shoved clear of its rail names nothing. A rail hard against the right edge
    // with a name wider than the room beyond it is the case: the label is
    // withheld rather than parked somewhere it does not belong.
    const result = placeAnnotationLabels({
      governments: [],
      eventSpans: [
        { id: "borde", name: "Un acontecimiento de nombre largo", x1: 925, x2: 933, y: 28, serifLength: 6 },
      ],
      area: WIDE_AREA,
      fontSize: WIDE_ANNOTATION_LABEL_FONT_SIZE,
    });
    for (const label of result.eventSpans) {
      expect(label.x, "the label lost contact with its rail").toBeLessThanOrEqual(933);
      expect(label.x + label.width).toBeGreaterThanOrEqual(925);
    }
    expect(outsidePlotArea(result.eventSpans, WIDE_AREA)).toEqual([]);
  });

  it("never lets a government label and a rail label share the same ink", () => {
    // Both layers are drawn at the top of the same plot, so they are each
    // other's nearest neighbour long before either is another of its own kind.
    // /indicador/tasa-de-paro-epa is the page where it really happens: Rajoy's
    // 2011 rule stands inside the 2008-2013 crisis rail.
    const result = placeAnnotationLabels({
      governments: [
        { id: "gobierno-zapatero-2004", name: "José Luis Rodríguez Zapatero", x: 137.37 },
        { id: "gobierno-rajoy-2011", name: "Mariano Rajoy", x: 408.61 },
        { id: "gobierno-sanchez-2018", name: "Pedro Sánchez", x: 643.68 },
      ],
      eventSpans: WIDE_RAILS,
      area: WIDE_AREA,
      fontSize: WIDE_ANNOTATION_LABEL_FONT_SIZE,
    });
    const clashes = overlappingPairs(allLabels(result));
    expect(clashes, `overlapping labels: ${clashes.join(", ")}`).toEqual([]);
    expect(outsidePlotArea(allLabels(result), WIDE_AREA)).toEqual([]);
    // ...and nothing was silently lost to make that true on this page.
    expect(result.governments).toHaveLength(3);
    expect(result.eventSpans).toHaveLength(2);
  });

  it("keeps a rail label clear of the rail it names, never through its own serifs", () => {
    const result = placeAnnotationLabels({
      governments: [],
      eventSpans: WIDE_RAILS,
      area: WIDE_AREA,
      fontSize: WIDE_ANNOTATION_LABEL_FONT_SIZE,
    });
    for (const label of result.eventSpans) {
      const rail = WIDE_RAILS.find((r) => r.id === label.id)!;
      expect(label.y, `${label.id} was drawn over its own rail`).toBeGreaterThanOrEqual(
        rail.y + rail.serifLength,
      );
    }
  });
});

describe("placeAnnotationLabels — the honest refusals", () => {
  it("withholds a label rather than drawing it outside the plot area", () => {
    // A pathological density: five investitures on one x. Four of them cannot
    // be stacked inside a 312-unit plot, and a label drawn past the bottom edge
    // would be clipped to a fragment of a name — a worse lie than no name.
    const crowd = Array.from({ length: 5 }, (_, i) => ({
      id: `g${i}`,
      name: "José Luis Rodríguez Zapatero",
      x: 500,
    }));
    const result = placeAnnotationLabels({
      governments: crowd,
      eventSpans: [],
      area: WIDE_AREA,
      fontSize: WIDE_ANNOTATION_LABEL_FONT_SIZE,
    });
    expect(result.governments.length).toBeLessThan(crowd.length);
    expect(outsidePlotArea(result.governments, WIDE_AREA)).toEqual([]);
    expect(overlappingPairs(result.governments)).toEqual([]);
  });

  it("labels nothing when nothing is marked, so the layer is byte-absent rather than empty", () => {
    const result = placeAnnotationLabels({
      governments: [],
      eventSpans: [],
      area: WIDE_AREA,
      fontSize: WIDE_ANNOTATION_LABEL_FONT_SIZE,
    });
    expect(result.governments).toEqual([]);
    expect(result.eventSpans).toEqual([]);
  });
});
