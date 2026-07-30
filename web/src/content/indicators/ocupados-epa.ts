// series-transformations spec's applicability table: a headcount, so per
// capita is offered over the population series' covered sub-span.
import type { IndicatorContentConfig } from "./types";

export const ocupadosEpa: IndicatorContentConfig = {
  slug: "ocupados-epa",
  name: "Ocupados",
  unit: "personas",
  frequency: "Q",
  decimals: 0,
  transforms: { yoy: "optional", qoq: "optional", perCapita: true },
};
