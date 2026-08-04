// The project's ONE reader-facing period formatter, tested directly rather
// than only through rendered output — the same discipline, and the same
// reasoning, as `test/format/number.test.ts` and `test/format/date.test.ts`.
//
// THE DEFECT THIS DRIVES. `/indicador/tasa-de-paro-epa` printed the database's
// own canonical storage format to its readers, twenty-two times on one page:
//
//   Periodo    2026-Q2
//   T2 header  Periodo: 2026-Q2
//   x axis     2019-Q1 … 2026-Q2
//   prose      "El valor sube de 10,2 % población activa en 2019-Q1 a …"
//
// `'2026-Q2' | '2026-06' | '2025'` is exactly what migration 0001's own
// comment calls it: the storage shape. The `Q` in it is the English
// abbreviation for *quarter*, and Spain's official statistics do not use it.
// The proof is in this project's own ingestion path: INE's API returns the
// field `T3_Periodo` carrying `"T1"`–`"T4"` — T for *trimestre* — and INE's
// press releases write "el segundo trimestre de 2020". Every figure on this
// site comes from a source that says T, and the site said Q.
//
// WHY THIS IS NOT THE SPEC'S "TECHNICAL IDENTIFIERS ARE NOT TRANSLATED" CASE.
// That scenario (indicator-page spec) enumerates what it protects: "indicator
// slugs, configuration filenames, source names and origin series identifiers".
// A period label is none of the four. It is a DATE — the same category as the
// extraction instant `lib/format/date.ts` already reformats, and for the same
// reason: a machine value printed into Spanish reader-facing copy.
//
// WHY TWO REGISTERS AND NOT ONE. See `src/lib/format/period.ts`'s own header
// for the rule. The tests below pin both halves of it, including the one
// cadence where they differ (months) and the two where they deliberately do
// not (quarters, years).
import { describe, expect, it } from "vitest";

import { formatPeriodCompact, formatPeriodProse } from "../../src/lib/format/period";

describe("formatPeriodCompact — a period where its width is load-bearing (table column, axis tick)", () => {
  it("renders a quarter as INE's own T, year last", () => {
    // The product owner's decision, and the source's own vocabulary: the
    // ingestion path reads `T3_Periodo: "T2"` from INE and the page now says
    // the same thing back.
    expect(formatPeriodCompact("2026-Q2")).toBe("T2 2026");
    expect(formatPeriodCompact("2019-Q1")).toBe("T1 2019");
    expect(formatPeriodCompact("2020-Q3")).toBe("T3 2020");
    expect(formatPeriodCompact("1971-Q4")).toBe("T4 1971");
  });

  it("renders a month as an abbreviated Spanish month name and a year", () => {
    // Lowercase is not an oversight: Spanish month names are common nouns
    // (RAE, *Ortografía* §4.7). CLDR's `es` abbreviations are what `Intl`
    // returns and what this site prints everywhere else a month is named.
    expect(formatPeriodCompact("2026-06")).toBe("jun 2026");
    expect(formatPeriodCompact("2026-01")).toBe("ene 2026");
    expect(formatPeriodCompact("2026-12")).toBe("dic 2026");
  });

  it("renders an annual period as the bare year, because it already is one", () => {
    expect(formatPeriodCompact("2026")).toBe("2026");
  });

  it("keeps a quarter at exactly the glyph count the canonical label had", () => {
    // Load-bearing for `lib/chart/geometry.ts`: the chart's margins are
    // DERIVED from the widest x-tick label, and those margins were fixed only
    // moments before this change to stop the last tick being clipped. Seven
    // glyphs in, seven glyphs out — so every quarterly page's geometry is
    // byte-identical to what it was, and the golden SVG fixture moves only in
    // its tick TEXT.
    expect(formatPeriodCompact("2026-Q2")).toHaveLength("2026-Q2".length);
  });
});

describe("formatPeriodProse — a period inside a sentence or a labelled field", () => {
  it("spells the month out, with the connector Spanish prose needs", () => {
    // "El valor sube de 121,2 en junio de 2019 a …" — the sentence the chart's
    // generated description actually builds. "en jun 2019" is an abbreviation
    // in running prose, and a screen reader has no column to save space in.
    expect(formatPeriodProse("2026-06")).toBe("junio de 2026");
    expect(formatPeriodProse("2026-09")).toBe("septiembre de 2026");
    expect(formatPeriodProse("1971-01")).toBe("enero de 1971");
  });

  it("leaves a quarter exactly as the compact register renders it", () => {
    // The two registers differ in ONE place — the month name — and this is
    // the assertion that says so. The owner fixed the quarter's form as
    // "T2 2026"; inventing a second, longer quarter form ("el segundo
    // trimestre de 2026") for prose would override a decision already taken,
    // and would leave the site saying two different things about one period.
    expect(formatPeriodProse("2026-Q2")).toBe("T2 2026");
    expect(formatPeriodProse("2026-Q2")).toBe(formatPeriodCompact("2026-Q2"));
  });

  it("leaves an annual period exactly as the compact register renders it", () => {
    // A bare year is already a word a reader reads. There is nothing to spell
    // out and nothing to abbreviate.
    expect(formatPeriodProse("2026")).toBe("2026");
    expect(formatPeriodProse("2026")).toBe(formatPeriodCompact("2026"));
  });
});

describe("both registers — inputs that are not canonical period labels", () => {
  // The fail-open discipline `formatCalendarDate` already states: an input
  // that is not the shape this function knows is returned VERBATIM. Every
  // alternative is worse — `Intl` would print "Invalid Date", a fallback
  // would fabricate a period that was never observed (P4), and throwing would
  // take a whole page down over one cell.
  it.each([
    ["", ""],
    ["2026-Q5", "2026-Q5"],
    ["2026-Q0", "2026-Q0"],
    ["2026-13", "2026-13"],
    ["2026-00", "2026-00"],
    ["2026-6", "2026-6"],
    ["26-Q2", "26-Q2"],
    ["2026-07-29", "2026-07-29"],
    ["2026-q2", "2026-q2"],
    ["tasa-de-paro-epa", "tasa-de-paro-epa"],
  ])("returns %j verbatim", (input, expected) => {
    expect(formatPeriodCompact(input)).toBe(expected);
    expect(formatPeriodProse(input)).toBe(expected);
  });

  it("is idempotent on its own output, so a double-formatted call site cannot corrupt a label", () => {
    // Not a licence to format twice — it is the safety net under a refactor
    // that accidentally does. An already-rendered "T2 2026" is not a
    // canonical label, so it falls through the verbatim path unchanged.
    for (const canonical of ["2026-Q2", "2026-06", "2026"]) {
      expect(formatPeriodCompact(formatPeriodCompact(canonical))).toBe(formatPeriodCompact(canonical));
      expect(formatPeriodProse(formatPeriodProse(canonical))).toBe(formatPeriodProse(canonical));
    }
  });

  it("resolves a two-digit-looking year as itself, never as 19xx", () => {
    // `Date.UTC(50, 5, 1)` is 1950, not year 50 — ECMAScript's legacy
    // two-digit-year mapping. No real series reaches back that far, but the
    // regex accepts any four digits and a formatter that silently moved a
    // year by nineteen centuries would be a corruption no fixture would show.
    expect(formatPeriodCompact("0050-06")).toBe("jun 50");
    expect(formatPeriodProse("0050-06")).toBe("junio de 50");
  });
});
