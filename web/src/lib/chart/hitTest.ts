// Chart island interaction primitive (indicator-page spec tooltips;
// web-accessibility-gates spec, "Every chart point is reachable by
// keyboard"): resolves a pointer's x coordinate, already expressed in the
// SVG's own viewBox units (the caller scales from the pointer event's
// client coordinates using the rendered element's bounding box — a DOM
// concern this pure function does not own), to the index of the nearest
// plotted point. O(n) linear scan — chart series top out at ~294 points
// (ipc-general/ipc-subyacente's monthly history), cheap even on every
// pointermove.
import { xForIndex, type ChartDimensions } from "./geometry";

export function nearestPointIndexForX(pointerX: number, count: number, dims: ChartDimensions): number {
  if (count <= 0) return -1;
  let bestIndex = 0;
  let bestDistance = Infinity;
  for (let i = 0; i < count; i++) {
    const distance = Math.abs(xForIndex(i, count, dims) - pointerX);
    if (distance < bestDistance) {
      bestDistance = distance;
      bestIndex = i;
    }
  }
  return bestIndex;
}
