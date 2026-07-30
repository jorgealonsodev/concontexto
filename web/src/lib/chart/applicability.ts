// series-transformations spec, "A transformation that does not apply does
// not exist" + "Transformation applicability for the six milestone-1.2
// series" (the exact applicability table). A control is visible only when
// BOTH (a) the series' own static, editorial configuration
// (`web/src/content/indicators/{slug}.ts`) says the transformation applies,
// AND (b) the transformation is genuinely computable over the real data —
// never rendered merely because the config allows it if the actual
// computation yields nothing, and never rendered disabled (a non-applicable
// or non-computable transformation simply does not exist in the markup).
//
// "Real"/nominal and "median" apply to none of the six configured series
// (spec: "applies to none of the six") — deliberately absent from this
// type; a future series with a real config field for either would need a
// new field here, not a boolean bolted onto every existing series.
import type { ToggleableTransform } from "./toggleState";

/** Whether YoY/QoQ is offered at all for this series, and whether the spec
 * itself calls that offering "mandatory" (documentation only, per D-5/tasks
 * reconciliation — both mandatory and optional controls render identically
 * once genuinely computable; "mandatory" records WHY the control exists
 * per PRD §7, not a different rendering rule). */
export type TransformOffering = "mandatory" | "optional" | false;

export interface IndicatorTransformConfig {
  yoy: TransformOffering;
  qoq: TransformOffering;
  perCapita: boolean;
}

export interface ApplicabilityInput {
  config: IndicatorTransformConfig;
  yoyPointCount: number;
  qoqPointCount: number;
  perCapitaCoverage: { from: string; to: string } | null;
}

export function visibleTransformControls(input: ApplicabilityInput): ToggleableTransform[] {
  const { config, yoyPointCount, qoqPointCount, perCapitaCoverage } = input;
  const visible: ToggleableTransform[] = [];
  if (config.yoy !== false && yoyPointCount > 0) visible.push("yoy");
  if (config.qoq !== false && qoqPointCount > 0) visible.push("qoq");
  if (config.perCapita && perCapitaCoverage !== null) visible.push("perCapita");
  return visible;
}
