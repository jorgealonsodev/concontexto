import { defineConfig } from "astro/config";
import svelte from "@astrojs/svelte";
import tailwindcss from "@tailwindcss/vite";

// indicator-page spec, "A slug change ships a permanent redirect" (task
// 9b.2): Astro's native `redirects` config option, fed from this project's
// own `SLUG_REDIRECTS` map (empty in production — see that module's doc
// comment; `redirects.test.ts` proves the mechanism via a fixture map).
import { buildAstroRedirects } from "./src/lib/indicator/redirects.ts";

// Fase 1 design system (ADR-6, ADR-8): Tailwind is build-time only via the
// Vite plugin — no PostCSS config, no Node in production (PRD §14.2). The
// Tailwind `theme` block (web/src/styles/theme.css) IS the project's
// design-token file, authored from scratch; see docs/adr/0006, docs/adr/0008.
// Svelte provides the one hydrated island (the interactive chart, Fase 1
// slice 8); every other component ships zero runtime JavaScript.

// Component workbench (design.md "Supporting decisions": "Workbench — routes
// injected only when WORKBENCH=1 (CI test/preview builds), rejected shipping
// unlinked prod routes — keeps prod surface = product pages; budget gates
// measure real pages only"). The entrypoint lives outside `src/pages`
// (`src/workbench/pages/`) so it is NEVER auto-routed regardless of this env
// var; injection is the only way `/workbench` becomes reachable.
function workbenchRoutes() {
  return {
    name: "concontexto-workbench-routes",
    hooks: {
      "astro:config:setup": ({ injectRoute }) => {
        if (process.env.WORKBENCH === "1") {
          injectRoute({
            pattern: "/workbench",
            entrypoint: "./src/workbench/pages/index.astro",
          });
        }
      },
    },
  };
}

export default defineConfig({
  outDir: "./dist",
  redirects: buildAstroRedirects(),
  integrations: [svelte(), workbenchRoutes()],
  vite: {
    plugins: [tailwindcss()],
  },
});
