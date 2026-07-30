// design-system spec, "AA contrast is measured in both themes, including
// tooltips" — tasks.md 5.6 (RED) / 5.7 (GREEN). Also covers the reusable
// pure-function contract (relativeLuminance/contrastRatio), which is
// unit-tested against known reference values independent of the theme file.
import { describe, expect, it } from "vitest";
import {
  checkContrastPairings,
  contrastRatio,
  DECLARED_PAIRINGS,
  relativeLuminance,
} from "../../src/lib/design-system/contrast";
import { readColorTokens } from "../../src/lib/design-system/theme-tokens";
import { THEME_CSS_PATH } from "./compile-theme";

describe("contrastRatio (pure function, WCAG 2.1 formula)", () => {
  it("black on white is the maximum ratio, 21:1", () => {
    expect(contrastRatio("#000000", "#ffffff")).toBeCloseTo(21, 1);
  });

  it("a colour against itself is the minimum ratio, 1:1", () => {
    expect(contrastRatio("#336699", "#336699")).toBeCloseTo(1, 5);
  });

  it("is symmetric regardless of argument order", () => {
    expect(contrastRatio("#123456", "#abcdef")).toBeCloseTo(contrastRatio("#abcdef", "#123456"), 10);
  });

  it("relativeLuminance of white is 1 and of black is 0", () => {
    expect(relativeLuminance("#ffffff")).toBeCloseTo(1, 5);
    expect(relativeLuminance("#000000")).toBeCloseTo(0, 5);
  });
});

describe("design system contrast harness (PRD §12.5): every declared pairing meets AA in both themes", () => {
  const tokens = readColorTokens(THEME_CSS_PATH);
  const results = checkContrastPairings(DECLARED_PAIRINGS, tokens);

  it("evaluated every declared pairing in both themes", () => {
    expect(results.length).toBe(DECLARED_PAIRINGS.length * 2);
  });

  it.each(results.map((r) => [`${r.pairing.name} (${r.theme})`, r] as const))(
    "%s meets its AA threshold",
    (_label, result) => {
      // Failure message names the pairing and the theme, per the spec
      // scenario "A token change that breaks contrast fails the build".
      expect(
        result.passes,
        `"${result.pairing.name}" in the ${result.theme} theme measured ${result.ratio.toFixed(2)}:1, ` +
          `below its required ${result.threshold}:1 (level: ${result.pairing.level})`,
      ).toBe(true);
    },
  );
});
