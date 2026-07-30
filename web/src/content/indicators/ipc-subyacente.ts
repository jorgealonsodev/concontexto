// series-transformations spec's applicability table: same shape as
// ipc-general — an index, year-on-year mandatory (PRD §7 #11).
import type { IndicatorContentConfig } from "./types";

export const ipcSubyacente: IndicatorContentConfig = {
  slug: "ipc-subyacente",
  name: "IPC subyacente",
  unit: "índice (base 2021=100)",
  frequency: "M",
  decimals: 2,
  transforms: { yoy: "mandatory", qoq: "optional", perCapita: false },
};
