// web-accessibility-gates spec, "The transferred-bytes budget is a blocking
// gate": "Lighthouse CI MUST run against every indicator page as a blocking
// gate and MUST assert EXPLICITLY that total transferred bytes, including
// default-range data and excluding the typeface, are under 300 KB."
//
// This is the pure, red-first-testable half of that gate (task 9b.8): given
// Lighthouse's own `network-requests` audit items for a page load, compute
// the total transferred bytes EXCLUDING font resources. The browser-driving
// half (`scripts/check-lighthouse-budget.ts`) is written alongside, per
// this project's own stated Strict-TDD boundary for acceptance gates — a
// red-first browser budget run proves nothing (web-accessibility-gates
// spec, "Test discipline is stated honestly").
export const TRANSFERRED_BYTES_BUDGET = 300 * 1024;

export interface NetworkRequestItem {
  resourceType: string;
  transferSize?: number;
  /** Lighthouse's own `network-requests` audit item field. Present for a
   * real page load; absent only for older/unfamiliar Lighthouse report
   * shapes, in which case `assertRealPageLoad` degrades gracefully rather
   * than false-failing on a field it cannot verify. */
  statusCode?: number;
}

/** Sums `transferSize` across every request whose `resourceType` is not
 * `"Font"` (Lighthouse's own `network-requests` audit resourceType value
 * for typeface files) — the exact "excluding the typeface" carve-out the
 * spec names explicitly. */
export function computeTransferredBytesExcludingFonts(requests: NetworkRequestItem[]): number {
  return requests
    .filter((r) => r.resourceType !== "Font")
    .reduce((sum, r) => sum + (r.transferSize ?? 0), 0);
}

/** verify-report CRITICAL-2: the blocking Lighthouse transferred-bytes gate
 * printed "PASS ... 0.0 KB" and exited 0 against a server that was not even
 * listening — `details?.items ?? []` collapsed a failed load to zero bytes,
 * and zero bytes is under any budget. Thrown by `assertRealPageLoad` to make
 * that class of broken measurement fail LOUDLY instead of silently passing. */
export class LighthouseMeasurementError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "LighthouseMeasurementError";
  }
}

export interface LighthouseRuntimeError {
  code: string;
  message: string;
}

export interface LighthousePageAudit {
  /** Lighthouse's own `lhr.runtimeError` — set when Lighthouse itself could
   * not complete the audit (e.g. the page never painted). */
  runtimeError?: LighthouseRuntimeError | null;
  /** Lighthouse's own `network-requests` audit's `details.items`. */
  items: NetworkRequestItem[];
}

/** Validates that `audit` represents a REAL page load before it is ever
 * compared against the transferred-bytes budget (web-accessibility-gates
 * spec, "The transferred-bytes budget is a blocking gate"). Throws
 * `LighthouseMeasurementError` when:
 *  - Lighthouse itself reports a `runtimeError` (the page never loaded), OR
 *  - the network record set is EMPTY (an unreachable server, a non-200
 *    response Lighthouse could not follow, or any other broken measurement
 *    that produces no requests at all), OR
 *  - the main document request's own `statusCode` is not 2xx.
 * A zero-byte page is never a pass — it is a broken measurement. Does NOT
 * throw when no item carries a `statusCode` at all (an unfamiliar Lighthouse
 * report shape) — that check degrades gracefully rather than false-failing a
 * real page load Lighthouse itself did not flag as an error. */
export function assertRealPageLoad(audit: LighthousePageAudit, url: string): void {
  if (audit.runtimeError) {
    throw new LighthouseMeasurementError(
      `Lighthouse reported a runtime error loading ${url}: ${audit.runtimeError.code} — ${audit.runtimeError.message}. A broken measurement is never a pass.`,
    );
  }
  if (audit.items.length === 0) {
    throw new LighthouseMeasurementError(
      `Lighthouse recorded ZERO network requests for ${url} — the page did not load (an unreachable server, a non-200 response, or another broken measurement). A zero-byte page is never a pass.`,
    );
  }
  const documentItem = audit.items.find((item) => item.resourceType === "Document") ?? audit.items[0];
  if (documentItem.statusCode !== undefined && (documentItem.statusCode < 200 || documentItem.statusCode >= 300)) {
    throw new LighthouseMeasurementError(
      `Lighthouse reported a non-2xx response loading ${url}: statusCode=${documentItem.statusCode}.`,
    );
  }
}

/** The route `astro.config.mjs`'s `workbenchRoutes()` integration injects
 * when — and only when — `WORKBENCH=1`. The entrypoint lives outside
 * `src/pages` (`src/workbench/pages/index.astro`) precisely so it is never
 * auto-routed, which makes the reachability of this one path an exact,
 * zero-ambiguity fingerprint of WHICH build a server is serving. */
export const WORKBENCH_PROBE_PATH = "/workbench";

/** verify-report WARNING-8: the transferred-bytes gate measured the WORKBENCH
 * build rather than the production build that ships. `.github/workflows/ci.yml`
 * ran the production `astro build` at step 3 and the Playwright suite at step
 * 5; under `CI`, `playwright.config.ts` sets `reuseExistingServer: false` and
 * re-runs `WORKBENCH=1 npm run build && npm run preview`, overwriting `dist/`.
 * Everything downstream — the preview server, the budget gate — then measured
 * an artifact that never reaches a reader.
 *
 * A distinct type from `LighthouseMeasurementError` on purpose: that one means
 * "the measurement is broken"; this one means "the measurement is fine but the
 * SUBJECT is wrong". Different causes, different fixes, and a CI log should not
 * have to guess which happened. */
export class WorkbenchBuildMeasuredError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "WorkbenchBuildMeasuredError";
  }
}

/** Asserts that the server under measurement is serving the PRODUCTION build,
 * given the HTTP status `WORKBENCH_PROBE_PATH` answered with.
 *
 * Correct workflow ordering is what actually fixes WARNING-8 — but ordering is
 * discipline, and discipline is exactly what a future edit undoes silently. The
 * failure mode there is not a red build; it is a GREEN build that measured the
 * wrong artifact. This turns that into a loud, self-diagnosing failure.
 *
 * Any 2xx means the injected route answered, so the served `dist/` came from a
 * `WORKBENCH=1` build. A 404 (production) passes; so does a 3xx, which is a
 * redirect rule rather than the injected route answering. */
export function assertProductionBuild(workbenchStatusCode: number, baseUrl: string): void {
  if (workbenchStatusCode >= 200 && workbenchStatusCode < 300) {
    throw new WorkbenchBuildMeasuredError(
      `${baseUrl}${WORKBENCH_PROBE_PATH} answered ${workbenchStatusCode}, so the server under measurement is serving a WORKBENCH build, not the production build that ships. ` +
        `Measure the artifact readers actually receive: run the production \`astro build\` (no WORKBENCH variable) AFTER the Playwright suite, which rebuilds \`dist/\` with \`WORKBENCH=1\`, and only then start the preview server. ` +
        `A budget gate that measures the wrong artifact is not a gate (verify-report WARNING-8).`,
    );
  }
}

export interface PageBudgetResult {
  path: string;
  transferredBytes: number;
  withinBudget: boolean;
}

/** Named assertion (web-accessibility-gates spec, "the budget assertion is
 * explicit, not implied by a performance score"): evaluates one page's
 * measured transferred bytes against `TRANSFERRED_BYTES_BUDGET`. */
export function evaluatePageBudget(path: string, requests: NetworkRequestItem[]): PageBudgetResult {
  const transferredBytes = computeTransferredBytesExcludingFonts(requests);
  return { path, transferredBytes, withinBudget: transferredBytes < TRANSFERRED_BYTES_BUDGET };
}
