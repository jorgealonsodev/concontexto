// series-transformations spec's applicability table: a rate, so per capita
// does not apply (dividing a percentage by population is meaningless).
import type { IndicatorContentConfig } from "./types";

export const tasaDeParoEpa: IndicatorContentConfig = {
  slug: "tasa-de-paro-epa",
  name: "Tasa de paro",
  unit: "% población activa",
  frequency: "Q",
  decimals: 2,
  transforms: { yoy: "optional", qoq: "optional", perCapita: false },
};
