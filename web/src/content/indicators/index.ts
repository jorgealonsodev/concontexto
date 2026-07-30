// Slug-keyed aggregate of the six frozen indicator routes' presentation
// config (indicator-page spec, "Six indicator routes with frozen slugs").
// Slice 9a's page template is this module's first real consumer; the
// workbench and this slice's own tests are its first consumers today.
import type { IndicatorContentConfig } from "./types";
import { tasaDeParoEpa } from "./tasa-de-paro-epa";
import { ocupadosEpa } from "./ocupados-epa";
import { ipcGeneral } from "./ipc-general";
import { ipcSubyacente } from "./ipc-subyacente";
import { pib } from "./pib";
import { poblacionResidente } from "./poblacion-residente";

export const INDICATOR_CONTENT: Record<string, IndicatorContentConfig> = {
  "tasa-de-paro-epa": tasaDeParoEpa,
  "ocupados-epa": ocupadosEpa,
  "ipc-general": ipcGeneral,
  "ipc-subyacente": ipcSubyacente,
  pib,
  "poblacion-residente": poblacionResidente,
};

/**
 * Reverse of `IndicatorContentConfig.artifactSlug`: resolves the EXPORT
 * ARTIFACT's own series slug to the frozen public ROUTE slug, or `null` when
 * no route serves that series.
 *
 * Needed because the two identifier spaces are not the same one: `pib`'s
 * artifact document is slugged `pib-cvi` (see `pib.ts` and
 * `[slug].astro`'s own doc comments). Any artifact-supplied slug that must
 * become a link — today, the discontinued state's `successorSlug`
 * (verify-report CRITICAL-4) — has to go through this, or it would render a
 * `/indicador/pib-cvi` href that 404s. Returning `null` rather than falling
 * back to the raw value is the point: the caller must be able to tell "this
 * series has a page" from "it does not", and render no link at all in the
 * second case rather than a dangling one.
 */
export function routeSlugForArtifactSlug(artifactSlug: string): string | null {
  for (const [routeSlug, config] of Object.entries(INDICATOR_CONTENT)) {
    if ((config.artifactSlug ?? config.slug) === artifactSlug) return routeSlug;
  }
  return null;
}

export type { IndicatorContentConfig } from "./types";
