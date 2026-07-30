// Explicit Svelte config so `vite-plugin-svelte` (via @astrojs/svelte) does
// not warn "no Svelte config found" on every dev/build/test run. No
// preprocessing is needed yet — Tailwind classes in Svelte markup are
// handled by the Vite plugin, not a Svelte preprocessor.
export default {};
