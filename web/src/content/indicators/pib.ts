// series-transformations spec's applicability table: PIB is the one series
// with BOTH mandatory rates (PRD §7 #14 names year-on-year AND
// quarter-on-quarter explicitly) plus per capita over the population
// series' covered sub-span.
//
// `artifactSlug: "pib-cvi"` — real, disclosed slug divergence found in slice
// 9b: `config/series/pib-cvi.yaml` is the Go pipeline's own series slug (and
// therefore the export artifact's `series/pib-cvi.json`), while
// indicator-page spec's frozen route table names this page `pib`. See
// `types.ts`'s `artifactSlug` doc comment and `[slug].astro`'s own comment
// for the resolution.
import type { IndicatorContentConfig } from "./types";

export const pib: IndicatorContentConfig = {
  slug: "pib",
  artifactSlug: "pib-cvi",
  name: "PIB (índice de volumen encadenado)",
  unit: "índice de volumen encadenado",
  frequency: "Q",
  decimals: 4,
  transforms: { yoy: "mandatory", qoq: "mandatory", perCapita: true },
};
