#!/usr/bin/env node
// web-accessibility-gates spec, "The transferred-bytes budget is a blocking
// gate": "Lighthouse CI MUST run against every indicator page as a blocking
// gate and MUST assert EXPLICITLY that total transferred bytes ... are
// under 300 KB" — task 9b.8.
//
// Backfilled in slice 9b. Disclosed timeline gap: the spec's own wording
// requires this gate "wired from the first web slice, not added at the
// end" (web-accessibility-gates spec, "the transferred-bytes budget"
// requirement). No Lighthouse gate of any kind existed in `.github/workflows/`
// before that slice — slices 5-8 shipped no user-facing PAGE this gate
// could have measured (design-system components + the static/island chart
// only became a real, navigable page in slice 9a) — but the spec's literal
// instruction was to wire the MECHANISM from slice 5 regardless, ready for
// the day a real page existed, and that did not happen. Recorded honestly
// in design.md's Open Questions and apply-progress.md rather than silently
// claimed as always-on.
//
// TypeScript, not `.mjs`, and deliberately so — verify-report SUGGESTION-12.
// This script used to hand-duplicate `computeTransferredBytesExcludingFonts`
// and `assertRealPageLoad` from `src/lib/budget/transferredBytes.ts`, on the
// stated grounds that plain `node` cannot import a TypeScript module. That
// stopped being true: Node strips types from `.ts` files natively (unflagged
// since 22.18; `--experimental-strip-types` is kept on the npm script so the
// gate also runs on 22.6-22.17, where it is required, and is an accepted
// no-op above that). So the version that runs in CI is now literally the
// version `test/budget/transferredBytes.test.ts` exercises, rather than a
// copy that could drift from it — on the one gate whose failure mode is a
// silent green. No build step, no `tsx`, no new dependency: the runtime the
// project already pins does it. Type stripping erases, it does not compile,
// so every import below must resolve at runtime and carry its `.ts`
// extension; type-only imports use `import type` so they erase cleanly.
//
// Reuses Playwright's own Chromium binary (already installed for the e2e
// job) via its remote-debugging port, rather than adding a second, separate
// headless-Chrome install for Lighthouse.
import { chromium } from "@playwright/test";
import lighthouse from "lighthouse";
import {
  TRANSFERRED_BYTES_BUDGET,
  WORKBENCH_PROBE_PATH,
  assertProductionBuild,
  assertRealPageLoad,
  evaluatePageBudget,
} from "../src/lib/budget/transferredBytes.ts";
import type { NetworkRequestItem, PageBudgetResult } from "../src/lib/budget/transferredBytes.ts";

const REMOTE_DEBUGGING_PORT = 9222;
const BASE_URL = process.env.PREVIEW_URL ?? "http://127.0.0.1:4321";
const SLUGS = ["tasa-de-paro-epa", "ocupados-epa", "ipc-general", "ipc-subyacente", "pib", "poblacion-residente"];

/** verify-report WARNING-8: confirms the server under measurement is serving
 * the PRODUCTION build before a single page is audited.
 *
 * Runs FIRST, before Chromium is even launched: a wrong-artifact run should
 * cost one HTTP request rather than six Lighthouse audits, and no page should
 * ever print a budget verdict measured against the wrong build. It doubles as
 * the cheapest possible liveness check — an unreachable preview server fails
 * here, in milliseconds, with a plain connection error, instead of six
 * Lighthouse runs later (verify-report CRITICAL-2). */
async function assertMeasuringProductionBuild(): Promise<void> {
  const probeUrl = `${BASE_URL}${WORKBENCH_PROBE_PATH}`;
  let response: Response;
  try {
    response = await fetch(probeUrl, { redirect: "manual" });
  } catch (cause) {
    throw new Error(
      `Could not reach ${probeUrl} to confirm which build is being served. The preview server is not up, so there is nothing to measure — and a gate that measures nothing is never a pass.`,
      { cause },
    );
  }
  assertProductionBuild(response.status, BASE_URL);
}

async function main(): Promise<void> {
  await assertMeasuringProductionBuild();

  const browser = await chromium.launch({
    args: [`--remote-debugging-port=${REMOTE_DEBUGGING_PORT}`, "--no-sandbox"],
  });

  const results: PageBudgetResult[] = [];
  try {
    for (const slug of SLUGS) {
      const url = `${BASE_URL}/indicador/${slug}`;
      // eslint-disable-next-line no-await-in-loop
      const report = await lighthouse(url, {
        port: REMOTE_DEBUGGING_PORT,
        onlyAudits: ["network-requests"],
        output: "json",
        logLevel: "error",
      });
      if (!report) {
        throw new Error(`Lighthouse returned no report at all for ${url}. A missing measurement is never a pass.`);
      }
      // Lighthouse types `audit.details` as a union across every details shape
      // it can emit (opportunity, table, criticalrequestchain, ...), and only
      // some carry `items` — so `details?.items` does not type-check (ts2339).
      // Narrowed rather than suppressed: `network-requests` emits a table, and
      // if a future Lighthouse ever stopped doing so, `items` would be absent,
      // `assertRealPageLoad` would see an empty record set and the gate would
      // FAIL loudly. The fallback is deliberately the empty array precisely
      // because empty is the case that assertion refuses to pass.
      const details = report.lhr.audits["network-requests"].details;
      const items: NetworkRequestItem[] =
        details && "items" in details ? (details.items as unknown as NetworkRequestItem[]) : [];
      // Fails loudly (throws, caught by main()'s own caller below, non-zero
      // exit) rather than letting an empty/broken measurement fall through
      // to "0.0 KB, PASS" — the exact CRITICAL-2 defect.
      assertRealPageLoad({ runtimeError: report.lhr.runtimeError, items }, url);
      const result = evaluatePageBudget(`/indicador/${slug}`, items);
      results.push(result);
      const kb = (result.transferredBytes / 1024).toFixed(1);
      const budgetKb = (TRANSFERRED_BYTES_BUDGET / 1024).toFixed(0);
      // Named assertion: prints the measured total and the budget explicitly
      // (spec: "reporting the measured total and the budget").
      console.log(
        `${result.withinBudget ? "PASS" : "FAIL"} ${result.path}: ${kb} KB transferred (excluding the typeface), budget ${budgetKb} KB`,
      );
    }
  } finally {
    await browser.close();
  }

  const failures = results.filter((r) => !r.withinBudget);
  if (failures.length > 0) {
    console.error(`\nLighthouse transferred-bytes budget gate FAILED for: ${failures.map((f) => f.path).join(", ")}`);
    process.exitCode = 1;
  } else {
    console.log(`\nLighthouse transferred-bytes budget gate PASSED for all ${results.length} pages.`);
  }
}

main().catch((err: unknown) => {
  console.error("check-lighthouse-budget failed to run:", err);
  process.exitCode = 1;
});
