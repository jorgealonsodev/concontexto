// tasks.md 5.1: bootstraps `experimental_AstroContainer` as this project's
// component-rendering test harness. This is a real, meaningful smoke test
// (not a placeholder): it proves the container can render a real `.astro`
// page through the real Astro+Svelte+Tailwind pipeline configured in
// astro.config.mjs, which slice 6 onward relies on for every component
// test.
import { describe, expect, it } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import Home from "../../src/pages/index.astro";

describe("Astro container harness (task 5.1)", () => {
  it("renders the deploy smoke-target page with its real content", async () => {
    const container = await AstroContainer.create();
    const html = await container.renderToString(Home);

    expect(html).toContain("ConContexto");
    expect(html).toContain("Portal de Datos Económicos de España");
    expect(html).toContain('data-theme="light"');
  });
});
