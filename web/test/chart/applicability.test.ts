// series-transformations spec, "A transformation that does not apply does
// not exist" + "Transformation applicability for the six milestone-1.2
// series" (the exact table). This is the pure decision function task 8.1's
// control-visibility test exercises: a control is visible only when BOTH
// (a) the series' own static configuration says the transformation applies,
// AND (b) the transformation is genuinely computable over the real data (a
// non-empty result) — never rendered merely because the config says so if
// the actual computation yields nothing, and never rendered disabled.
import { describe, expect, it } from "vitest";
import { visibleTransformControls } from "../../src/lib/chart/applicability";
import { INDICATOR_CONTENT } from "../../src/content/indicators";
import { computeIntraPeriodRate, computeYoY } from "../../src/lib/transform/yoy";
import { computePerCapita } from "../../src/lib/transform/perCapita";
import * as fx from "../../src/workbench/fixtures";

describe("visibleTransformControls", () => {
  it("a non-applicable transformation has no control at all (tasa-de-paro-epa: no per-capita)", () => {
    const config = INDICATOR_CONTENT["tasa-de-paro-epa"].transforms;
    const visible = visibleTransformControls({
      config,
      yoyPointCount: computeYoY(fx.indicatorChart.points, "Q").length,
      qoqPointCount: computeIntraPeriodRate(fx.indicatorChart.points, "Q").length,
      perCapitaCoverage: null,
    });
    expect(visible).not.toContain("perCapita");
  });

  it("pib offers both YoY and QoQ (both mandatory)", () => {
    const config = INDICATOR_CONTENT["pib"].transforms;
    const visible = visibleTransformControls({
      config,
      yoyPointCount: computeYoY(fx.indicatorChart.points, "Q").length,
      qoqPointCount: computeIntraPeriodRate(fx.indicatorChart.points, "Q").length,
      perCapitaCoverage: { from: "2019-Q1", to: "2020-Q1" },
    });
    expect(visible).toContain("yoy");
    expect(visible).toContain("qoq");
  });

  it("ipc-general, ipc-subyacente and pib all show YoY as present (mandatory)", () => {
    for (const slug of ["ipc-general", "ipc-subyacente", "pib"] as const) {
      const config = INDICATOR_CONTENT[slug].transforms;
      expect(config.yoy).not.toBe(false);
    }
  });

  it("real/median never appear in the static config for any of the six series", () => {
    for (const config of Object.values(INDICATOR_CONTENT)) {
      expect(config.transforms).not.toHaveProperty("real");
      expect(config.transforms).not.toHaveProperty("median");
    }
  });

  it("a control does not render when the config allows it but the computation yields zero points", () => {
    const config = INDICATOR_CONTENT["ocupados-epa"].transforms;
    const visible = visibleTransformControls({
      config,
      yoyPointCount: 0, // simulates a series too short for any YoY point
      qoqPointCount: 0,
      perCapitaCoverage: null,
    });
    expect(visible).toEqual([]);
  });

  it("a control does not render when the computation yields data but the config says it does not apply", () => {
    const config = INDICATOR_CONTENT["tasa-de-paro-epa"].transforms; // perCapita: false
    const visible = visibleTransformControls({
      config,
      yoyPointCount: 10,
      qoqPointCount: 10,
      perCapitaCoverage: { from: "2020-Q1", to: "2021-Q1" }, // hypothetically computable, still config-gated off
    });
    expect(visible).not.toContain("perCapita");
  });

  it("every one of the six configured series has a transforms config with no `real` or `median` field and at least yoy or qoq offered", () => {
    const slugs = Object.keys(INDICATOR_CONTENT);
    expect(slugs).toHaveLength(6);
    for (const config of Object.values(INDICATOR_CONTENT)) {
      expect(config.transforms.yoy !== false || config.transforms.qoq !== false).toBe(true);
    }
  });
});
