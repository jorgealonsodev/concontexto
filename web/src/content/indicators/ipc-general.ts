// series-transformations spec's applicability table: the configuration
// stores the price INDEX; year-on-year is mandatory (PRD §7 #10 — the
// headline figure is the annual rate, not the raw index). Per capita does
// not apply to a price index.
import type { IndicatorContentConfig } from "./types";

export const ipcGeneral: IndicatorContentConfig = {
  slug: "ipc-general",
  name: "IPC general",
  unit: "índice (base 2021=100)",
  frequency: "M",
  decimals: 2,
  transforms: { yoy: "mandatory", qoq: "optional", perCapita: false },
};
