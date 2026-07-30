/// <reference types="vitest/config" />
// The triple-slash reference is what makes `test` a known property of Vite's
// `UserConfig` (verify-report SUGGESTION-14: `astro check` reported ts2353
// here — "'test' does not exist in type 'UserConfig'" — because nothing had
// ever type-checked this file). Vitest augments Vite's own config type through
// this declaration file; `getViteConfig` takes a Vite config, so without the
// augmentation loaded the `test` block below is, to the compiler, a typo.
import { getViteConfig } from "astro/config";

// design.md D-6 / D-5: Vitest shares Astro's own Vite config (Svelte
// integration, the Tailwind Vite plugin) so `experimental_AstroContainer`
// can render real `.astro` files and component tests see the same
// transforms production builds do. See tasks.md 5.1.
export default getViteConfig({
  test: {
    environment: "node",
    include: ["test/**/*.test.ts"],
  },
});
