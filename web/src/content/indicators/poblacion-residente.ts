// series-transformations spec's applicability table: population is the
// per-capita DENOMINATOR itself — dividing it by itself is meaningless, so
// per capita is absent here even though it is offered for two of the other
// five series.
import type { IndicatorContentConfig } from "./types";

export const poblacionResidente: IndicatorContentConfig = {
  slug: "poblacion-residente",
  name: "Población residente",
  unit: "personas",
  frequency: "Q",
  decimals: 0,
  transforms: { yoy: "optional", qoq: "optional", perCapita: false },
};
