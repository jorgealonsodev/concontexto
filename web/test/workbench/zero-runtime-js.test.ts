// design-system spec, "Seven components ship no runtime JavaScript" — tasks.md
// 6.3 (RED) / 6.4 (GREEN). A page composing all seven static components MUST
// ship no client-side script.
//
// Slice 8 adds `ChartIsland.svelte` (PRD §14.2's ONE sanctioned hydrated
// island, `client:idle`) to the SAME shared `WorkbenchShowcase` this test
// renders — so the container now needs the Svelte renderer registered
// (`@astrojs/svelte/container-renderer`, the container-mode equivalent of
// `astro.config.mjs`'s `svelte()` integration) purely to render the page at
// all. This does NOT relax "seven components ship no runtime JavaScript":
// the assertion below strips the chart-island section's own subtree before
// checking for a `<script>` tag, so the seven static components (plus the
// static `IndicatorChart`, the eighth catalog entry's no-JS half) are still
// held to exactly zero script tags; only the deliberately-hydrated island
// section is excluded, matching PRD §14.2's own "exactly one island" carve-out.
import { describe, expect, it } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import { loadRenderers } from "astro:container";
import { getContainerRenderer as getSvelteContainerRenderer } from "@astrojs/svelte/container-renderer";
import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import WorkbenchShowcase from "../../src/workbench/WorkbenchShowcase.astro";

/** Removes the chart-island section's own subtree (identified by its
 * `data-testid`) before the zero-script assertion runs — a coarse but
 * sufficient balanced-tag strip since exactly one such section exists per
 * theme and it is always the last `<section>` in `WorkbenchShowcase.astro`. */
function stripChartIslandSection(html: string, testid: string): string {
  const start = html.indexOf(`data-testid="${testid}"`);
  if (start === -1) throw new Error(`expected to find a "${testid}" section to strip`);
  const sectionStart = html.lastIndexOf("<section", start);
  return html.slice(0, sectionStart) + html.slice(html.indexOf("</section>", start) + "</section>".length);
}

function walk(dir: string, exts: string[], out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) walk(full, exts, out);
    else if (exts.some((ext) => full.endsWith(ext))) out.push(full);
  }
  return out;
}

describe("zero-runtime-JS (design-system spec: 'Seven components ship no runtime JavaScript')", () => {
  it("the rendered workbench markup (both themes), excluding the ChartIsland section, contains no <script> tag", async () => {
    const renderers = await loadRenderers([getSvelteContainerRenderer()]);
    const container = await AstroContainer.create({ renderers });
    const light = await container.renderToString(WorkbenchShowcase, { props: { theme: "light" } });
    const dark = await container.renderToString(WorkbenchShowcase, { props: { theme: "dark" } });
    const lightWithoutIsland = stripChartIslandSection(light, "chart-island-section-light");
    const darkWithoutIsland = stripChartIslandSection(dark, "chart-island-section-dark");
    expect(lightWithoutIsland.toLowerCase()).not.toContain("<script");
    expect(darkWithoutIsland.toLowerCase()).not.toContain("<script");
  });

  it("the ChartIsland section is the ONLY source of a <script> tag on the workbench page", async () => {
    const renderers = await loadRenderers([getSvelteContainerRenderer()]);
    const container = await AstroContainer.create({ renderers });
    const light = await container.renderToString(WorkbenchShowcase, { props: { theme: "light" } });
    // Sanity check for the test above itself: without stripping, a script
    // tag IS present (proving the strip is load-bearing, not vacuous).
    expect(light.toLowerCase()).toContain("<script");
  });

  it("none of the seven component source files declares a client:* hydration directive", () => {
    // A structural, mechanism-level guarantee beneath the rendered-output
    // assertion above: no `.astro` file under src/components uses a
    // `client:` directive at all, so the zero-JS property cannot regress by
    // one of the seven starting to hydrate.
    const componentsDir = path.resolve(
      path.dirname(fileURLToPath(import.meta.url)),
      "../../src/components",
    );
    const files = walk(componentsDir, [".astro"]);
    expect(files.length).toBeGreaterThanOrEqual(7);
    for (const file of files) {
      const content = readFileSync(file, "utf-8");
      expect(content, `${file} declares a client:* directive`).not.toMatch(/client:(load|idle|visible|media|only)/);
    }
  });
});
