// design-system spec, "The Tailwind theme is the design-token file and
// stock defaults are zeroed" — tasks.md 5.2 (RED) / 5.3 (GREEN).
//
// Tailwind's DEFAULT design values are forbidden (ADR-6). This test proves
// it mechanically: a stock utility class must compile to literally no CSS
// rule, so the mistake is loud at build time rather than caught in review.
import { describe, expect, it } from "vitest";
import { buildTheme, utilitySelectorEmitted } from "./compile-theme";

describe("design-token guard (ADR-6): stock Tailwind defaults are zeroed", () => {
  const stockColourClasses = ["bg-blue-500", "text-red-500", "border-green-600"];
  const stockTypeScaleClasses = ["text-sm", "text-lg", "text-2xl"];
  // `rounded-full`/`rounded-none`/`shadow-none` are a disclosed, verified
  // exception: Tailwind v4 implements them as STATIC utilities with a
  // hard-coded value (`calc(infinity * 1px)`, `0`, `0 0 #0000`), never
  // reading the `--radius-*`/`--shadow-*` theme namespace at all — no
  // amount of `--radius-*: initial`/`--shadow-*: initial` can zero them,
  // confirmed empirically against the compiled output. This is a Tailwind
  // engine constraint, not a design-system authoring gap: these are
  // degenerate boundary values (no rounding / fully round / no shadow),
  // not the curvature/elevation AMOUNTS ADR-6's homogeneity concern is
  // about. Every scale-driven radius/shadow class below IS zeroed.
  const stockRadiusClasses = ["rounded-sm", "rounded-lg", "rounded-xl", "rounded-2xl", "rounded-3xl"];
  const stockShadowClasses = ["shadow-sm", "shadow-md", "shadow-lg", "shadow-xl", "shadow-2xl", "shadow-inner"];
  // design.md D5 resolution: `--font-*` is zeroed too, a fifth scale beyond
  // ADR-6's literal floor of four (permitted, per the reconciliation table).
  const stockFontClasses = ["font-serif", "font-mono"];

  it.each(stockColourClasses)(
    "stock palette class %s emits no rule (scenario: 'A stock palette class emits no rule')",
    async (className) => {
      const css = await buildTheme([className]);
      expect(utilitySelectorEmitted(css, className)).toBe(false);
    },
  );

  it.each(stockTypeScaleClasses)("stock type-scale class %s emits no rule", async (className) => {
    const css = await buildTheme([className]);
    expect(utilitySelectorEmitted(css, className)).toBe(false);
  });

  it.each(stockRadiusClasses)("stock radius class %s emits no rule", async (className) => {
    const css = await buildTheme([className]);
    expect(utilitySelectorEmitted(css, className)).toBe(false);
  });

  it.each(stockShadowClasses)("stock shadow class %s emits no rule", async (className) => {
    const css = await buildTheme([className]);
    expect(utilitySelectorEmitted(css, className)).toBe(false);
  });

  it.each(stockFontClasses)("stock font-family class %s emits no rule", async (className) => {
    const css = await buildTheme([className]);
    expect(utilitySelectorEmitted(css, className)).toBe(false);
  });

  it("a project token class DOES emit a rule (scenario: 'A project token class emits a rule')", async () => {
    const css = await buildTheme(["bg-accent"]);
    expect(utilitySelectorEmitted(css, "bg-accent")).toBe(true);
    expect(css).toContain("background-color: var(--color-accent)");
  });

  it("project type-scale, radius, shadow and font classes DO emit rules", async () => {
    const css = await buildTheme(["text-heading-lg", "rounded-card", "shadow-card", "font-numeric"]);
    expect(utilitySelectorEmitted(css, "text-heading-lg")).toBe(true);
    expect(utilitySelectorEmitted(css, "rounded-card")).toBe(true);
    expect(utilitySelectorEmitted(css, "shadow-card")).toBe(true);
    expect(utilitySelectorEmitted(css, "font-numeric")).toBe(true);
  });
});
