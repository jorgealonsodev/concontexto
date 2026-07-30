// verify-report CRITICAL-2 remediation: the blocking Lighthouse
// transferred-bytes gate (`web/scripts/check-lighthouse-budget.ts`) printed
// "PASS ... 0.0 KB transferred" and exited 0 when reproduced against a
// server that was not even listening:
//
//   $ PREVIEW_URL=http://127.0.0.1:4399 node scripts/check-lighthouse-budget.mjs
//   PASS /indicador/pib: 0.0 KB transferred (excluding the typeface), budget 300 KB
//   Lighthouse transferred-bytes budget gate PASSED for all 6 pages.   EXIT=0
//
// These tests run the REAL script (not a mock) and assert a NON-ZERO exit
// code, because the whole point of this remediation is that a mock could not
// have caught the original defect.
import { describe, expect, it } from "vitest";
import { spawn } from "node:child_process";
import { createServer } from "node:http";
import type { Server } from "node:http";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const WEB_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const SCRIPT_PATH = path.join(WEB_DIR, "scripts/check-lighthouse-budget.ts");

// Deliberately distinct from Playwright's own dev-server port (4321) and from
// any port a developer might have a real service on — nothing should ever be
// listening here.
const UNREACHABLE_PREVIEW_URL = "http://127.0.0.1:4399";

interface GateRun {
  status: number | null;
  stdout: string;
  stderr: string;
}

/** Runs the real gate script exactly as `npm run budget:lighthouse` does.
 *
 * Asynchronous on purpose, and NOT `spawnSync`: one of the tests below serves
 * the probe from an in-process HTTP server, and `spawnSync` blocks this
 * process's event loop, so that server could never answer — the gate would
 * "fail" on a request timeout and the test would prove nothing about the
 * behaviour under test. */
function runGate(previewUrl: string): Promise<GateRun> {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, ["--experimental-strip-types", SCRIPT_PATH], {
      cwd: WEB_DIR,
      env: { ...process.env, PREVIEW_URL: previewUrl },
    });
    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf-8").on("data", (chunk: string) => {
      stdout += chunk;
    });
    child.stderr.setEncoding("utf-8").on("data", (chunk: string) => {
      stderr += chunk;
    });
    child.on("error", reject);
    child.on("close", (status) => resolve({ status, stdout, stderr }));
  });
}

function listenOnEphemeralPort(server: Server): Promise<number> {
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      resolve(typeof address === "object" && address !== null ? address.port : 0);
    });
  });
}

describe("check-lighthouse-budget.ts — the blocking gate must fail loudly, never silently pass, on a broken measurement", () => {
  it(
    "exits non-zero when PREVIEW_URL points at a server that is not listening (the exact CRITICAL-2 reproduction)",
    async () => {
      const result = await runGate(UNREACHABLE_PREVIEW_URL);
      expect(
        result.status,
        `expected a non-zero exit; got ${result.status}.\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`,
      ).not.toBe(0);
      // The failure must be loud and named, not a silent/blank non-zero exit.
      const output = `${result.stdout}\n${result.stderr}`;
      expect(output).not.toMatch(/PASS .*0\.0 KB transferred/);
    },
    60_000,
  );
});

// verify-report WARNING-8: the gate measured the WORKBENCH build, not the
// production build that ships. The workflow ordering is fixed in
// `.github/workflows/ci.yml`, but ordering is discipline and discipline is
// what a future edit undoes silently — so the gate now refuses to measure a
// build that answers on `/workbench`, a route `astro.config.mjs` injects ONLY
// under `WORKBENCH=1`. This exercises that refusal end to end against a real
// (if minimal) HTTP server, which is the only way to prove the script actually
// performs the probe rather than merely importing the function that would.
describe("check-lighthouse-budget.ts — the gate must refuse to measure a WORKBENCH build", () => {
  it(
    "exits non-zero, naming WORKBENCH, when the served build answers 2xx on /workbench",
    async () => {
      const requestedPaths: string[] = [];
      const server = createServer((req, res) => {
        requestedPaths.push(req.url ?? "");
        res.writeHead(200, { "content-type": "text/html" });
        res.end("<!doctype html><title>workbench</title>");
      });
      const port = await listenOnEphemeralPort(server);
      try {
        const result = await runGate(`http://127.0.0.1:${port}`);
        const output = `${result.stdout}\n${result.stderr}`;
        expect(result.status, `expected a non-zero exit; got ${result.status}.\n${output}`).not.toBe(0);
        expect(output).toMatch(/WORKBENCH/);
        // The probe must come BEFORE any page is measured: a wrong-artifact
        // run should cost one HTTP request, not six Lighthouse audits, and no
        // page may report a budget verdict against the wrong build.
        expect(requestedPaths).toEqual(["/workbench"]);
        expect(output).not.toMatch(/PASS \/indicador/);
      } finally {
        server.close();
      }
    },
    60_000,
  );
});

// verify-report SUGGESTION-12: the gate script hand-duplicated
// `computeTransferredBytesExcludingFonts` and `assertRealPageLoad` from
// `src/lib/budget/transferredBytes.ts`, so the version that ran in CI was not
// the version the unit tests exercised — a real drift risk on the one gate
// whose failure mode is a silent green. The duplication is now removed (the
// script is TypeScript, run under Node's own type stripping, and imports the
// tested module directly). This guard fails if anyone copies it back.
describe("check-lighthouse-budget.ts — no hand-duplicated copy of the tested budget logic (SUGGESTION-12)", () => {
  const source = readFileSync(SCRIPT_PATH, "utf-8");

  it("imports the budget logic from the unit-tested module rather than restating it", () => {
    expect(source).toMatch(/from\s+"\.\.\/src\/lib\/budget\/transferredBytes\.ts"/);
  });

  it.each(["computeTransferredBytesExcludingFonts", "assertRealPageLoad", "assertProductionBuild", "evaluatePageBudget"])(
    "declares no local copy of %s",
    (name) => {
      const declaration = new RegExp(`(function|const|let|var|class)\\s+${name}\\b`);
      expect(
        declaration.test(source),
        `${name} is declared locally in the gate script; CI would then run an untested copy. Import it from src/lib/budget/transferredBytes.ts instead.`,
      ).toBe(false);
    },
  );

  it("declares no local copy of the budget constant", () => {
    expect(/TRANSFERRED_BYTES_BUDGET\s*=/.test(source), "the 300 KB budget is restated in the gate script").toBe(false);
  });

  it("leaves no orphan .mjs predecessor behind for CI to accidentally keep running", () => {
    expect(() => readFileSync(path.join(WEB_DIR, "scripts/check-lighthouse-budget.mjs"), "utf-8")).toThrow();
  });
});
