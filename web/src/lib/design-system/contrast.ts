// The design system's contrast harness (design-system spec, "AA contrast is
// measured in both themes, including tooltips"; PRD §12.5). Pure functions
// only — no DOM — so both the Vitest suite and (later) any build-time gate
// can share the exact same implementation. Values are MEASURED against
// `web/src/styles/theme.css`'s actual declared hex tokens, never assumed.

/** WCAG 2.1 relative luminance of an sRGB channel value (0-255). */
function srgbChannelToLinear(channel: number): number {
  const c = channel / 255;
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

/** WCAG 2.1 relative luminance of a `#rrggbb` hex colour. */
export function relativeLuminance(hex: string): number {
  const normalized = hex.replace("#", "");
  if (!/^[0-9a-fA-F]{6}$/.test(normalized)) {
    throw new Error(`relativeLuminance: expected a 6-digit hex colour, got "${hex}"`);
  }
  const r = parseInt(normalized.slice(0, 2), 16);
  const g = parseInt(normalized.slice(2, 4), 16);
  const b = parseInt(normalized.slice(4, 6), 16);
  return 0.2126 * srgbChannelToLinear(r) + 0.7152 * srgbChannelToLinear(g) + 0.0722 * srgbChannelToLinear(b);
}

/** WCAG 2.1 contrast ratio between two `#rrggbb` hex colours, in [1, 21]. */
export function contrastRatio(hexA: string, hexB: string): number {
  const lA = relativeLuminance(hexA);
  const lB = relativeLuminance(hexB);
  const lighter = Math.max(lA, lB);
  const darker = Math.min(lA, lB);
  return (lighter + 0.05) / (darker + 0.05);
}

export const CONTRAST_LEVEL = {
  BODY_TEXT: "body-text",
  LARGE_TEXT: "large-text",
  GRAPHIC: "graphic",
} as const;

export type ContrastLevel = (typeof CONTRAST_LEVEL)[keyof typeof CONTRAST_LEVEL];

/** WCAG 2.1 AA minimum ratio for each usage level (PRD §12.5 / design-system spec). */
export const CONTRAST_THRESHOLD: Record<ContrastLevel, number> = {
  [CONTRAST_LEVEL.BODY_TEXT]: 4.5,
  [CONTRAST_LEVEL.LARGE_TEXT]: 3,
  [CONTRAST_LEVEL.GRAPHIC]: 3,
};

export interface ContrastPairing {
  /** Human-readable pairing name, used verbatim in failure messages. */
  name: string;
  /** Token name (without the `--color-` prefix) used as the foreground. */
  fg: string;
  /** Token name (without the `--color-` prefix) used as the background. */
  bg: string;
  level: ContrastLevel;
}

export interface ContrastCheckResult {
  pairing: ContrastPairing;
  theme: "light" | "dark";
  ratio: number;
  threshold: number;
  passes: boolean;
}

/**
 * Evaluates every declared pairing against both theme token sets.
 * `tokens.light`/`tokens.dark` map bare token names ("ink", "bg", ...) to
 * their `#rrggbb` hex value, exactly the shape `readColorTokens` in
 * `theme-tokens.ts` produces.
 */
export function checkContrastPairings(
  pairings: readonly ContrastPairing[],
  tokens: { light: Record<string, string>; dark: Record<string, string> },
): ContrastCheckResult[] {
  const results: ContrastCheckResult[] = [];
  for (const theme of ["light", "dark"] as const) {
    const set = tokens[theme];
    for (const pairing of pairings) {
      const fgHex = set[pairing.fg];
      const bgHex = set[pairing.bg];
      if (!fgHex || !bgHex) {
        throw new Error(
          `checkContrastPairings: pairing "${pairing.name}" references an unknown token (fg="${pairing.fg}", bg="${pairing.bg}") in the ${theme} theme`,
        );
      }
      const ratio = contrastRatio(fgHex, bgHex);
      const threshold = CONTRAST_THRESHOLD[pairing.level];
      results.push({ pairing, theme, ratio, threshold, passes: ratio >= threshold });
    }
  }
  return results;
}

/**
 * The enumerated set of foreground/background pairings the current
 * component library declares (design-system spec, "AA contrast is measured
 * in both themes"). Extend this list as slices 6-8 add real components —
 * the harness re-evaluates every entry against both themes automatically.
 */
export const DECLARED_PAIRINGS: readonly ContrastPairing[] = [
  { name: "body text on page background", fg: "ink", bg: "bg", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "body text on surface", fg: "ink", bg: "surface", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "muted text on page background", fg: "ink-muted", bg: "bg", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "muted text on surface", fg: "ink-muted", bg: "surface", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "accent text/link on surface", fg: "accent", bg: "surface", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "accent as a chart stroke on page background", fg: "accent", bg: "bg", level: CONTRAST_LEVEL.GRAPHIC },
  { name: "fresh (green) text on surface", fg: "fresh", bg: "surface", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "fresh (green) text on page background", fg: "fresh", bg: "bg", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "fresh (green) as an icon stroke on page background", fg: "fresh", bg: "bg", level: CONTRAST_LEVEL.GRAPHIC },
  { name: "pending (amber) text on surface", fg: "pending", bg: "surface", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "pending (amber) text on page background", fg: "pending", bg: "bg", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "pending (amber) as a chart/icon stroke on page background", fg: "pending", bg: "bg", level: CONTRAST_LEVEL.GRAPHIC },
  { name: "provisional (grey) text on surface", fg: "provisional", bg: "surface", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "provisional (grey) text on page background", fg: "provisional", bg: "bg", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "provisional (grey) as a dotted chart stroke on page background", fg: "provisional", bg: "bg", level: CONTRAST_LEVEL.GRAPHIC },
  { name: "tooltip text on tooltip background", fg: "tooltip-ink", bg: "tooltip-bg", level: CONTRAST_LEVEL.BODY_TEXT },
  { name: "break-band fill on page background", fg: "break-band", bg: "bg", level: CONTRAST_LEVEL.GRAPHIC },
  // The change-of-government marker. No new token: it is drawn in `--color-ink`
  // — the annotation-layer colour, deliberately NOT the axis grey, which is
  // near-identical to the reserved provisional grey and would have collided
  // with that semantic in everything but name. `ink` is already measured as
  // TEXT above; these two entries measure the usage this chart really makes of
  // it, as a non-text graphic (WCAG 1.4.11), in both themes and against both
  // surfaces a chart can sit on.
  { name: "government-change marker rule on page background", fg: "ink", bg: "bg", level: CONTRAST_LEVEL.GRAPHIC },
  { name: "government-change marker rule on surface", fg: "ink", bg: "surface", level: CONTRAST_LEVEL.GRAPHIC },
  // The editorial event span's rail — the chart's FOURTH annotation treatment.
  // It gets a token of its own rather than borrowing one, because every
  // existing colour is already spoken for: purple is the break band, blue the
  // accent, ink the government rule, and amber/grey are RESERVED semantics.
  // Rose is the one hue left that no other mark on this chart uses.
  //
  // Colour is the SECOND channel here and never the first — the rail is
  // horizontal where every other vertical treatment is vertical, and a stroke
  // where the band is a fill — but a non-text graphic still has to meet WCAG
  // 1.4.11, in both themes and against both surfaces a chart can sit on.
  { name: "event-span rail on page background", fg: "event-span", bg: "bg", level: CONTRAST_LEVEL.GRAPHIC },
  { name: "event-span rail on surface", fg: "event-span", bg: "surface", level: CONTRAST_LEVEL.GRAPHIC },
];
