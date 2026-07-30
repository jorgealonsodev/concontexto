// indicator-page spec, Requirement "Six indicator routes with frozen
// slugs", Scenario "A slug change ships a permanent redirect": "no
// published permalink returns a not-found response". This change freezes
// and renames NONE of its six slugs -- `SLUG_REDIRECTS` is empty in
// production by design, not by omission (see the module-level constant's
// own doc comment). The MECHANISM a future rename would need is built and
// unit-tested here, not deferred to the day a rename actually happens
// under time pressure -- task 9b.2's own instruction: "exercised via a test
// fixture ... deliberately unused in production".
//
// `astro.config.mjs`'s own `redirects` option is Astro's native,
// static-build-safe permanent-redirect mechanism: with no SSR adapter
// configured (this project's exact deployment shape), Astro emits a static
// HTML page carrying a meta-refresh + canonical link for every configured
// entry, so a renamed slug's old permalink never 404s even without a
// server-level HTTP 301 rewrite (a real 301 MAY additionally come from the
// eventual reverse proxy, but that is out of this project's control and
// not required for "no permalink ever 404s" to hold). `buildAstroRedirects`
// below is the pure function translating THIS project's own from->to slug
// map into the shape that option expects, kept unit-testable independently
// of a real Astro build.

/** Previous slug -> current slug. Empty in production: no slug has ever
 * changed in this project (all six are frozen by this very change). */
export const SLUG_REDIRECTS: Record<string, string> = {};

/**
 * Translates a from->to slug map into Astro's `redirects` config shape,
 * anchored under `/indicador/`. Exported so both `astro.config.mjs`
 * (production, fed `SLUG_REDIRECTS`) and this module's own test (fed a
 * fixture map, per task 9b.2's own instruction) exercise the identical
 * function -- the mechanism is proven even though production feeds it an
 * empty map.
 */
export function buildAstroRedirects(slugMap: Record<string, string> = SLUG_REDIRECTS): Record<string, string> {
  const entries: Record<string, string> = {};
  for (const [from, to] of Object.entries(slugMap)) {
    entries[`/indicador/${from}`] = `/indicador/${to}`;
  }
  return entries;
}
