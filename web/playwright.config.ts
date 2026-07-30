import { defineConfig, devices } from "@playwright/test";

// Frontend e2e/accessibility test stack (web-accessibility-gates spec,
// "Web test commands are declared and blocking"; ADR-6/ADR-8's harness).
// Slice 5 establishes this command and its first real test against the
// existing deploy smoke-target page; the full suite (axe on all six
// indicator pages, keyboard traversal, 44px targets, no-JS context,
// Lighthouse budget) is slice 9's own deliverable — see tasks.md 9a.13,
// 9b.4-9b.9.
export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: "list",
  timeout: 30_000,
  use: {
    baseURL: "http://localhost:4321",
    trace: "on-first-retry",
  },
  webServer: {
    // `build` first: `preview` serves the static `dist/` output, and the
    // suite must exercise the real production build (no-JS/budget gates in
    // slice 9 depend on this being the actual built artifact, not `astro
    // dev`'s dev server). `WORKBENCH=1` injects `/workbench` (astro.config.mjs)
    // — this Playwright run IS the "CI test/preview build" design.md's
    // Workbench decision names as the sanctioned case; the real production
    // build (the Go binary's own deploy pipeline) never sets this variable, so
    // `/workbench` never reaches production.
    //
    // `BUILD_WITH_SYNTHETIC_FIXTURE=1` (verify-report CRITICAL-15): the loader
    // no longer defaults to `test/fixtures/export`, because that default was
    // also reached by the Dockerfile's production build, which therefore
    // shipped the fixture's SYNTHESISED history as INE statistics. The fixture
    // is now opt-in, and this suite is one of the two legitimate opters-in (the
    // other is a developer's local build): it needs a long, six-series artifact
    // to traverse, and it publishes nothing. Stated HERE, in the command, for
    // the same reason `WORKBENCH=1` is: the sanction is visible at the point of
    // use rather than inherited from an ambient environment.
    command: "WORKBENCH=1 BUILD_WITH_SYNTHETIC_FIXTURE=1 npm run build && npm run preview",
    url: "http://localhost:4321",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
