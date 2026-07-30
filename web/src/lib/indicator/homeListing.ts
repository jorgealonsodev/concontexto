// What the homepage lists, derived from the same export artifact every
// indicator page is built from (milestone 1.2). The counterpart of
// `routes.ts` for `/`: that module decides which pages the build EMITS,
// this one decides which of them `/` OFFERS.
//
// WHY IT DELEGATES TO `resolveIndicatorRouteSlugs` INSTEAD OF LISTING
// WHATEVER IT FINDS. The homepage is the only place on this site where a
// reader learns that an indicator exists. `/indicador/{slug}` is reachable
// by typing a permalink; `/` is reachable by arriving. So the failure mode
// of a homepage that lists five of six is strictly worse than the failure
// mode `routes.ts` was written to prevent: a missing PAGE 404s loudly for
// anyone holding the link, while a missing ROW is invisible — the site
// looks complete, and the indicator that vanished is precisely the one
// nothing else on the site now mentions.
//
// Reusing the guard rather than re-deriving the list is also the concrete
// lesson of verify-report CRITICAL-27. That defect was not "someone wrote a
// bad filter"; it was "the route list was derived twice, and the second
// derivation was weaker than the first". A homepage with its own
// `Object.keys(INDICATOR_CONTENT)` walk would be the third derivation, and
// it would drift the same way. There is one function that answers "which
// indicators does this build serve", it returns all six or it throws, and
// both surfaces call it.
//
// The consequence is deliberate and currently live: `ocupados-epa` is held
// back by the ingestion publish gate, so the artifact carries no document
// for it and a production build of this site FAILS — homepage included. It
// is meant to. Shipping a homepage that silently dropped a frozen indicator
// would turn a pipeline outage into a quieter, longer-lived product defect.
import type { SeriesDoc, ArtifactFreshness } from "../export/schema";
import { INDICATOR_CONTENT } from "../../content/indicators";
import { artifactSlugFor, resolveIndicatorRouteSlugs, type IndicatorRouteCatalogs } from "./routes";

/**
 * One row of the homepage listing — deliberately the exact prop shape
 * `IndicatorCard.astro` already accepts, so the page spreads it straight
 * onto the component and no second card model exists to drift from the
 * first. `decimals` is required here (not optional as on the component)
 * because every frozen indicator's content entry declares it; leaving it
 * optional would let a caller fall through to the component's default of 1
 * and silently misreport PIB, which is configured to four.
 */
export interface IndicatorListingEntry {
  slug: string;
  name: string;
  unit: string;
  latestValue: number | null;
  latestPeriod: string;
  freshness: ArtifactFreshness;
  decimals: number;
}

function emptySeries(routeSlug: string, artifactSlug: string): string {
  return (
    `home listing: "${routeSlug}" (artifact slug "${artifactSlug}") is present in the export artifact but carries ` +
    "no observations, so there is no latest value to list for it.\n" +
    "  `concontexto export` skips a zero-observation series outright (app/internal/publishing/export.go), so an\n" +
    "  artifact that contains one means the export contract changed. This build refuses to render a card for it.\n" +
    "  A card showing a placeholder dash under a real indicator's name asserts a measurement nobody published (P4),\n" +
    "  and the reader has no way to tell that card apart from one whose source genuinely reported nothing."
  );
}

/**
 * The six rows `/` renders, or a thrown error naming what is missing.
 *
 * @param seriesBySlug `loadExportArtifact`'s own map, keyed by ARTIFACT
 * slug — `pib`'s document is keyed `pib-cvi`, which is why every lookup
 * goes through `artifactSlugFor`.
 * @param catalogs the same injection seam `resolveIndicatorRouteSlugs`
 * takes, so both the guard and this mapping can be tested against catalog
 * shapes that do not exist in the repository today. Production passes none.
 */
export function homeIndicatorListing(
  seriesBySlug: ReadonlyMap<string, SeriesDoc>,
  catalogs: IndicatorRouteCatalogs = {},
): IndicatorListingEntry[] {
  const indicators = catalogs.indicators ?? INDICATOR_CONTENT;

  // All six or nothing. Everything below may therefore index without a
  // fallback: a slug this returned is a slug with both a content entry and
  // an artifact document, by construction.
  const routeSlugs = resolveIndicatorRouteSlugs(seriesBySlug, catalogs);

  return routeSlugs.map((routeSlug) => {
    const artifactSlug = artifactSlugFor(routeSlug, indicators);
    const doc = seriesBySlug.get(artifactSlug)!;
    const content = indicators[routeSlug];
    const latest = doc.points[doc.points.length - 1];
    if (latest === undefined) throw new Error(emptySeries(routeSlug, artifactSlug));

    return {
      slug: routeSlug,
      // The editorial name and unit come from the content catalog, not from
      // the artifact, for the same reason `IndicatorPage.astro`'s header
      // does it: those are presentation decisions (P1), and the two
      // surfaces must read identically or a reader who follows the link
      // sees the indicator renamed under them.
      name: content.name,
      unit: content.unit,
      decimals: content.decimals,
      // The observation and its freshness come from the artifact, never
      // from the catalog — the whole point of the semaphore is that it
      // reports a fact about the pipeline that no editorial file can
      // override (verify-report CRITICAL-4).
      latestValue: latest.value,
      latestPeriod: latest.period,
      freshness: doc.freshness,
    };
  });
}
