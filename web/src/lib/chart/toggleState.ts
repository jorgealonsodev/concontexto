// Chart island interaction primitive: the single-active-transform toggle
// (series-transformations spec — YoY, QoQ and per-capita are mutually
// exclusive VIEWS over one chart; this project never overlays two derived
// series on one axis). Selecting the currently-active transform again
// returns to the raw series — an ordinary pressed-toggle-button semantic,
// not a radio group that always keeps exactly one option selected.
export type TransformKind = "raw" | "yoy" | "qoq" | "perCapita";
export type ToggleableTransform = Exclude<TransformKind, "raw">;

export function reduceTransform(current: TransformKind, requested: TransformKind): TransformKind {
  return current === requested ? "raw" : requested;
}
