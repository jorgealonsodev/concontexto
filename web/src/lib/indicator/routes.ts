// Route resolution for `/indicador/{slug}` — which pages the build emits,
// and what happens when it cannot emit one of them.
//
// WHY THIS IS A MODULE AND NOT SIX LINES INSIDE `[slug].astro`. It used to
// be six lines inside `[slug].astro`, and they read:
//
//   const slugs = Object.keys(INDICATOR_CONTENT).filter(
//     (slug) => seriesBySlug.has(artifactSlugFor(slug)) && METHODOLOGY_CONTENT[slug] !== undefined,
//   );
//
// That is verify-report CRITICAL-27. A frozen slug the export artifact does
// not carry was FILTERED OUT, and no layer objected:
// `app/internal/publishing/export.go` skips a series with zero observations
// (`if len(obs) == 0 { continue }`), so the slug is written into neither
// `series/` nor `manifest.series` — the digest chain is complete and the
// Zod loader validates cleanly. The build exited 0, emitted five pages, and
// `/indicador/ocupados-epa/` returned 404 on a permalink the spec froze
// permanently ("Six indicator routes with frozen slugs": the build MUST
// produce `/indicador/{slug}` for exactly these six... frozen by this change
// because permalinks are a permanent commitment; and "A slug change ships a
// permanent redirect": "no published permalink returns a not-found
// response").
//
// SILENTLY SHIPPING A SITE THAT IS MISSING A ROUTE IT PROMISED IS WORSE
// THAN NOT SHIPPING. A filter turns a pipeline failure into a smaller
// website, and nothing anywhere notices — not the digest chain, not the
// schema, not CI, not a reader, who simply gets a 404 on a URL that was
// promised to be permanent. Failing the build puts the failure in front of
// the one person who can fix it, at the moment it becomes true.
//
// THE ALTERNATIVE, CONSIDERED AND REJECTED: a fourth "no data yet" page
// state alongside fresh / validation-failure / discontinued, so the route
// would still exist. It does not work here. The indicator-page spec
// separately requires that the chart is never hidden — "Series breaks are
// always visible and never dismissible", "The default range is the full
// series", and the anatomy requirement's "main interactive chart at full
// width on mobile" — and a series with zero observations has no chart to
// show. The page would be a promise with nothing behind it.
//
// It lives in `src/lib/` rather than in the `.astro` file so the rule can
// be asserted directly (`test/indicator/routes.test.ts`). The previous
// all-six assertion was `.github/workflows/ingest-export-build.yml`'s
// dist/ loop, which builds from a fixture that always contains all six —
// it exercises the real build but can never receive the failing input, so
// it could not go red for this defect. A rule that cannot be shown failing
// is not a check.
import { INDICATOR_CONTENT, type IndicatorContentConfig } from "../../content/indicators";
import { METHODOLOGY_CONTENT, type MethodologyContent } from "../../content/indicators/methodology";

/** The six permalinks indicator-page spec freezes, in the order the spec
 * lists them. Frozen means permanent: a slug that changes ships a redirect
 * from the old path (`redirects.ts`), it is never simply dropped.
 *
 * Resolution iterates THIS list, never `Object.keys(INDICATOR_CONTENT)`.
 * The difference is the whole fix: deriving the route list from whatever
 * configuration happens to exist means any gap upstream quietly becomes a
 * smaller site, while deriving it from the frozen list means a gap is a
 * failure with a name. */
export const FROZEN_INDICATOR_SLUGS = [
  "tasa-de-paro-epa",
  "ocupados-epa",
  "ipc-general",
  "ipc-subyacente",
  "pib",
  "poblacion-residente",
] as const;

/** Injection seam for the two catalogs, so the guard can be tested against
 * a gap that does not exist in the repository today. Production passes
 * neither and gets the real modules. */
export interface IndicatorRouteCatalogs {
  indicators?: Record<string, IndicatorContentConfig>;
  methodology?: Record<string, MethodologyContent>;
}

/**
 * The slug the EXPORT ARTIFACT uses for a frozen ROUTE slug.
 *
 * Identical for five of the six; `pib`'s document is slugged `pib-cvi`,
 * the Go pipeline's own slug from `config/series/pib-cvi.yaml` (see
 * `IndicatorContentConfig.artifactSlug`). Every artifact lookup has to
 * cross that boundary, which is exactly why the lookup is one named
 * function rather than repeated inline.
 */
export function artifactSlugFor(
  routeSlug: string,
  indicators: Record<string, IndicatorContentConfig> = INDICATOR_CONTENT,
): string {
  return indicators[routeSlug]?.artifactSlug ?? routeSlug;
}

function missingFromArtifact(routeSlug: string, artifactSlug: string, available: string[]): string {
  return (
    `  - "${routeSlug}" has no series in the export artifact.\n` +
    `    It looks for artifact slug "${artifactSlug}"; the artifact carries: ${available.join(", ") || "(nothing)"}.\n` +
    "    THIS IS A PIPELINE PROBLEM, not a problem in the web tree, and there are exactly two causes:\n" +
    "      (a) the series has never been published — `concontexto export` skips a series with zero\n" +
    "          observations, so it is written into neither series/ nor manifest.series; or\n" +
    "      (b) its latest ingestion run was held back by the publish gate — a validation rule\n" +
    "          (rule1-schema, rule2-continuity, rule3-plausibility, rule4-revision) blocked the run,\n" +
    "          so no new observations were committed for it.\n" +
    "    Check the ingestion run's outcome for this series first. Do not edit this file to make the\n" +
    "    build pass: the route is frozen, and a site without it is a site that 404s a permanent URL."
  );
}

function missingMethodology(routeSlug: string): string {
  return (
    `  - "${routeSlug}" has no entry in METHODOLOGY_CONTENT.\n` +
    "    Its series IS present in the export artifact, so this is a CONTENT-AUTHORING gap, not a\n" +
    "    pipeline failure: someone has to write the methodology sheet. Add the entry in\n" +
    "    web/src/content/indicators/methodology.ts (measures, doesNotMeasure, ingestionScriptHref,\n" +
    "    nextPublicationHref, relatedSlugs)."
  );
}

function missingContentConfig(routeSlug: string): string {
  return (
    `  - "${routeSlug}" has no entry in INDICATOR_CONTENT.\n` +
    "    Also a CONTENT-AUTHORING gap: add web/src/content/indicators/" +
    `${routeSlug}.ts and register it in\n` +
    "    web/src/content/indicators/index.ts (name, unit, frequency, decimals, transforms — plus\n" +
    "    artifactSlug if the Go pipeline slugs the series differently)."
  );
}

/**
 * The unit a series is MEASURED in, read off its artifact document.
 *
 * This is the only field route resolution reads out of a series document,
 * which is why the parameter below is typed to exactly it rather than to
 * `SeriesDoc`: a test still supplies one string per slug, not several
 * hundred lines of provenance nobody asserts on.
 */
export interface ArtifactUnitRef {
  unit: string;
}

/** Trims and NFC-normalises a unit label before comparison.
 *
 * `índice` can be written with a precomposed `í` or with an `i` plus a
 * combining acute. The two are byte-different and reader-identical, and a
 * guard that failed on the difference would send someone hunting for a
 * discrepancy their editor cannot show them. Normalising both sides means
 * this rule only ever fires on a difference a reader could actually see. */
function normaliseUnit(unit: string): string {
  return unit.normalize("NFC").trim();
}

/**
 * Whether an editorial unit label may stand in front of the unit the
 * pipeline actually measured in.
 *
 * THE RULE: editorial copy may only ELABORATE the artifact's unit. It may
 * append (a base, a qualifier); it may never replace, shorten or rescale.
 *
 * NOT string equality, because two of the six labels legitimately differ
 * and must keep differing: the artifact says `índice` for both IPC series
 * while the pages say `índice (base 2021=100)`, because the artifact's own
 * `base` field is a disclosed gap (`export/schema.ts`: "always null until a
 * future slice adds index-base config") and the editorial layer is what
 * tells a reader which base an index sits on. A guard demanding equality
 * would delete that base from two public pages in order to pass.
 *
 * The appended part must begin with a space or an opening parenthesis. That
 * keeps the rule at "the pipeline's unit, then more" rather than "any string
 * with the right prefix", under which `índicex` — a different word — would
 * qualify.
 */
function elaborates(editorialUnit: string, artifactUnit: string): boolean {
  const editorial = normaliseUnit(editorialUnit);
  const artifact = normaliseUnit(artifactUnit);
  if (editorial === artifact) return true;
  if (!editorial.startsWith(artifact)) return false;
  const remainder = editorial.slice(artifact.length);
  return remainder.startsWith(" ") || remainder.startsWith("(");
}

function contradictedUnit(routeSlug: string, editorialUnit: string, artifactUnit: string): string {
  return (
    `  - "${routeSlug}" is labelled "${editorialUnit}" on the page, but the artifact measured it in "${artifactUnit}".\n` +
    "    THIS IS AN EDITORIAL PROBLEM, not a pipeline one: the artifact is the measurement, and this\n" +
    `    label is prose restating it. Fix web/src/content/indicators/${routeSlug}.ts, never the artifact.\n` +
    "    A label may only ELABORATE the measured unit — append a base or a qualifier after a space or a\n" +
    "    parenthesis, the way `índice` becomes `índice (base 2021=100)`. It may not replace it, shorten\n" +
    "    it, or drop its scale.\n" +
    "    Dropping a scale is why this check exists. `ocupados-epa` was labelled \"personas\" against an\n" +
    "    artifact reading \"miles de personas\", so the header and the homepage card published 22779 as a\n" +
    "    headcount when it means 22.8 million people — a published figure wrong by a factor of a\n" +
    "    thousand, which no schema, digest or route guard could see, because the unit was derived twice\n" +
    "    and only the weaker derivation was ever rendered."
  );
}

function notFrozen(routeSlugs: string[]): string {
  return (
    `  - ${routeSlugs.map((slug) => `"${slug}"`).join(", ")} ${routeSlugs.length === 1 ? "is" : "are"} configured in ` +
    "INDICATOR_CONTENT but not frozen.\n" +
    "    The spec fixes EXACTLY six routes, so this build emits none of them rather than quietly\n" +
    "    ignoring authored content. Adding a seventh public indicator page is a deliberate decision:\n" +
    "    add the slug to FROZEN_INDICATOR_SLUGS in this file, and update the spec that froze the six."
  );
}

/**
 * Resolves the route slugs `/indicador/[slug].astro` must build, or throws.
 *
 * Returns `FROZEN_INDICATOR_SLUGS` verbatim on success — there is no
 * shorter successful result, by construction. Every failure names the slug,
 * says it is one of the six frozen routes, and distinguishes WHICH of the
 * two catalogs plus the artifact fell short, because those are different
 * failures with different fixes: a slug missing from the ARTIFACT is fixed
 * upstream in the Go pipeline, a slug missing from a CONTENT catalog is
 * fixed by writing prose in this repository. Collapsing them into one
 * boolean (which is what the filter did) sends the reader to the wrong
 * place, or to no place at all.
 *
 * All problems are reported together rather than one per build: a build
 * that surfaces one missing slug at a time turns a two-slug outage into two
 * rounds of CI.
 *
 * IT ALSO CHECKS THAT THE PAGES' UNIT LABELS DO NOT CONTRADICT THE
 * ARTIFACT, and that is here rather than in a module of its own on purpose.
 * `ocupados-epa` was labelled "personas" while its artifact — and
 * `config/series/ocupados-epa.yaml`, and INE's own API response
 * (`T3_Unidad: "Personas"`, `T3_Escala: "Miles"`) — all said
 * "miles de personas", so the site published a headcount a thousand times
 * too small. That is CRITICAL-27's shape again: a fact derived twice, with
 * nothing comparing the two derivations. The lesson of CRITICAL-27 was that
 * a SECOND guard function is a second thing to forget to call, so this rule
 * lives inside the one function `/` and `/indicador/{slug}` already both
 * reach — via `homeIndicatorListing` and `getStaticPaths` respectively —
 * and there is no weaker sibling to call instead.
 *
 * Only `unit` is checked, and NOT `decimals`, which the two layers also
 * both carry and which currently disagrees for three series (see this
 * module's own note in the repository's change report). That is deliberate:
 * `decimals` is display PRECISION, and rounding a published value for a
 * reader never changes what the value means. `unit` is DIMENSION, and
 * relabelling it changes the measurement itself. Guarding the second is
 * correctness; guarding the first would be freezing an editorial choice.
 *
 * @param seriesBySlug `loadExportArtifact`'s own map, keyed by ARTIFACT
 * slug. Typed to the one field this function reads (`unit`) rather than to
 * `SeriesDoc`: the presence check needs no fields at all, and the unit
 * check needs exactly one, so a test still supplies a one-key object per
 * slug instead of synthesising full documents.
 */
export function resolveIndicatorRouteSlugs(
  seriesBySlug: ReadonlyMap<string, ArtifactUnitRef>,
  catalogs: IndicatorRouteCatalogs = {},
): string[] {
  const indicators = catalogs.indicators ?? INDICATOR_CONTENT;
  const methodology = catalogs.methodology ?? METHODOLOGY_CONTENT;

  const problems: string[] = [];

  for (const routeSlug of FROZEN_INDICATOR_SLUGS) {
    if (indicators[routeSlug] === undefined) {
      problems.push(missingContentConfig(routeSlug));
      continue;
    }
    const artifactSlug = artifactSlugFor(routeSlug, indicators);
    const doc = seriesBySlug.get(artifactSlug);
    if (doc === undefined) {
      problems.push(missingFromArtifact(routeSlug, artifactSlug, [...seriesBySlug.keys()].sort()));
      continue;
    }
    if (methodology[routeSlug] === undefined) {
      problems.push(missingMethodology(routeSlug));
    }
    const editorialUnit = indicators[routeSlug].unit;
    if (!elaborates(editorialUnit, doc.unit)) {
      problems.push(contradictedUnit(routeSlug, editorialUnit, doc.unit));
    }
  }

  const unfrozen = Object.keys(indicators).filter(
    (slug) => !(FROZEN_INDICATOR_SLUGS as readonly string[]).includes(slug),
  );
  if (unfrozen.length > 0) problems.push(notFrozen(unfrozen));

  if (problems.length > 0) {
    throw new Error(
      `indicator routes: this build cannot produce all ${FROZEN_INDICATOR_SLUGS.length} frozen indicator routes, so it refuses to produce any of them.\n` +
        `  The frozen routes are: ${FROZEN_INDICATOR_SLUGS.join(", ")}.\n` +
        "  They are permalinks (indicator-page spec, \"Six indicator routes with frozen slugs\"), and the same spec\n" +
        "  requires that \"no published permalink returns a not-found response\". Shipping the routes that happen to\n" +
        "  resolve would put a 404 on a URL this project promised to keep, and nothing downstream would report it —\n" +
        "  which is precisely what used to happen here (verify-report CRITICAL-27).\n" +
        `\n${problems.join("\n")}\n`,
    );
  }

  return [...FROZEN_INDICATOR_SLUGS];
}
