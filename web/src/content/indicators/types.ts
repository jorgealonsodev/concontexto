// Per-page presentation configuration (design.md D-5: "which pages carry
// [YoY], and each page's default view, are presentation config in
// `web/src/content/indicators/{slug}.ts`... pipeline data and editorial
// presentation stay separate, P1"). This module declares WHICH
// transformations a series' page offers — series-transformations spec's own
// applicability table — never the series' actual observation data, which
// always comes from the export artifact (slice 9a's loader).
import type { IndicatorTransformConfig } from "../../lib/chart/applicability";
import type { Frequency } from "../../lib/chart/periods";

export interface IndicatorContentConfig {
  slug: string;
  name: string;
  unit: string;
  frequency: Frequency;
  decimals: number;
  transforms: IndicatorTransformConfig;
  /**
   * Slug the export artifact carries for this series, when it differs from
   * the frozen route `slug` above. Real, disclosed gap discovered in slice
   * 9b: `config/series/pib-cvi.yaml`'s Go-pipeline slug is `pib-cvi`, but
   * indicator-page spec's frozen route table names this page `pib`. All six
   * OTHER slug/series identities coincide; only PIB diverges. Undefined
   * means "identical to `slug`" (every other series). See
   * `web/src/pages/indicador/[slug].astro`'s own doc comment for how this
   * is resolved without touching the Go pipeline (out of this slice's file
   * scope) and design.md's corresponding Open Question.
   */
  artifactSlug?: string;
}
