// design-system spec, "Reserved semantics are exclusive" — tasks.md 5.8.
// Amber MUST mean pending data; dotted grey MUST mean provisional data;
// neither MAY be reused for any other meaning (ADR-8's colour freedom does
// not touch this — it carries meaning, not identity).
//
// Scope note: no component consumes these tokens yet (they arrive in
// slice 6+); this test enforces the two mechanically-checkable facts
// available NOW — (1) the token declarations are correctly self-documented
// as reserved, in both themes, and (2) no other source file under
// `web/src` silently reuses the same hex value under a different name. Once
// real components exist, this same test keeps guarding them without any
// further change.
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { THEME_CSS_PATH } from "./compile-theme";
import { narrowChartVariant, renderChartSVG } from "../../src/lib/chart/svg";
import type { ChartPoint } from "../../src/lib/chart/geometry";

function walk(dir: string, exts: string[], out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) walk(full, exts, out);
    else if (exts.some((ext) => full.endsWith(ext))) out.push(full);
  }
  return out;
}

const SRC_DIR = path.resolve(path.dirname(THEME_CSS_PATH), "..", "..");
const theme = readFileSync(THEME_CSS_PATH, "utf-8");

/**
 * Extracts ONLY the trailing `/* ... *\/` comment text of a declaration
 * line, deliberately excluding the property name/value. The property name
 * itself is `--color-pending`, which already contains the substring
 * "pending" — asserting against the whole line would be vacuous (it would
 * pass even if the comment were replaced with something unrelated).
 */
function trailingComment(line: string): string {
  const match = /\/\*(.*)\*\//.exec(line);
  return match ? match[1] : "";
}

describe("reserved semantics (PRD §12.5): amber is pending-only, dotted grey is provisional-only", () => {
  it("every non-zeroing --color-pending declaration is commented as pending-data", () => {
    const lines = theme
      .split("\n")
      .filter((l) => /--color-pending\s*:/.test(l) && !l.includes("--color-*"));
    expect(lines.length).toBeGreaterThan(0); // one per theme (light + dark)
    for (const line of lines) {
      expect(trailingComment(line).toLowerCase()).toContain("pending");
    }
  });

  it("every non-zeroing --color-provisional declaration is commented as provisional-data", () => {
    const lines = theme
      .split("\n")
      .filter((l) => /--color-provisional\s*:/.test(l) && !l.includes("--color-*"));
    expect(lines.length).toBeGreaterThan(0);
    for (const line of lines) {
      expect(trailingComment(line).toLowerCase()).toContain("provisional");
    }
  });

  it("no other source file redeclares the reserved amber/grey hex under a different token name", () => {
    const pendingHexes = [...theme.matchAll(/--color-pending:\s*(#[0-9a-fA-F]{6})/g)].map((m) => m[1].toLowerCase());
    const provisionalHexes = [...theme.matchAll(/--color-provisional:\s*(#[0-9a-fA-F]{6})/g)].map((m) =>
      m[1].toLowerCase(),
    );
    expect(pendingHexes.length).toBeGreaterThan(0);
    expect(provisionalHexes.length).toBeGreaterThan(0);
    const reservedHexes = new Set([...pendingHexes, ...provisionalHexes]);

    const otherFiles = walk(path.join(SRC_DIR, "src"), [".astro", ".svelte", ".css", ".ts"]).filter(
      (f) => path.resolve(f) !== path.resolve(THEME_CSS_PATH),
    );
    for (const file of otherFiles) {
      const content = readFileSync(file, "utf-8").toLowerCase();
      for (const hex of reservedHexes) {
        expect(content.includes(hex), `${file} reuses reserved colour ${hex} outside theme.css`).toBe(false);
      }
    }
  });
});

describe("reserved semantics usage (slice 6): every real component usage is exclusive", () => {
  // Now that real components exist, this activates the usage half of the
  // spec scenarios ("every usage of the amber token is a pending-data
  // indication" / "every dotted-grey usage marks a provisional observation")
  // that slice 5's own tests could only assert vacuously (no consumer yet).
  const COMPONENTS_DIR = path.resolve(path.dirname(THEME_CSS_PATH), "..", "components");
  const componentFiles = walk(COMPONENTS_DIR, [".astro"]);

  it("the -pending token class is used only in FreshnessSemaphore.astro", () => {
    const offenders = componentFiles.filter((f) => {
      if (f.endsWith("FreshnessSemaphore.astro")) return false;
      const content = readFileSync(f, "utf-8");
      return /-pending\b/.test(content);
    });
    expect(offenders, `pending-token usage outside FreshnessSemaphore.astro: ${offenders.join(", ")}`).toEqual([]);
  });

  it("the -provisional token class is used only in AccessibleDataTable.astro and IndicatorChart.astro", () => {
    // Slice 7: indicator-page spec's "Every point discloses provisional or
    // definitive" requires the CHART itself (not only its data table) to
    // render provisional points with the reserved dotted-grey encoding — a
    // second, legitimate consumer, not a leak.
    const ALLOWED = ["AccessibleDataTable.astro", "IndicatorChart.astro"];
    const offenders = componentFiles.filter((f) => {
      if (ALLOWED.some((name) => f.endsWith(name))) return false;
      const content = readFileSync(f, "utf-8");
      return /-provisional\b/.test(content);
    });
    expect(
      offenders,
      `provisional-token usage outside ${ALLOWED.join(", ")}: ${offenders.join(", ")}`,
    ).toEqual([]);
  });

  it("FreshnessSemaphore's pending usage is paired with the exact pending-data label, never a different meaning", () => {
    const content = readFileSync(path.join(COMPONENTS_DIR, "FreshnessSemaphore.astro"), "utf-8");
    expect(content).toContain("Pendiente de actualización por la fuente");
  });
});

// ---------------------------------------------------------------------------
// The DASH half of the reserved semantic, asserted on what is actually drawn.
//
// The tests above police the reserved COLOURS by scanning source text. Nothing
// policed the reserved STROKE PATTERN — and theme.css's header reserves both
// ("dotted grey always means provisional data"). That gap became load-bearing
// the moment the chart gained a third vertical/line treatment for changes of
// government: "línea punteada" is the obvious way to draw one, and it is
// exactly the thing that must not happen, because one reader would then hold
// two contradictory meanings for one visual code.
//
// So this is asserted against `renderChartSVG`'s real output rather than
// against its source: any element that carries a dash must also carry the
// provisional colour, in both variants. Make the government marker dashed and
// these fail.
describe("reserved semantics: a dash in the chart means provisional data and nothing else", () => {
  const POINTS: ChartPoint[] = [
    { period: "2019-Q1", value: 10.2, status: "D" },
    { period: "2019-Q2", value: 10.5, status: "D" },
    { period: "2019-Q3", value: 10.1, status: "D" },
    { period: "2019-Q4", value: 9.8, status: "P" },
  ];
  const INPUT = {
    points: POINTS,
    breaks: [{ key: "covid-2020", date: "2019-06-01" }],
    governmentChanges: [
      { id: "gobierno-ejemplo", group: "governments", name: "Gobierno de ejemplo", dateStart: "2019-07-01" },
    ],
    eventSpans: [
      { id: "hito-ejemplo", group: "milestones", name: "Hito de ejemplo", dateStart: "2019-04-01", dateEnd: "2019-09-30" },
    ],
    frequency: "Q" as const,
    decimals: 1,
    unit: "% población activa",
    titleId: "t",
    descriptionId: "d",
    tableId: "tb",
  };

  /** Every drawn element (self-closing tag) that carries a dash pattern. */
  function dashedElements(svg: string): string[] {
    return (svg.match(/<[a-z]+[^>]*stroke-dasharray[^>]*>/g) ?? []) as string[];
  }

  for (const [variantName, input] of [
    ["wide", INPUT],
    ["narrow", { ...INPUT, titleId: "t-narrow", ...narrowChartVariant(POINTS, 1) }],
  ] as const) {
    it(`every dashed element in the ${variantName} drawing is a provisional one`, () => {
      const svg = renderChartSVG(input);
      const dashed = dashedElements(svg);
      // Not vacuous: the fixture ends on a provisional observation, so there
      // IS a legitimate dash to find.
      expect(dashed.length).toBeGreaterThan(0);
      for (const element of dashed) {
        expect(element, `a dashed element that is not provisional: ${element}`).toContain(
          "var(--color-provisional)",
        );
      }
    });

    it(`the ${variantName} change-of-government marker is solid and never borrows the provisional encoding`, () => {
      const svg = renderChartSVG(input);
      const marker = /<g class="chart-government-marker"[\s\S]*?<\/g>/.exec(svg)?.[0];
      expect(marker, "no government marker was rendered, so this guard would pass vacuously").toBeTruthy();
      expect(marker).not.toContain("stroke-dasharray");
      expect(marker).not.toContain("var(--color-provisional)");
      expect(marker).not.toContain("var(--color-break-band)");
    });

    // The FOURTH treatment, held to the same standard as the third and for the
    // same reason. "Una línea punteada que abarque el periodo" is the obvious
    // way to draw a span — and it is exactly the thing that must not happen,
    // because the dash already means "this observation is provisional" and one
    // reader cannot hold two meanings for one code.
    //
    // Mutation-checked rather than assumed: add `stroke-dasharray` to the rail
    // in `svg.ts`, or repaint it in the provisional grey, and this fails.
    it(`the ${variantName} event-span rail is solid and borrows no other layer's encoding`, () => {
      const svg = renderChartSVG(input);
      const rail = /<g class="chart-event-span"[\s\S]*?<\/g>/.exec(svg)?.[0];
      expect(rail, "no event span was rendered, so this guard would pass vacuously").toBeTruthy();
      expect(rail).not.toContain("stroke-dasharray");
      expect(rail).not.toContain("var(--color-provisional)");
      // Not the break band's wash either: the separation between a rail and a
      // band is stroke-versus-fill, so a rail that gained a fill would have
      // collapsed the two treatments into one whatever colour it used.
      expect(rail).not.toContain("var(--color-break-band)");
      expect(rail).toContain('fill="none"');
      // Nor the government rule's ink. Colour is the SECOND channel here, but
      // two marks in one ink that differ only by orientation is a weaker
      // separation than this chart can afford now that it carries four.
      expect(rail).not.toContain("var(--color-ink)");
      expect(rail).toContain("var(--color-event-span)");
    });
  }
});
