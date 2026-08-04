import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { IndicatorPage } from "./indicator-page";

// The annotation CHIPS answer the same window the MARKS are drawn against —
// proved in a real browser, on real built markup.
//
// ── THE DEFECT ─────────────────────────────────────────────────────────────
//
// Measured in Chromium against the real full-history stack before it was
// written down. On /indicador/tasa-de-paro-epa with the measures group open,
// selecting "Desde 2018":
//
//   series points   98 -> 34    narrowed, correct
//   measure marks   12 ->  8    narrowed, correct
//   measure chips    3 ->  3    NOT narrowed — still listing a 2012 reform
//
// The marks respected the visible window and the chip row did not, so a reader
// who narrowed to 2018 onwards still saw a 2012 measure listed as though it
// were in view. `governments` and `exogenous` behaved identically — and
// `governments` was wrong even at the FULL range: six chips over three rules,
// because three of this registry's six confirmed investitures predate the
// series' own 2002 start. Fixing only the group that was reported would have
// left the other two lying in the same way.
//
// ── WHICH GROUPS THIS FILE CAN PROVE, AND WHY NOT THE THIRD ────────────────
//
// `governments` and `exogenous`, on this page, against the built artifact's
// own registry — which for those two groups carries exactly the entries the
// real one does (same ids, same names, same dates, the same 2002-Q1..2026-Q2
// span). `measures` is absent from that artifact entirely: the policy-measure
// registry landed after the fixture was written, and the fixture is not this
// slice's to change. That is not a gap left open — it is the same split
// `chart-policy-measures.spec.ts` already documents and works within, and the
// measures group's own range behaviour is proved there, in a browser, against
// the workbench fixture that does carry one.
//
// ── WHAT "AGREE" MEANS, PRECISELY ──────────────────────────────────────────
//
// Chips, marks and the generated sentence must name the same entries — with
// ONE asymmetry that is correct and long-standing rather than a leak: an
// `exogenous` entry carrying no `date_end` is an INSTANT, so it earns a chip
// when its date is in the window, but it has no interval to project and
// therefore no rail, and the sentence names only the events the registry
// "acota con fecha de inicio y de fin". Four chips over two rails at the full
// range is that difference, and the sentence is what accounts for it.
const SLUG = "tasa-de-paro-epa";

const CRISIS = "Crisis financiera global y crisis de deuda soberana europea";
const PANDEMIA = "Pandemia de COVID-19";
const NGEU = "Mecanismo de Recuperación";
const SHOCK = "Shock energético 2022";

test.describe("Indicator page — annotations answer the visible date range", () => {
  test(
    "at the full range, the chips name exactly the entries inside the series",
    { tag: ["@indicator-page", "@annotation-range"] },
    async ({ page }) => {
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await indicator.annotationToggle("governments").click();
      await indicator.annotationToggle("exogenous").click();

      // GOVERNMENTS — the group that was wrong before any narrowing at all.
      // Calvo-Sotelo (1981), González (1982) and Aznar (1996) all took office
      // before this series' first observation, so no rule is drawn for them
      // and no chip may name them.
      await expect(indicator.annotationChips("governments")).toHaveText([
        /Rodríguez Zapatero/,
        /Rajoy/,
        /Sánchez/,
      ]);
      await expect(indicator.governmentMarkers).toHaveCount(3);
      await expect(indicator.governmentNote).toContainText("Rodríguez Zapatero");
      await expect(indicator.governmentNote).not.toContainText("Aznar");

      // EXOGENOUS — four chips over two rails, which is the end-less-event
      // asymmetry rather than a leak: NGEU and the energy shock are instants
      // inside the series, so they are chipped and not railed, and the
      // sentence names only the events it can bound.
      await expect(indicator.annotationChips("exogenous")).toHaveCount(4);
      await expect(indicator.eventSpans).toHaveCount(2);
      await expect(indicator.eventSpansNote).toContainText(CRISIS);
      await expect(indicator.eventSpansNote).toContainText(PANDEMIA);
      await expect(indicator.eventSpansNote).not.toContainText(SHOCK);
    },
  );

  test(
    'narrowing to "Desde 2018" narrows the chips, the marks and the sentence alike',
    { tag: ["@indicator-page", "@annotation-range"] },
    async ({ page }) => {
      // The exact gesture the defect was measured under.
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await indicator.annotationToggle("governments").click();
      await indicator.annotationToggle("exogenous").click();
      await indicator.rangePreset("since-2018").click();

      // GOVERNMENTS: one investiture inside 2018-Q1..2026-Q2, in the chips, in
      // the drawing and in the words.
      await expect(indicator.annotationChips("governments")).toHaveText([/Sánchez/]);
      await expect(indicator.governmentMarkers).toHaveCount(1);
      await expect(indicator.governmentMarkersNarrow).toHaveCount(1);
      await expect(indicator.governmentNote).toContainText("Pedro Sánchez");
      await expect(indicator.governmentNote).not.toContainText("Rajoy");
      await expect(indicator.annotationGroupContent("governments")).not.toContainText("Rajoy");

      // EXOGENOUS: the 2008-2013 crisis does not reach 2018, so it loses both
      // its rail and — this is the fix — its chip. The pandemic keeps both.
      // NGEU and the energy shock keep their chips as in-range instants, and
      // are correctly absent from the rails and from the sentence.
      await expect(indicator.annotationChips("exogenous")).toHaveText([
        new RegExp(PANDEMIA),
        new RegExp(NGEU),
        new RegExp(SHOCK),
      ]);
      await expect(indicator.eventSpans).toHaveCount(1);
      await expect(indicator.eventSpansNote).toContainText(PANDEMIA);
      await expect(indicator.eventSpansNote).not.toContainText(CRISIS);
      await expect(indicator.annotationGroupContent("exogenous")).not.toContainText(CRISIS);
    },
  );

  test(
    "the government filter — a narrower window still — keeps only what it contains",
    { tag: ["@indicator-page", "@annotation-range"] },
    async ({ page }) => {
      // Rajoy's term (T4 2011 .. T1 2018) is the sharpest check available,
      // because it is not "recent entries survive": it keeps the 2008-2013
      // crisis that "Desde 2018" drops, and drops the pandemic that "Desde
      // 2018" keeps. A filter that merely favoured newer entries passes the
      // test above and fails this one.
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await indicator.annotationToggle("governments").click();
      await indicator.annotationToggle("exogenous").click();
      await indicator.governmentSelect.selectOption("gobierno-rajoy-2011");
      await expect(indicator.governmentStatus).toContainText("de T4 2011 a T1 2018");

      // GOVERNMENTS: attribution is half-open, so the term begins at its own
      // investiture and the successor's falls outside it — at most one marker,
      // on the left edge, naming the boundary the reader selected.
      await expect(indicator.annotationChips("governments")).toHaveText([/Rajoy/]);
      await expect(indicator.governmentMarkers).toHaveCount(1);
      await expect(indicator.governmentNote).toContainText("Mariano Rajoy");
      await expect(indicator.governmentNote).not.toContainText("Sánchez");

      // EXOGENOUS: the crisis INTERSECTS this window without being contained
      // in it, which is the whole reason an interval gets the weaker test —
      // the rail is drawn, clamped at the right, so the chip must stay. The
      // pandemic, NGEU and the energy shock are all after the term ends.
      await expect(indicator.annotationChips("exogenous")).toHaveText([new RegExp(CRISIS)]);
      await expect(indicator.eventSpans).toHaveCount(1);
      await expect(indicator.eventSpansNote).toContainText(CRISIS);
      await expect(indicator.eventSpansNote).toContainText("fuera del periodo representado");
      await expect(indicator.annotationGroupContent("exogenous")).not.toContainText(PANDEMIA);
    },
  );

  test(
    "a group with nothing left in the window loses its control, and regains it when the range widens",
    { tag: ["@indicator-page", "@annotation-range"] },
    async ({ page }) => {
      // The empty-group decision, measured. A custom range of 2023-2026 sits
      // after every entry this artifact carries for this series, so both
      // groups — and the section holding them — go ABSENT rather than present
      // and empty: the same discipline `availablePresets` and the government
      // filter already follow, and the same one the break list follows two
      // elements up.
      //
      // The recovery is the other half of the decision, and the reason the
      // disappearance is acceptable: "Todo el periodo" is offered on every
      // series whatever its span, so a reader is always one click from every
      // entry again. This asserts that, rather than assuming it.
      const indicator = new IndicatorPage(page, SLUG);
      await indicator.goto();
      await indicator.waitForChartHydrated();
      await expect(indicator.annotations).toBeVisible();

      await indicator.customRangeFrom.fill("2023-01-01");
      await indicator.customRangeTo.fill("2026-04-01");
      await indicator.customRangeApply.click();
      await expect(indicator.customRangeStatus).toContainText("Rango personalizado aplicado");

      await expect(indicator.annotations).toHaveCount(0);
      await expect(indicator.annotationToggle("governments")).toHaveCount(0);
      await expect(indicator.annotationToggle("exogenous")).toHaveCount(0);
      // Nothing is drawn there either, which is the whole argument for
      // removing the control rather than emptying it: it could not have
      // changed one pixel of the page.
      await expect(indicator.governmentMarkers).toHaveCount(0);
      await expect(indicator.eventSpans).toHaveCount(0);

      await indicator.rangePreset("full").click();
      await expect(indicator.annotations).toBeVisible();
      await expect(indicator.annotationToggle("governments")).toBeVisible();
      await indicator.annotationToggle("governments").click();
      await expect(indicator.annotationChips("governments")).toHaveCount(3);
    },
  );

  // The state this slice introduced is scanned rather than assumed to inherit
  // the page's existing pass: the chips are links and text inside a disclosure
  // whose contents now change under the reader's hand, and a group going
  // absent removes the very node its toggle's `aria-controls` pointed at. Both
  // are exactly the kind of change that breaks a reference or an accessible
  // name.
  //
  // ONE TEST PER THEME, each on its own page load, exactly as
  // `indicator-pages.spec.ts` parameterises its own audit — not a loop inside
  // one test. Measured rather than stylistic: axe-core caches resolved
  // background colours per frame, so a second `analyze()` after flipping
  // `data-theme` on an already-audited page reports the DARK foreground
  // against the LIGHT background it cached (here #4ade80 on #626569, a
  // "3.36:1" violation of a pair that never appears on screen). A fresh load
  // per theme is what makes the contrast figure real.
  for (const theme of ["light", "dark"] as const) {
    test(
      `a narrowed chart with its groups open has zero axe violations (${theme} theme)`,
      { tag: ["@indicator-page", "@annotation-range", "@a11y"] },
      async ({ page }) => {
        const indicator = new IndicatorPage(page, SLUG);
        await indicator.goto();
        await indicator.waitForChartHydrated();
        // The page always renders `data-theme="light"` server-side and carries
        // no toggle, so the dark pass sets the attribute the
        // `[data-theme="dark"]` block declares.
        if (theme === "dark") {
          await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
        }
        await indicator.annotationToggle("governments").click();
        await indicator.annotationToggle("exogenous").click();
        await indicator.rangePreset("since-2018").click();
        await expect(indicator.annotationChips("exogenous")).toHaveCount(3);

        const results = await new AxeBuilder({ page }).analyze();
        expect(
          results.violations,
          `${theme}: ${JSON.stringify(results.violations, null, 2)}`,
        ).toEqual([]);
      },
    );
  }
});
