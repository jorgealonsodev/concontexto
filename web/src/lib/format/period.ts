// The ONE place a period label becomes reader-facing text on this site — the
// third sibling of `./number.ts` and `./date.ts`, deliberately built to the
// same shape and to the same boundary.
//
// WHY THIS MODULE EXISTS. `/indicador/tasa-de-paro-epa` printed the
// database's own canonical storage format to its readers twenty-two times on
// one page: `2026-Q2` in the header, in every row of the accessible data
// table, on every x-axis tick and inside the generated Spanish prose. That
// shape — `'2026-Q2' | '2026-06' | '2025'` — is what migration 0001's own
// comment calls it: how a period is STORED. The `Q` in it is the English
// abbreviation for *quarter*.
//
// Spain's official statistics do not use it, and this project's own source
// proves the point: INE's API returns the field `T3_Periodo` carrying
// `"T1"`–`"T4"` — T for *trimestre* — and INE's press releases write "el
// segundo trimestre de 2020". Every figure here is taken from a source that
// says T and was being shown to the reader as Q.
//
// This is NOT the indicator-page spec's "technical identifiers are not
// translated" case. That scenario names what it protects — "indicator slugs,
// configuration filenames, source names and origin series identifiers" — and
// a period label is none of the four. It is a date, and dates on this site
// are already reformatted for their reader by `./date.ts`.
//
// ---------------------------------------------------------------------------
// TWO REGISTERS, AND THE RULE THAT PICKS BETWEEN THEM.
//
// A period is rendered in two structurally different places, and one form
// cannot serve both:
//
//   COMPACT  the label sits in a COLUMN or on an AXIS, and its width is
//            load-bearing. The accessible data table's `Periodo` column is
//            three columns wide on a 375 px phone; the chart's x-axis ticks
//            are what the derived margins in `lib/chart/geometry.ts` are
//            sized from. "septiembre de 2026" wraps the first and widens the
//            second for no gain a reader can use.
//   PROSE    the label sits in a SENTENCE or in a labelled field, where there
//            is no column to keep and no tick to fit. The chart's generated
//            description reads "El valor sube de 121,2 en junio de 2019 a
//            …"; the page header reads "Periodo: junio de 2026". An
//            abbreviation there is a saving nobody asked for, and a screen
//            reader announcing the same string has no horizontal space to
//            save at all.
//
// The rule is stated HERE, in two named functions, rather than left implied
// by which call site happened to pick which — that is the whole reason there
// are two exports and not one function with a boolean.
//
// The two registers differ in EXACTLY ONE PLACE: the month name. A quarter is
// "T2 2026" in both, because that form was chosen by the product owner and a
// second, longer quarter form would leave the site saying two different
// things about one period. A year is "2026" in both, because a bare year is
// already a word a reader reads: there is nothing to spell out and nothing to
// abbreviate.
//
// ---------------------------------------------------------------------------
// THE MACHINE-READABLE BOUNDARY, STATED AS ITS TWO SIBLINGS STATE IT — and
// here it is sharper than for either of them, because the canonical period
// label is not merely published, it is COMPUTED WITH.
//
// This module formats text a PERSON reads. It must never touch:
//
//   - `/data-derived/csv/*.csv` and `/data-derived/series/*.json`. Written by
//     the Go pipeline (`app/internal/publishing`), covered by manifest.json's
//     sha256 digest chain, consumed by programs. The web tree never generates
//     either file — `ActionBar` links to the published bytes — so the
//     boundary is structural here, not a rule someone has to remember.
//     `test/format/machine-surfaces.test.ts` pins it anyway.
//   - Permalink query parameters. `lib/chart/permalink.ts` writes the custom
//     range's bounds as PERIOD LABELS and `resolveCustomRange` parses them
//     back. A display label in `?from=` is a shared link that silently
//     resolves to the full series on every reopen.
//   - `<time datetime>` attribute values, which exist precisely so the prose
//     beside them can be reformatted without the machine value being lost.
//   - ANY value that is sorted, compared, subtracted or looked up:
//     `periodOrdinalIndex`, `nearestPeriodIndex`, `addYears`,
//     `previousPeriod`, `sliceRange`/`sliceCustomRange`, the
//     `period → status`/`period → population` join maps, and the `{#each}`
//     keys the island renders its table and its points with. "T2 2026" does
//     not sort, does not parse, and does not match a key written as
//     "2026-Q2". A display formatter applied one function too early turns a
//     chronological axis into an alphabetical one.
//
// The rule that keeps all of that safe is a single sentence: FORMAT AT THE
// RENDER SITE, never in the data. Nothing in this module mutates a point, and
// no caller of it may store what it returns.
import { es } from "../../i18n/es";
import { LOCALE } from "./number";

// The canonical storage shapes, re-stated here rather than imported from
// `lib/chart/periods.ts`. Two reasons, both about direction of dependency:
// `lib/format/` is the presentation layer and must not reach into the chart's
// domain arithmetic to render a table cell, and `parsePeriod` over there is
// deliberately fail-CLOSED (it throws on a mismatch, because a mismatch there
// is a data-contract defect) while this module must be fail-OPEN for exactly
// the reason `formatCalendarDate` is — see `formatVerbatim` below.
const QUARTER_RE = /^(\d{4})-Q([1-4])$/;
const MONTH_RE = /^(\d{4})-(0[1-9]|1[0-2])$/;
const YEAR_RE = /^\d{4}$/;

/** The month name alone, abbreviated — the compact register's month half.
 *
 * `Intl` rather than a hand-written table of twelve strings, for the same
 * reason `./number.ts` gives for `Intl.NumberFormat`: `es-ES` is the locale
 * authority this site already publishes under, and `./date.ts` already prints
 * "29 de julio de 2026" from it. A private table would be a second, silently
 * diverging Spanish calendar.
 *
 * The COMPONENTS are pinned explicitly (never `dateStyle`), which is the
 * pinning `./date.ts` argues for: CLDR may revise a name — it revised `es`'s
 * September abbreviation from "sep" to "sept" — but it cannot change WHICH
 * fields this site prints without a code change here. */
const COMPACT_MONTH = new Intl.DateTimeFormat(LOCALE, {
  month: "short",
  year: "numeric",
  timeZone: "UTC",
});

/** The month name spelled out, with the connector CLDR's own `es` pattern
 * supplies ("junio de 2026") — the prose register's month half. The connector
 * is deliberately NOT authored in `es.ts`: it is the locale's grammar, the
 * same grammar `formatExtractionInstant` already renders through. */
const PROSE_MONTH = new Intl.DateTimeFormat(LOCALE, {
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

/** The instant `Intl` names a month from: the first day of that month, in
 * UTC — deliberately NOT in `Europe/Madrid`.
 *
 * A period has no time and no zone. Resolving one in a civil zone west of
 * Greenwich would move `2026-06` to 31 May and print "mayo", which is the
 * corruption `./date.ts`'s `CALENDAR_DAY` documents for bare calendar dates —
 * invisible on a machine that happens to build in Madrid.
 *
 * `setUTCFullYear` is not decoration: `Date.UTC(50, 5, 1)` is 1950, not the
 * year 50, because of ECMAScript's legacy two-digit-year mapping. The regex
 * above accepts any four digits, so the remap is reachable. */
function firstDayOfMonth(year: number, month: number): Date {
  const day = new Date(Date.UTC(2000, month - 1, 1));
  day.setUTCFullYear(year);
  return day;
}

/**
 * One period label where its WIDTH is load-bearing — a data-table column or a
 * chart axis tick: `T2 2026`, `jun 2026`, `2026`.
 *
 * A quarter comes out at exactly the seven glyphs `2026-Q2` occupied, so the
 * margins `lib/chart/geometry.ts` derives from the widest tick label are
 * unchanged for every quarterly series. A month grows from seven to eight (or
 * nine, for "sept"), which those same derived margins absorb — that is what
 * deriving them was for.
 *
 * An input that is not a canonical period label is returned VERBATIM, for the
 * reason `formatCalendarDate` states: `Intl` would print "Invalid Date", a
 * fallback would fabricate a period that was never observed (P4), and
 * throwing would take a whole page down over one cell. The raw value is at
 * least true.
 */
export function formatPeriodCompact(period: string): string {
  return formatPeriod(period, COMPACT_MONTH);
}

/**
 * One period label inside a sentence or a labelled field — the generated
 * chart description, the page header, a break band, a screen-reader point
 * announcement: `T2 2026`, `junio de 2026`, `2026`.
 *
 * Differs from `formatPeriodCompact` in the month name and nowhere else. Same
 * verbatim fallback, for the same reason.
 */
export function formatPeriodProse(period: string): string {
  return formatPeriod(period, PROSE_MONTH);
}

function formatPeriod(period: string, monthFormat: Intl.DateTimeFormat): string {
  const quarter = QUARTER_RE.exec(period);
  if (quarter) return es.periods.quarter(Number(quarter[2]), quarter[1]);

  const month = MONTH_RE.exec(period);
  if (month) return monthFormat.format(firstDayOfMonth(Number(month[1]), Number(month[2])));

  // An annual period is already what both registers would print, so it is
  // returned as-is rather than routed through a formatter that could only
  // change it (`Intl.NumberFormat` would be free to group "2026" as "2.026").
  if (YEAR_RE.test(period)) return period;

  return period;
}
