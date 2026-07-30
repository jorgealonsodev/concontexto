// design-system spec, "Light and dark token sets are independently
// authored" — tasks.md 5.4 (RED) / 5.5 (GREEN).
//
// Scope note: the requirement's own worked example (design.md D-6) only
// re-states COLOUR tokens under `[data-theme="dark"]` — typography scale,
// radii and shadows are theme-independent design tokens (a card's corner
// radius does not change between light and dark). This test is therefore
// scoped to the `--color-*` namespace, the only one that genuinely varies
// by theme, consistent with the spec's own illustrative code.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { readColorTokens } from "../../src/lib/design-system/theme-tokens";
import { THEME_CSS_PATH } from "./compile-theme";

const { light: lightTokens, dark: darkTokens } = readColorTokens(THEME_CSS_PATH);
const lightNames = Object.keys(lightTokens);
const darkNames = Object.keys(darkTokens);

describe("dark token parity (PRD §12.5): every light colour token has an explicit dark counterpart", () => {
  it("the light theme declares at least one project colour token", () => {
    expect(lightNames.length).toBeGreaterThan(0);
  });

  it.each(lightNames)("--color-%s has an explicit dark counterpart", (name) => {
    expect(darkTokens[name]).toBeDefined();
  });

  it("the dark set introduces no colour token absent from light", () => {
    for (const name of darkNames) {
      expect(lightTokens[name]).toBeDefined();
    }
  });

  it("no dark declaration is a var()/color-mix() reference to the light value (not derived)", () => {
    // `readColorTokens` only captures plain `#rrggbb` literals in the first
    // place (its regex requires a literal hex value), so reaching this far
    // with a populated `darkTokens` map is itself partial proof; this test
    // additionally scans the raw dark block text for a `var(--color-`/
    // `color-mix(` reference to rule out a mixed literal+derived block.
    const css = readFileSync(THEME_CSS_PATH, "utf-8");
    const darkBlockStart = css.search(/\n\[data-theme="dark"\]\s*\{/);
    expect(darkBlockStart).toBeGreaterThan(-1);
    const darkBlockText = css.slice(darkBlockStart, css.indexOf("}", darkBlockStart) + 1);
    expect(darkBlockText).not.toMatch(/var\(--color-/);
    expect(darkBlockText).not.toMatch(/color-mix\(/);
  });
});
