// Government-term derivation (indicator-page spec, "Annotation layers per
// PRD §6.1.1(a)" — the `governments` event group; series-transformations
// spec, "Range presets": "A preset whose start precedes the series' first
// observation MUST be absent, not disabled").
//
// WHY THIS MODULE EXISTS AT ALL, and why its tests are this insistent: every
// government in `config/gobiernos.yaml` carries a START date and NONE carries
// an end date. Slicing a chart to "el gobierno de Rajoy" therefore requires
// deciding where that term ENDS, and that decision is an INFERENCE — it is not
// a fact the editorial registry states. This project does not let inferences
// pass unmarked (`BreakConfig.DateStatus` refuses to project an unconfirmed
// date at all; `reconocimientos.yaml` refuses to treat a drafted argument as a
// signature), so the derived end is carried in the type as `endKind`, never
// collapsed into an indistinguishable date, and the UI discloses it.
//
// The inference is: government N runs until government N+1 takes office. It
// holds for Spanish prime-ministerial succession, which is continuous by
// construction — the outgoing president stays in functions until the successor
// is sworn in, so there is no unattributed gap. It BREAKS if the registry ever
// records a genuine gap, a caretaker period modelled as an absence, or if a
// government is missing from the registry entirely: a missing MIDDLE government
// would be silently absorbed into its predecessor's term. That last case is not
// hypothetical — `gobierno-suarez-1976` carries `date_status: unconfirmed` and
// is deliberately never projected, which is exactly why the first derivable
// term starts in 1981 and every observation before it belongs to NO term.
import { describe, expect, it } from "vitest";
import {
  availableGovernmentTerms,
  deriveGovernmentTerms,
  type GovernmentAnnotation,
} from "../../src/lib/transform/governmentTerms";
import { sliceCustomRange } from "../../src/lib/transform/sliceRange";

/** The six entries `config/gobiernos.yaml` really projects today, in the shape
 * the export artifact delivers them (`EventRefSchema`) — real dates, because
 * the boundary arithmetic this module performs is only meaningful against the
 * real ones. `gobierno-suarez-1976` is absent here for the same reason it is
 * absent from the artifact: its date is unconfirmed and reconciliation never
 * projects it. */
const GOVERNMENTS: GovernmentAnnotation[] = [
  { id: "gobierno-calvo-sotelo-1981", group: "governments", name: "Leopoldo Calvo-Sotelo", dateStart: "1981-02-25" },
  { id: "gobierno-gonzalez-1982", group: "governments", name: "Felipe González", dateStart: "1982-12-02" },
  { id: "gobierno-aznar-1996", group: "governments", name: "José María Aznar", dateStart: "1996-05-05" },
  { id: "gobierno-zapatero-2004", group: "governments", name: "José Luis Rodríguez Zapatero", dateStart: "2004-04-17" },
  { id: "gobierno-rajoy-2011", group: "governments", name: "Mariano Rajoy", dateStart: "2011-12-21" },
  { id: "gobierno-sanchez-2018", group: "governments", name: "Pedro Sánchez", dateStart: "2018-06-02" },
];

/** Quarterly period labels from `from` to `to` inclusive — a compact way to
 * write the observation lists these tests slice against. */
function quarters(fromYear: number, fromQuarter: number, toYear: number, toQuarter: number): string[] {
  const periods: string[] = [];
  let year = fromYear;
  let quarter = fromQuarter;
  while (year < toYear || (year === toYear && quarter <= toQuarter)) {
    periods.push(`${year}-Q${quarter}`);
    quarter += 1;
    if (quarter > 4) {
      quarter = 1;
      year += 1;
    }
  }
  return periods;
}

describe("deriveGovernmentTerms — the succession inference, made explicit", () => {
  it("ends each term at the period BEFORE its successor takes office, and marks that end as derived", () => {
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");

    const rajoy = terms.find((t) => t.id === "gobierno-rajoy-2011")!;
    // 2011-12-21 -> 2011-Q4; Sánchez's 2018-06-02 -> 2018-Q2, so Rajoy's last
    // fully-attributable period is 2018-Q1.
    expect(rajoy.startPeriod).toBe("2011-Q4");
    expect(rajoy.endPeriod).toBe("2018-Q1");
    // The end is NOT a fact the registry states, and the type says so.
    expect(rajoy.endKind).toBe("succession");
    expect(rajoy.succeededById).toBe("gobierno-sanchez-2018");
    // The configured start, by contrast, travels verbatim.
    expect(rajoy.startDate).toBe("2011-12-21");
  });

  it("leaves the sitting government's term OPEN rather than inventing a closing date", () => {
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");
    const sanchez = terms.at(-1)!;

    expect(sanchez.id).toBe("gobierno-sanchez-2018");
    expect(sanchez.startPeriod).toBe("2018-Q2");
    expect(sanchez.endPeriod).toBeNull();
    expect(sanchez.endKind).toBe("open");
    expect(sanchez.succeededById).toBeNull();
  });

  it("prefers a CONFIGURED end date over the inference, and marks it as configured", () => {
    // Nothing in `gobiernos.yaml` carries `date_end` today, but `EventRefSchema`
    // admits it (that is how an `exogenous` episode is closed). If the registry
    // ever states an end, the stated fact must win over the derived one —
    // otherwise the inference would overwrite the very data it stands in for.
    const withConfiguredEnd: GovernmentAnnotation[] = [
      { ...GOVERNMENTS[4], dateEnd: "2018-01-15" },
      GOVERNMENTS[5],
    ];
    const rajoy = deriveGovernmentTerms(withConfiguredEnd, "Q")[0];

    expect(rajoy.endPeriod).toBe("2018-Q1");
    expect(rajoy.endKind).toBe("configured");
    expect(rajoy.succeededById).toBeNull();
  });

  it("orders by start date regardless of the order the artifact happens to deliver", () => {
    // The artifact's `events` array is not contractually sorted, and a
    // succession read off an unsorted list would attribute terms to the wrong
    // governments — silently, and with no rendering error to notice it by.
    const shuffled = [GOVERNMENTS[3], GOVERNMENTS[0], GOVERNMENTS[5], GOVERNMENTS[2], GOVERNMENTS[4], GOVERNMENTS[1]];
    expect(deriveGovernmentTerms(shuffled, "Q").map((t) => t.id)).toEqual(GOVERNMENTS.map((g) => g.id));
  });

  it("ignores every non-government annotation, so an unrelated event cannot interrupt a succession", () => {
    const mixed: GovernmentAnnotation[] = [
      GOVERNMENTS[4],
      { id: "pandemia-2020-2021", group: "exogenous", name: "Pandemia de COVID-19", dateStart: "2020-03-14", dateEnd: "2021-05-09" },
      { id: "reforma-laboral-2021", group: "milestones", name: "Reforma laboral", dateStart: "2021-12-30" },
      GOVERNMENTS[5],
    ];
    const terms = deriveGovernmentTerms(mixed, "Q");

    expect(terms.map((t) => t.id)).toEqual(["gobierno-rajoy-2011", "gobierno-sanchez-2018"]);
    // Rajoy still ends where Sánchez begins — the 2020 shock in between did
    // not become a successor.
    expect(terms[0].endPeriod).toBe("2018-Q1");
  });

  it("gives the handover period to the INCOMING government, so no period belongs to two terms", () => {
    // A president is sworn in mid-period, and a period is a span, not an
    // instant: 2018-Q2 contains two months of Rajoy and one of Sánchez. Half-
    // open attribution ([start of N, start of N+1)) is the only rule that
    // leaves every period in exactly one term; the alternative puts one
    // observation under two governments at once.
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");
    const windows = terms.map((t) => ({ from: t.startPeriod, to: t.endPeriod }));

    expect(windows).toEqual([
      { from: "1981-Q1", to: "1982-Q3" },
      { from: "1982-Q4", to: "1996-Q1" },
      { from: "1996-Q2", to: "2004-Q1" },
      { from: "2004-Q2", to: "2011-Q3" },
      { from: "2011-Q4", to: "2018-Q1" },
      { from: "2018-Q2", to: null },
    ]);
  });

  it("derives the same succession on a monthly axis", () => {
    const terms = deriveGovernmentTerms(GOVERNMENTS, "M");
    const aznar = terms.find((t) => t.id === "gobierno-aznar-1996")!;

    expect(aznar.startPeriod).toBe("1996-05");
    expect(aznar.endPeriod).toBe("2004-03"); // Zapatero: 2004-04-17 -> 2004-04
  });
});

describe("availableGovernmentTerms — a filter that cannot narrow anything is absent", () => {
  // The spec states the absence rule for the five FIXED presets: "A preset
  // whose start precedes the series' first observation MUST be absent, not
  // disabled", with the stated reason that such a preset is identical in
  // effect to "full". A government term is not one of those presets and its
  // disqualifying condition is different, so this is the same discipline
  // EXTENDED BY ANALOGY, not the clause applied literally — recorded here
  // rather than claimed as compliance. The two disqualifying cases are:
  //   (1) the term selects NO observation (it is a view of nothing), and
  //   (2) the term selects EVERY observation (it is "Todo el periodo" under
  //       another name — the presets' own stated reason for absence).
  it("omits a government whose term does not overlap the series at all", () => {
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");
    // The real `tasa-de-paro-epa` span.
    const available = availableGovernmentTerms(terms, quarters(2002, 1, 2026, 2), "Q");

    expect(available.map((t) => t.id)).toEqual([
      "gobierno-aznar-1996",
      "gobierno-zapatero-2004",
      "gobierno-rajoy-2011",
      "gobierno-sanchez-2018",
    ]);
  });

  it("omits a term that would select the WHOLE series — a duplicate of the full-range control", () => {
    // The three-period artifact the deployed stack currently serves: 2025-Q4
    // to 2026-Q2 sits entirely inside Sánchez's open term, so selecting it
    // would redraw exactly the same chart. One option, doing nothing, reads
    // as a broken control rather than as a filter.
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");
    expect(availableGovernmentTerms(terms, ["2025-Q4", "2026-Q1", "2026-Q2"], "Q")).toEqual([]);
  });

  it("keeps every observation before the first derivable government reachable, without attributing it to anyone", () => {
    // `poblacion-residente` starts in 1971. `gobierno-suarez-1976` is
    // `date_status: unconfirmed` and is deliberately never projected, so the
    // earliest term this module can derive starts in 1981. The 1971-1980
    // observations therefore belong to NO selectable government — and the
    // earliest term must NOT be stretched back to the series' start to cover
    // them, which would assert that Calvo-Sotelo governed in 1971.
    const periods = quarters(1971, 1, 2026, 2);
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");
    const available = availableGovernmentTerms(terms, periods, "Q");

    expect(available).toHaveLength(6);
    const earliest = available[0];
    expect(earliest.startPeriod).toBe("1981-Q1");
    const sliced = sliceCustomRange(
      periods.map((period) => ({ period })),
      "Q",
      earliest.startPeriod,
      earliest.endPeriod!,
    );
    expect(sliced[0].period).toBe("1981-Q1");
    expect(sliced.at(-1)!.period).toBe("1982-Q3");
    // The pre-1981 data is untouched and still reachable through the full
    // range — it is simply not selectable BY GOVERNMENT.
    expect(periods[0]).toBe("1971-Q1");
  });

  it("omits a term whose whole window falls inside a gap in the series' own cadence", () => {
    // Same failure `resolveCustomRange` already guards against: a window can
    // sit between two observations without containing either. Checked against
    // the real observation list, never against [first, last].
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");
    const gapped = ["1979-Q1", "1990-Q1", "2026-Q1"]; // nothing in 1981-1982
    const available = availableGovernmentTerms(terms, gapped, "Q");

    expect(available.map((t) => t.id)).not.toContain("gobierno-calvo-sotelo-1981");
    expect(available.map((t) => t.id)).toContain("gobierno-gonzalez-1982");
  });

  it("omits a term whose window is empty because two governments share one period", () => {
    // On an ANNUAL axis two governments sworn in the same calendar year
    // collapse to one period, so the earlier one's half-open window
    // [start, successor start) holds nothing. Absent, not present-and-empty.
    const sameYear: GovernmentAnnotation[] = [
      { id: "a", group: "governments", name: "A", dateStart: "1981-02-25" },
      { id: "b", group: "governments", name: "B", dateStart: "1981-11-30" },
    ];
    const terms = deriveGovernmentTerms(sameYear, "A");
    expect(terms[0].endPeriod).toBe("1980");
    expect(availableGovernmentTerms(terms, ["1979", "1980", "1981", "1982"], "A").map((t) => t.id)).toEqual(["b"]);
  });

  it("returns nothing at all for a series with no observations", () => {
    const terms = deriveGovernmentTerms(GOVERNMENTS, "Q");
    expect(availableGovernmentTerms(terms, [], "Q")).toEqual([]);
  });

  it("returns nothing when the artifact carries no government events", () => {
    expect(deriveGovernmentTerms([], "Q")).toEqual([]);
    expect(availableGovernmentTerms([], quarters(2002, 1, 2026, 2), "Q")).toEqual([]);
  });
});
