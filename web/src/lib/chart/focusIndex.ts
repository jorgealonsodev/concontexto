// Chart island interaction primitive (web-accessibility-gates spec,
// "Every chart point is reachable by keyboard alone... focus is visible at
// every step" / "No keyboard trap"): pure roving-tabindex focus-index
// arithmetic. Deliberately CLAMPS at the ends rather than wrapping — no key
// this chart binds ever moves focus outside `[0, count-1]`, so Tab always
// leaves the chart via the browser's own natural document order instead of
// being caught in a wrap-around loop.
export type ChartNavKey = "ArrowRight" | "ArrowLeft" | "Home" | "End";

export function nextFocusIndex(current: number, key: ChartNavKey, count: number): number {
  if (count <= 0) return -1;
  switch (key) {
    case "ArrowRight":
      return Math.min(count - 1, current + 1);
    case "ArrowLeft":
      return Math.max(0, current - 1);
    case "Home":
      return 0;
    case "End":
      return count - 1;
  }
}
