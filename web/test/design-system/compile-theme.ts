// Shared helper for the design-token guard tests. Compiles
// `web/src/styles/theme.css` for an explicit candidate-class list using
// Tailwind's own low-level compiler (`@tailwindcss/node`'s `compile`, the
// same primitive `@tailwindcss/vite` uses internally), so these tests
// exercise the REAL Tailwind engine against the REAL theme file rather than
// a hand-rolled CSS parser standing in for it.
import { compile } from "@tailwindcss/node";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const THEME_CSS_PATH = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../src/styles/theme.css",
);

export async function buildTheme(candidates: string[]): Promise<string> {
  const css = readFileSync(THEME_CSS_PATH, "utf-8");
  const { build } = await compile(css, {
    base: path.dirname(THEME_CSS_PATH),
    onDependency: () => {},
  });
  return build(candidates);
}

/**
 * True iff the compiled CSS declares a rule whose selector is exactly
 * `.{className}` (Tailwind emits `.foo {` — this checks for that literal
 * selector text inside the `@layer utilities` output rather than a full CSS
 * parse, which is sufficient because Tailwind never emits a bare rule with
 * this selector text for any other reason).
 */
export function utilitySelectorEmitted(css: string, className: string): boolean {
  return css.includes(`.${className} {`);
}
