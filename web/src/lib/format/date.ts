// The ONE place a date or an instant becomes reader-facing text on this
// site — the sibling of `./number.ts`, deliberately built to the same shape
// and to the same boundary.
//
// WHY THIS MODULE EXISTS. The methodology sheet rendered the pipeline's own
// extraction instant verbatim: `Última extracción: 2026-07-29T12:00:00Z`.
// The page-state banner did the same in prose: `Última actualización
// correcta: 2026-07-29.` Both are machine values printed into Spanish
// sentences. `pageState.ts`'s own comment said so at the time, and named the
// reason — "reformatted into prose this codebase has no locale-formatting
// vocabulary for". This module is that vocabulary.
//
// WHAT PRECISION A READER GETS, AND WHY IT IS NOT JUST THE DATE. The
// temptation with a machine timestamp is to hide everything technical and
// print "29 de julio de 2026". That is the wrong call HERE, for one reason
// specific to what this field is. `extractedAt` is not a "last updated"
// badge; it is a provenance fact (P2: every figure traceable to its primary
// source) recorded by the ingestion run. The question a reader actually
// brings to it on the one day it matters is "the INE published this
// morning — is the figure on this page from before or after that release?"
// A date alone cannot answer that question, and two extractions on the same
// day would be indistinguishable in the visible copy. So:
//
//   - DATE AND TIME, to the minute. The minute is the finest granularity
//     that answers a real question. Seconds answer none, and printing them
//     would advertise a precision the ingestion schedule does not have —
//     so `formatExtractionInstant` truncates them rather than rounding.
//   - THE TIMEZONE MATTERS, and is converted rather than dropped. The
//     artifact records UTC; Spain is UTC+1 or UTC+2. Printing "12:00" from
//     `12:00:00Z` would be wrong by an hour or two for every reader, and
//     would be wrong INVISIBLY — a reader reconciling against a 09:00 INE
//     release would draw a confident, false conclusion. The instant is
//     resolved in `Europe/Madrid` and the clock is NAMED (see
//     `es.dates.instant`), because an unattributed local time is a weaker
//     claim than a UTC instant, not a stronger one.
//   - THE MACHINE VALUE IS NOT DESTROYED. `MethodologySheetFields.astro`
//     renders the result inside `<time datetime="{the original ISO
//     instant}">`, which is exactly what that element is for: the reader
//     sees prose, the markup still carries the unambiguous instant, and a
//     scraper or a citation tool reads the same value it read before this
//     change. That is why nothing here needs to compromise between the two
//     audiences.
//
// THE MACHINE-READABLE BOUNDARY, STATED AS `./number.ts` states it. This
// module formats text a PERSON reads. It must never touch `/data-derived`'s
// published CSV/JSON (written by the Go pipeline, covered by manifest.json's
// digest chain, consumed by programs), the `datetime` attribute value
// itself, permalink query parameters, or any value round-tripped back into
// code. As with numerals, the boundary is structural: the web tree never
// generates those files.
import { es } from "../../i18n/es";
import { LOCALE } from "./number";

/** Spain's civil time. Named once, used by both the day part and the clock
 * part, so the two can never be resolved in different zones and report an
 * instant that never existed (23:30 UTC on 31 December is 00:30 on 1
 * January here — the date and the hour have to move together). */
const CIVIL_TIME_ZONE = "Europe/Madrid";

/** The day, in Spanish civil time. Explicit component options rather than
 * `dateStyle: "long"`, for the reason `formatNumber` gives for
 * `useGrouping: "min2"`: a future CLDR revision to `es` must not be able to
 * change what this site prints without a code change. */
const DAY_IN_CIVIL_TIME = new Intl.DateTimeFormat(LOCALE, {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: CIVIL_TIME_ZONE,
});

/** The clock, in Spanish civil time.
 *
 * `hourCycle: "h23"` is stated rather than inherited. `es-ES` is a 24-hour
 * locale today, so this changes nothing today — which is the point: it is
 * pinned so that it stays true, and so that no reader ever has to
 * disambiguate a bare "6:03". `2-digit` on both fields keeps every instant
 * the same width, which is what makes the `tabular-nums` the sheet already
 * applies mean anything. */
const CLOCK_IN_CIVIL_TIME = new Intl.DateTimeFormat(LOCALE, {
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
  timeZone: CIVIL_TIME_ZONE,
});

/** A BARE calendar date, resolved in UTC — deliberately NOT in
 * `CIVIL_TIME_ZONE`.
 *
 * `YYYY-MM-DD` has no time and no zone; ECMAScript parses it as UTC
 * midnight. Formatting that instant in any zone west of Greenwich moves it
 * to the previous day, so `2026-01-01` would print as "31 de diciembre de
 * 2025" — a corruption no fixture on this site would ever reveal, because
 * the machine that builds the pages happens to run in Madrid. Resolving in
 * the same zone it was parsed in is what makes the day a fixed fact rather
 * than a function of the builder's locale. */
const CALENDAR_DAY = new Intl.DateTimeFormat(LOCALE, {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

/**
 * One extraction instant as a Spanish reader reads it:
 * `29 de julio de 2026 a las 14:00 (hora peninsular)`.
 *
 * An unparseable input is returned VERBATIM. `SeriesDocSchema` types
 * `extractedAt` as a non-empty string rather than an RFC 3339 instant, so a
 * drifted writer really can reach this function with something else, and
 * every alternative is worse than showing what was recorded: `Intl` would
 * print "Invalid Date", a fallback to `Date.now()` would fabricate
 * provenance (P4), and throwing would take the whole site down over one
 * field of one sheet. The raw value is at least true.
 */
export function formatExtractionInstant(iso: string): string {
  const instant = new Date(iso);
  if (Number.isNaN(instant.getTime())) return iso;
  return es.dates.instant(DAY_IN_CIVIL_TIME.format(instant), CLOCK_IN_CIVIL_TIME.format(instant));
}

/**
 * One bare `YYYY-MM-DD` calendar date as a Spanish reader reads it:
 * `29 de julio de 2026`.
 *
 * Used by the page-state banner, whose date the export schema already
 * constrains to exactly this shape (`PageStateSchema`, so that the banner
 * "prints this value verbatim into reader-facing Spanish prose" could never
 * receive a full instant). The shape is re-checked here anyway rather than
 * trusted: this function is reachable from anywhere, and `new Date()` would
 * happily parse a full instant and then silently resolve it in UTC.
 */
export function formatCalendarDate(isoDate: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(isoDate)) return isoDate;
  const day = new Date(`${isoDate}T00:00:00Z`);
  if (Number.isNaN(day.getTime())) return isoDate;
  return CALENDAR_DAY.format(day);
}
