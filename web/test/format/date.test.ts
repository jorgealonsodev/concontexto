// The project's ONE reader-facing date formatter, tested directly rather
// than only through rendered output — the same discipline, and the same
// reasoning, as `test/format/number.test.ts`.
//
// THE DEFECT THIS DRIVES. The methodology sheet published the extraction
// instant exactly as the pipeline recorded it:
//
//   Última extracción
//   2026-07-29T12:00:00Z
//
// That is a machine value in a Spanish sentence. It carries a `T` a reader
// has no reason to know, a `Z` that silently means "this clock is not
// yours", and a UTC wall time that is one or two hours behind the one the
// reader's own clock shows. The page-state banner had the same shape in
// prose: "Última actualización correcta: 2026-07-29."
//
// WHY A DIRECT UNIT TEST AND NOT ONLY A PAGE ASSERTION. The two properties
// that actually matter here are invisible to a rendered-output check. The
// first is the timezone conversion: the fixture's `12:00:00Z` is 14:00 in
// Madrid in July and would be 13:00 in January, and only a test that
// constructs both instants can prove the conversion happens at all rather
// than the `Z` merely being stripped. The second is that a BARE calendar
// date must not move: `2026-07-29` formatted through a local-time path
// becomes 28 de julio for any reader west of Greenwich, and no fixture on
// this site would ever reveal it.
import { describe, expect, it } from "vitest";

import { formatCalendarDate, formatExtractionInstant } from "../../src/lib/format/date";

describe("formatExtractionInstant — the pipeline's extraction instant, for a person", () => {
  it("renders the instant the methodology sheet was publishing raw", () => {
    // The exact value live on /indicador/tasa-de-paro-epa when this was
    // found. 12:00 UTC is 14:00 in Madrid in July.
    expect(formatExtractionInstant("2026-07-29T12:00:00Z")).toBe(
      "29 de julio de 2026 a las 14:00 (hora peninsular)",
    );
  });

  it("converts to Spanish civil time rather than stripping the Z, and follows summer/winter", () => {
    // The load-bearing pair. Same wall-clock UTC hour, six months apart:
    // Europe/Madrid is UTC+2 in July and UTC+1 in January, so a correct
    // conversion produces two DIFFERENT local hours from the same input
    // hour. An implementation that merely deleted the "Z" would print
    // "12:00" for both and pass any single-instant assertion.
    expect(formatExtractionInstant("2026-07-29T12:00:00Z")).toContain("a las 14:00");
    expect(formatExtractionInstant("2026-01-29T12:00:00Z")).toContain("a las 13:00");
  });

  it("crosses the day boundary in the reader's timezone, not in UTC", () => {
    // 23:30 UTC on 31 December is already 00:30 on 1 January in Madrid. The
    // date and the time have to be resolved in the SAME zone or the sheet
    // reports an instant that never existed.
    expect(formatExtractionInstant("2025-12-31T23:30:00Z")).toBe(
      "1 de enero de 2026 a las 00:30 (hora peninsular)",
    );
  });

  it("uses a 24-hour clock, so no reader has to disambiguate a bare hour", () => {
    // `es-ES` is a 24-hour locale, but this is pinned rather than inherited:
    // a CLDR revision that introduced an am/pm form for `es` would otherwise
    // change what this site prints with no code change. Same reasoning as
    // `formatNumber`'s explicit `useGrouping: "min2"`.
    const evening = formatExtractionInstant("2026-07-29T20:05:00Z");
    expect(evening).toContain("a las 22:05");
    expect(evening).not.toMatch(/\b[ap]\.?\s?m\.?/i);
  });

  it("pads the hour and the minute, so a column of instants stays aligned", () => {
    expect(formatExtractionInstant("2026-07-29T04:03:00Z")).toContain("a las 06:03");
  });

  it("names the timezone rather than leaving the clock unattributed", () => {
    // This is the honest half of dropping the "Z". A local time printed with
    // no reference is a worse claim than a UTC instant, not a better one:
    // the reader cannot reconcile it against the source's own release time.
    // "hora peninsular" is correct by construction — the formatter resolves
    // in Europe/Madrid, which IS peninsular time — and, unlike "CEST"/"GMT+2",
    // it does not change wording twice a year.
    expect(formatExtractionInstant("2026-07-29T12:00:00Z")).toContain("(hora peninsular)");
    expect(formatExtractionInstant("2026-01-29T12:00:00Z")).toContain("(hora peninsular)");
  });

  it("names no seconds, because no reader decision turns on one", () => {
    // Minutes are the finest granularity that answers a real question ("did
    // we read the source before or after this morning's release?"). Seconds
    // answer none, and printing them would present a precision the ingestion
    // schedule does not have.
    expect(formatExtractionInstant("2026-07-29T12:00:45Z")).toBe(
      "29 de julio de 2026 a las 14:00 (hora peninsular)",
    );
  });

  it("returns an unparseable value verbatim rather than inventing a date for it", () => {
    // P4: nothing is fabricated. The export schema types `extractedAt` as a
    // non-empty string, not an RFC 3339 instant, so a drifted writer CAN
    // reach this function with something else. A visibly raw value on one
    // field of one sheet is a smaller failure than a build that dies, or —
    // far worse — than an "Invalid Date" or a silently substituted today.
    expect(formatExtractionInstant("not-a-timestamp")).toBe("not-a-timestamp");
    expect(formatExtractionInstant("")).toBe("");
  });
});

describe("formatCalendarDate — a bare YYYY-MM-DD, for a person", () => {
  it("renders the page-state banner's date as Spanish prose", () => {
    expect(formatCalendarDate("2026-07-29")).toBe("29 de julio de 2026");
  });

  it("never shifts the day, whatever the formatting machine's own timezone is", () => {
    // The silent-corruption case this function exists to prevent. A bare
    // calendar date has no time and no zone; resolving it through local time
    // moves it backwards for any builder west of Greenwich and forwards for
    // one far enough east. The first and last day of a month are where that
    // is loudest.
    expect(formatCalendarDate("2026-01-01")).toBe("1 de enero de 2026");
    expect(formatCalendarDate("2026-12-31")).toBe("31 de diciembre de 2026");
    expect(formatCalendarDate("2024-02-29")).toBe("29 de febrero de 2024");
  });

  it("names the month in Spanish, lower-case, as Spanish orthography requires", () => {
    // RAE: month names are common nouns and are not capitalised. This is
    // `es-ES`'s own CLDR output, pinned so a locale change cannot quietly
    // introduce "Julio".
    expect(formatCalendarDate("2026-07-29")).toContain("julio");
    expect(formatCalendarDate("2026-07-29")).not.toContain("Julio");
  });

  it("returns an unparseable value verbatim, for the same reason the instant formatter does", () => {
    expect(formatCalendarDate("29/07/2026")).toBe("29/07/2026");
  });
});
