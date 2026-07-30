// web-accessibility-gates spec, "The transferred-bytes budget is a blocking
// gate" (task 9b.8, RED-first: this is the pure computation half).
import { describe, expect, it } from "vitest";
import {
  TRANSFERRED_BYTES_BUDGET,
  WORKBENCH_PROBE_PATH,
  LighthouseMeasurementError,
  WorkbenchBuildMeasuredError,
  assertProductionBuild,
  assertRealPageLoad,
  computeTransferredBytesExcludingFonts,
  evaluatePageBudget,
} from "../../src/lib/budget/transferredBytes";

describe("computeTransferredBytesExcludingFonts", () => {
  it("sums transferSize across every non-Font request", () => {
    const total = computeTransferredBytesExcludingFonts([
      { resourceType: "Document", transferSize: 1000 },
      { resourceType: "Script", transferSize: 2000 },
      { resourceType: "Stylesheet", transferSize: 500 },
    ]);
    expect(total).toBe(3500);
  });

  it("excludes Font resources explicitly — the spec's named typeface carve-out", () => {
    const total = computeTransferredBytesExcludingFonts([
      { resourceType: "Document", transferSize: 1000 },
      { resourceType: "Font", transferSize: 50_000 },
      { resourceType: "Font", transferSize: 30_000 },
    ]);
    expect(total).toBe(1000);
  });

  it("treats a missing transferSize as zero rather than throwing", () => {
    const total = computeTransferredBytesExcludingFonts([{ resourceType: "Image" }]);
    expect(total).toBe(0);
  });

  it("returns zero for an empty request list", () => {
    expect(computeTransferredBytesExcludingFonts([])).toBe(0);
  });
});

describe("evaluatePageBudget", () => {
  it("passes a page under the 300 KB budget, excluding fonts", () => {
    const result = evaluatePageBudget("/indicador/tasa-de-paro-epa", [
      { resourceType: "Document", transferSize: 100 * 1024 },
      { resourceType: "Font", transferSize: 250 * 1024 }, // would blow the budget if counted
    ]);
    expect(result.withinBudget).toBe(true);
    expect(result.transferredBytes).toBe(100 * 1024);
  });

  it("fails a page at or over the 300 KB budget, excluding fonts", () => {
    const result = evaluatePageBudget("/indicador/pib", [
      { resourceType: "Document", transferSize: TRANSFERRED_BYTES_BUDGET },
    ]);
    expect(result.withinBudget).toBe(false);
  });
});

// verify-report CRITICAL-2 remediation: `check-lighthouse-budget.mjs` printed
// "PASS ... 0.0 KB transferred" and exited 0 against a server that was not
// even listening, because `details?.items ?? []` collapses a failed load to
// zero bytes, and zero is always under budget. `assertRealPageLoad` is the
// pure, red-first-tested gate that a broken measurement can never pass
// silently — a zero-byte page is never a pass, it is a broken measurement.
describe("assertRealPageLoad", () => {
  it("throws when Lighthouse reports a runtimeError (e.g. the page never loaded at all)", () => {
    expect(() =>
      assertRealPageLoad(
        { runtimeError: { code: "NO_FCP", message: "The page did not paint any content" }, items: [] },
        "http://127.0.0.1:4399/indicador/pib",
      ),
    ).toThrow(LighthouseMeasurementError);
  });

  it("throws when the network record set is empty — an unreachable server or a broken measurement, NEVER a pass", () => {
    expect(() => assertRealPageLoad({ runtimeError: null, items: [] }, "http://127.0.0.1:4399/indicador/pib")).toThrow(
      LighthouseMeasurementError,
    );
  });

  it("throws when the main document response is not 2xx (e.g. a 404/500 page)", () => {
    expect(() =>
      assertRealPageLoad(
        {
          runtimeError: null,
          items: [{ resourceType: "Document", transferSize: 500, statusCode: 404 }],
        },
        "http://127.0.0.1:4321/indicador/does-not-exist",
      ),
    ).toThrow(LighthouseMeasurementError);
  });

  it("does NOT throw for a real page load — non-empty items, no runtimeError, a 200 document response (triangulation: the positive case)", () => {
    expect(() =>
      assertRealPageLoad(
        {
          runtimeError: null,
          items: [
            { resourceType: "Document", transferSize: 12_000, statusCode: 200 },
            { resourceType: "Script", transferSize: 8_000, statusCode: 200 },
          ],
        },
        "http://127.0.0.1:4321/indicador/pib",
      ),
    ).not.toThrow();
  });

  it("does NOT throw when no document item carries a statusCode at all (degrade gracefully rather than false-failing on an unfamiliar Lighthouse shape)", () => {
    expect(() =>
      assertRealPageLoad({ runtimeError: null, items: [{ resourceType: "Script", transferSize: 500 }] }, "http://127.0.0.1:4321/x"),
    ).not.toThrow();
  });
});

// verify-report WARNING-8 ("CI gate ordering measures the WORKBENCH build"):
// `.github/workflows/ci.yml` ran the production `astro build` at step 3 and
// the Playwright suite at step 5; under `CI`, Playwright has
// `reuseExistingServer: false` and re-runs `WORKBENCH=1 npm run build && npm
// run preview`, overwriting `dist/`. The preview server and the budget gate
// that followed therefore measured the WORKBENCH build, not the production
// build that ships. A budget gate that measures the wrong artifact is not a
// gate.
//
// The workflow ordering is fixed separately, but ordering is a discipline any
// future edit can silently undo, and the failure mode is a GREEN build that
// measured the wrong thing. This assertion turns that class of mistake into a
// loud, self-diagnosing failure: `/workbench` exists ONLY in a `WORKBENCH=1`
// build (astro.config.mjs injects the route behind that env var and the
// entrypoint lives outside `src/pages`, so it is never auto-routed), which
// makes its reachability an exact, zero-ambiguity fingerprint of which build
// is being served.
describe("assertProductionBuild", () => {
  it("throws when /workbench answers 200 — the served build is a WORKBENCH build, so the measurement is of the wrong artifact", () => {
    expect(() => assertProductionBuild(200, "http://127.0.0.1:4321")).toThrow(WorkbenchBuildMeasuredError);
  });

  it("names the ordering defect in its message so a CI failure is self-diagnosing, not a riddle", () => {
    expect(() => assertProductionBuild(200, "http://127.0.0.1:4321")).toThrow(/WORKBENCH/);
  });

  it("throws for any 2xx, not only 200 (a 204 on /workbench still means the route was injected)", () => {
    expect(() => assertProductionBuild(204, "http://127.0.0.1:4321")).toThrow(WorkbenchBuildMeasuredError);
  });

  it("does NOT throw on 404 — the production build has no /workbench route (triangulation: the passing case)", () => {
    expect(() => assertProductionBuild(404, "http://127.0.0.1:4321")).not.toThrow();
  });

  it("does NOT throw on a 3xx redirect, which is not the injected route answering", () => {
    expect(() => assertProductionBuild(301, "http://127.0.0.1:4321")).not.toThrow();
  });

  it("is a distinct error type from a broken measurement — the two failures have different causes and different fixes", () => {
    expect(new WorkbenchBuildMeasuredError("x")).not.toBeInstanceOf(LighthouseMeasurementError);
  });

  it("probes the exact path astro.config.mjs injects under WORKBENCH=1", () => {
    expect(WORKBENCH_PROBE_PATH).toBe("/workbench");
  });
});
