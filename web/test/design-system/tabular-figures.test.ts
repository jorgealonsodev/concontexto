// design-system spec, "Tabular figures on all numerals" — tasks.md 5.11
// (RED) / 5.12 (GREEN).
//
// Scope note: the spec's own scenario ("computed font-variant-numeric of
// each numeric element" on "a rendered indicator page") describes a
// browser-measured assertion over real numeral-bearing components, which
// do not exist until slice 6+ (IndicatorCard etc.) and real pages until
// slice 9. This slice proves the MECHANISM at the level that exists today:
// the compiled CSS for the utility numeric elements will use resolves
// `font-variant-numeric` to tabular figures, and the `--font-numeric`
// token survives the `--font-*` zeroing. The full rendered-page scenario
// is re-proven once slice 9's Playwright suite has real numeral-bearing
// pages to measure — disclosed here rather than silently narrowed.
import { describe, expect, it } from "vitest";
import { buildTheme } from "./compile-theme";

describe("tabular figures on numerals (PRD §12.1)", () => {
  it("the tabular-nums utility resolves font-variant-numeric to tabular figures", async () => {
    const css = await buildTheme(["tabular-nums"]);
    expect(css).toContain(".tabular-nums {");
    expect(css).toContain("--tw-numeric-spacing: tabular-nums");
    expect(css).toMatch(/font-variant-numeric:\s*var\(--tw-ordinal,?\)/);
  });

  it("--font-numeric survives the --font-* zeroing as a real project token", async () => {
    const css = await buildTheme(["font-numeric"]);
    expect(css).toContain(".font-numeric {");
    expect(css).toContain("font-family: var(--font-numeric)");
  });
});
