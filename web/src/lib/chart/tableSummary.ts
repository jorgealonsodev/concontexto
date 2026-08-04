// The label on the disclosure the accessible data table lives inside.
//
// WHY THIS IS A MODULE AND NOT TWO TEMPLATE EXPRESSIONS. The table is emitted
// by two independent renderers — `components/AccessibleDataTable.astro` (the
// static build) and `components/ChartIsland.svelte` (which re-renders the
// whole table as the range narrows, because the Astro/Svelte boundary forbids
// hydrating the other one's DOM). This project already paid for that shape
// once: `test/design-system/break-band-parity.test.ts` exists because the same
// duplication let the break band's two copies part company. design.md D-5
// states the answer — "never two renderers that must agree" — so the label is
// computed HERE, once, and both renderers print what this returns.
//
// WHY THE LABEL SAYS WHAT IT SAYS. The disclosure is closed on arrival, which
// means the reader decides whether to open it from this line alone. The row
// count and the covered span are both known at render time and are exactly the
// two facts that decision needs: 98 rows over 2002-2026 is a different
// proposition from 26 rows over the last five years. The caption INSIDE the
// table already names the series and its unit, so this deliberately names
// neither — see `i18n/es.ts`'s `chart.tableDisclosure` for that reasoning and
// for why the phrasing is a noun phrase rather than "Ver la tabla".
import { es } from "../../i18n/es";
import { formatNumber } from "../format/number";
import { formatPeriodProse } from "../format/period";

/** The narrowest input that can answer the question: this label is built from
 * how many periods there are and which two sit at the ends. Values, statuses
 * and everything else a `ChartPoint` carries are irrelevant here, and asking
 * for them would tie this module to a shape it does not read. */
export interface DataTableSummaryPoint {
  period: string;
}

/**
 * `Tabla de datos (98 periodos, de T1 2002 a T2 2026)`.
 *
 * The period labels come out in the PROSE register, not the compact one the
 * table's own `Periodo` column uses. `lib/format/period.ts` states the rule
 * that picks between them: compact where a column width or an axis tick is
 * load-bearing, prose where the label sits in a sentence. This label has a
 * line to itself and is read aloud as a sentence, so there is no width to save
 * and an abbreviation would be a saving nobody asked for.
 *
 * The count goes through `formatNumber` for the same reason every other
 * reader-facing number on this site does. No series reaches four digits today
 * (`ipc-general` is the longest at 294), so this changes nothing now and stays
 * correct if one ever does.
 *
 * READS, NEVER WRITES. The periods handed in are the same strings the island
 * sorts, slices and joins its `period → status` maps on; this returns display
 * text and stores nothing, which is `lib/format/period.ts`'s "format at the
 * render site, never in the data" boundary applied at one more call site.
 */
export function dataTableSummaryLabel(points: readonly DataTableSummaryPoint[]): string {
  if (points.length === 0) return es.chart.tableDisclosure.empty;

  const from = formatPeriodProse(points[0].period);
  if (points.length === 1) return es.chart.tableDisclosure.single(from);

  const to = formatPeriodProse(points[points.length - 1].period);
  return es.chart.tableDisclosure.span(formatNumber(points.length, 0), from, to);
}
