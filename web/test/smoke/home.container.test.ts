// tasks.md 5.1: bootstraps `experimental_AstroContainer` as this project's
// component-rendering test harness. This is a real, meaningful smoke test
// (not a placeholder): it proves the container can render a real `.astro`
// page through the real Astro+Svelte+Tailwind pipeline configured in
// astro.config.mjs, which slice 6 onward relies on for every component
// test.
//
// The env stub is new in milestone 1.2. `/` used to be a static placeholder
// that read nothing; it now loads the export artifact at build time like
// every other page, and `resolveLoadOptionsFromEnv` deliberately has no
// default artifact source (verify-report CRITICAL-15), so this harness has
// to name the synthetic fixture in the same words a developer would. What
// this test asserts is unchanged — it is still about the harness, not about
// the homepage's content, which `test/pages/home.container.test.ts` owns.
import { afterEach, describe, expect, it, vi } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import Home from "../../src/pages/index.astro";

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("Astro container harness (task 5.1)", () => {
  it("renders the home page with its real content", async () => {
    vi.stubEnv("BUILD_WITH_SYNTHETIC_FIXTURE", "1");
    const container = await AstroContainer.create();
    const html = await container.renderToString(Home);

    expect(html).toContain("ConContexto");
    expect(html).toContain("Portal de Datos Económicos de España");
    expect(html).toContain('data-theme="light"');
  });
});
