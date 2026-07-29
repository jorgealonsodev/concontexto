import { defineConfig } from "astro/config";

// Minimal hello-world config for the 0.1 deploy scenario (PR 1b).
// No integrations, no component framework — that arrives in Fase 1
// alongside the Svelte islands and the real design system.
export default defineConfig({
  outDir: "./dist",
});
