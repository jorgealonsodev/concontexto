// series-transformations spec's applicability table: a headcount, so per
// capita is offered over the population series' covered sub-span.
//
// `unit` READ "personas" UNTIL THIS CHANGE, AND THAT WAS A PUBLISHED ERROR.
// INE reports this series in thousands and says so twice in the same API
// response (`DATOS_SERIE/EPA387796`: `T3_Unidad: "Personas"` alongside
// `T3_Escala: "Miles"`, beside `Valor: 22779.0`); `config/series/ocupados-epa.yaml`
// carries `unit: "miles de personas"` and the export artifact repeats it.
// Only this file disagreed, and this file is the one the page header and the
// homepage card read — so the site published "22779 personas" for 22.8
// million people, wrong by a factor of a thousand.
//
// The value itself is NOT rescaled to compensate. INE's scale is real, the
// Go pipeline stores what INE published, and dividing or multiplying here to
// make a wrong label true would be inventing a figure nobody reported (P4).
// `resolveIndicatorRouteSlugs` now fails the build if this label and the
// artifact's ever contradict each other again.
import type { IndicatorContentConfig } from "./types";

export const ocupadosEpa: IndicatorContentConfig = {
  slug: "ocupados-epa",
  name: "Ocupados",
  unit: "miles de personas",
  frequency: "Q",
  decimals: 0,
  transforms: { yoy: "optional", qoq: "optional", perCapita: true },
};
