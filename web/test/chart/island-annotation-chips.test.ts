// The island's annotation chips answer the window the drawing is showing.
//
// ── THE DEFECT THIS PINS ───────────────────────────────────────────────────
//
// Measured in Chromium against real full-history data before it was written
// down. On /indicador/tasa-de-paro-epa with the measures group open, selecting
// "Desde 2018":
//
//   series points   98 -> 34   narrowed
//   measure marks   12 ->  8   narrowed
//   measure chips    3 ->  3   NOT narrowed, still naming a 2012 reform
//
// The same held for `governments` (6 chips, 1 mark at that range) and for
// `exogenous` (4 chips, 1 rail — with a 2008-2013 crisis listed under a chart
// that starts in 2018). `governments` was inconsistent even at the FULL range:
// six chips over three rules, because three of that registry's six confirmed
// investitures predate the series' own first observation.
//
// ── WHAT THIS FILE CAN AND CANNOT PROVE ────────────────────────────────────
//
// `svelte/server`'s `render()` produces the island's UNHYDRATED markup, which
// is by definition the full range: the range presets are buttons, and a
// button needs JavaScript. So these tests pin the full-range half of the rule
// — which is precisely the no-JS reader's whole experience — and the
// interactive narrowing is proved where it actually happens, in a real
// browser, by `tests/e2e/indicator/indicator-annotation-range.spec.ts`.
import { describe, expect, it } from "vitest";
import { render } from "svelte/server";
import ChartIsland from "../../src/components/ChartIsland.svelte";

const POINTS = [
  { period: "2019-Q1", value: 10.2, status: "D" as const },
  { period: "2019-Q2", value: 10.5, status: "D" as const },
  { period: "2020-Q1", value: 14.4, status: "D" as const },
  { period: "2020-Q2", value: 15.3, status: "D" as const },
];

/** One entry per shape the rule has to tell apart, each named so a failing
 * assertion says WHICH shape was misjudged rather than only that a chip was
 * missing. `milestones` and `measures` are the two groups open at first
 * render, so their chips are in the markup without any gesture. */
const ANNOTATIONS = [
  // An INSTANT before the series: no mark is drawn for it, so no chip.
  {
    id: "milestone-before",
    group: "milestones" as const,
    name: "Hito anterior a la serie",
    dateStart: "2012-02-01",
    dateEnd: null,
    href: null,
  },
  // An INSTANT inside the series.
  {
    id: "milestone-inside",
    group: "milestones" as const,
    name: "Hito dentro de la serie",
    dateStart: "2019-04-01",
    dateEnd: null,
    href: null,
  },
  // An INTERVAL entirely before the series.
  {
    id: "milestone-span-before",
    group: "milestones" as const,
    name: "Periodo anterior a la serie",
    dateStart: "2008-01-01",
    dateEnd: "2013-12-31",
    href: null,
  },
  // An INTERVAL that begins before the series and reaches into it. The rail
  // is drawn (clamped at its left end), so the chip must survive too.
  {
    id: "milestone-span-overlapping",
    group: "milestones" as const,
    name: "Periodo que alcanza la serie",
    dateStart: "2016-01-01",
    dateEnd: "2019-06-30",
    href: null,
  },
  // A MEASURE before the series, and one inside it.
  {
    id: "measure-before",
    group: "measures" as const,
    name: "Medida anterior a la serie",
    dateStart: "2012-02-10",
    dateEnd: null,
    href: "https://www.boe.es/",
  },
  {
    id: "measure-inside",
    group: "measures" as const,
    name: "Medida dentro de la serie",
    dateStart: "2020-03-17",
    dateEnd: null,
    href: "https://www.boe.es/",
  },
];

function renderIsland(annotations: typeof ANNOTATIONS, points = POINTS): string {
  return render(ChartIsland, {
    props: {
      slug: "chips",
      name: "Serie de prueba",
      unit: "% población activa",
      frequency: "Q" as const,
      decimals: 1,
      points,
      breaks: [],
      annotations,
      transforms: { yoy: false, qoq: false, perCapita: false },
      titleId: "chips-title",
      descriptionId: "chips-description",
      tableId: "chips-table",
    },
  }).body;
}

describe("ChartIsland — annotation chips against the visible window", () => {
  it("prints a chip for an instant inside the window and none for one outside it", () => {
    const html = renderIsland(ANNOTATIONS);
    expect(html).toContain("Hito dentro de la serie");
    expect(html).not.toContain("Hito anterior a la serie");
    expect(html).toContain("Medida dentro de la serie");
    expect(html).not.toContain("Medida anterior a la serie");
  });

  it("keeps an interval that merely OVERLAPS the window, and drops one that does not", () => {
    // The chip agrees with the rail rather than with the instant rule: an
    // event bounded at both ends is projected on intersection
    // (`eventSpans.ts`), because a 2016-2019 period genuinely covers the
    // first quarters of a series beginning in 2019. Applying the instant rule
    // here would delete a chip whose rail is on screen.
    const html = renderIsland(ANNOTATIONS);
    expect(html).toContain("Periodo que alcanza la serie");
    expect(html).not.toContain("Periodo anterior a la serie");
  });

  it("the chips name exactly what the generated sentence names, for the measures group", () => {
    // Three lists that can disagree is worse than the bug being fixed. The
    // sentence is derived from `selectPolicyMeasures` and the chips from
    // `annotationsInWindow`; this asserts the two answer alike.
    const html = renderIsland(ANNOTATIONS);
    const note = /data-testid="chart-measures-note"[^>]*>([\s\S]*?)<\/p>/.exec(html);
    expect(note).not.toBeNull();
    expect(note![1]).toContain("Medida dentro de la serie");
    expect(note![1]).not.toContain("Medida anterior a la serie");
  });

  it("a group whose every entry falls outside the window has no toggle at all", () => {
    // Absent, not present-and-empty — the rule `availablePresets` and the
    // government filter already follow. This toggle gates the DRAWING, so with
    // nothing in range it is a focus stop that cannot change one pixel.
    const html = renderIsland(ANNOTATIONS.filter((a) => a.id === "measure-before"));
    expect(html).not.toContain('data-testid="annotation-toggle-measures"');
    expect(html).not.toContain('data-testid="chart-annotations"');
  });

  it("a group keeps its toggle while ONE entry is still in range", () => {
    const html = renderIsland(ANNOTATIONS.filter((a) => a.group === "measures"));
    expect(html).toContain('data-testid="annotation-toggle-measures"');
    expect(html).toContain("Medida dentro de la serie");
  });
});
