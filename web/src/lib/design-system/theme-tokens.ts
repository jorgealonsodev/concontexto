// Parses `web/src/styles/theme.css` into plain `{ tokenName: "#rrggbb" }`
// maps for light and dark, so consumers (the contrast harness, its tests)
// read the SAME declared values the build uses, rather than a second,
// hand-duplicated copy that could silently drift from the real theme file.
import { readFileSync } from "node:fs";

export interface ColorTokenSets {
  light: Record<string, string>;
  dark: Record<string, string>;
}

function extractBlock(css: string, selector: string): string {
  // Anchor the selector at the start of a line, immediately followed by its
  // opening brace. A plain substring search is not enough here: this file's
  // `@custom-variant dark (&:where([data-theme="dark"], ...))` line also
  // CONTAINS the literal text `[data-theme="dark"]`, earlier in the file
  // than the real `[data-theme="dark"] { ... }` rule — a naive `indexOf`
  // would match there first and then grab the wrong (light) block.
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const re = new RegExp(`(?:^|\\n)${escaped}\\s*\\{`);
  const match = re.exec(css);
  if (!match) {
    throw new Error(`theme-tokens: block "${selector}" not found`);
  }
  const braceStart = css.indexOf("{", match.index);
  let depth = 0;
  let i = braceStart;
  for (; i < css.length; i++) {
    if (css[i] === "{") depth++;
    if (css[i] === "}") {
      depth--;
      if (depth === 0) break;
    }
  }
  return css.slice(braceStart + 1, i);
}

function colorTokens(block: string): Record<string, string> {
  const out: Record<string, string> = {};
  const re = /--color-([a-z0-9-]+)\s*:\s*(#[0-9a-fA-F]{6})/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(block))) {
    out[m[1]] = m[2].toLowerCase();
  }
  return out;
}

export function readColorTokens(themeCssPath: string): ColorTokenSets {
  const css = readFileSync(themeCssPath, "utf-8");
  const lightBlock = extractBlock(css, "@theme");
  const darkBlock = extractBlock(css, '[data-theme="dark"]');
  return { light: colorTokens(lightBlock), dark: colorTokens(darkBlock) };
}
