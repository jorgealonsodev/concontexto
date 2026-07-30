// Every route entry point declares an icon, and the icon it declares is a
// real, reviewable file in this repository.
//
// THE DEFECT THIS DRIVES. A browser that finds no `<link rel="icon">` in a
// document's `<head>` falls back to requesting `/favicon.ico` from the site
// root. Nothing served one, so every single page load — homepage, all six
// indicator pages, the workbench — ended with exactly one console error:
//
//   Failed to load resource: the server responded with a status of 404
//   (Not Found) @ /favicon.ico
//
// One error on every page is not a cosmetic complaint. It is the only entry
// in the console on a site whose whole thesis is that its pipeline is
// inspectable (P5), so it is the first thing anyone who opens dev tools
// sees, and it trains a reader (and a future maintainer) to expect noise
// there rather than silence.
//
// WHY THIS TEST SCANS RATHER THAN NAMES THE THREE FILES. There are three
// documents with their own `<html>` today (`/`, `/indicador/{slug}`,
// `/workbench`), and the defect was not "one page is missing a favicon" —
// it was "no page has one". A hand-maintained list of three paths would go
// stale the first time a fourth route is added, and would go stale
// SILENTLY, which is exactly how `es.ts`'s own no-inlined-copy scan missed
// `src/pages/index.astro` and `src/workbench/pages/index.astro` for two
// slices (verify-report WARNING-18). This test therefore discovers the
// entry points the same way a browser does: anything that opens its own
// `<html>` document owns its own `<head>`.
//
// WHY AN SVG AND NOT AN `.ico`. A binary `.ico` is an opaque blob in a diff:
// a reviewer can see that it changed and never what it changed to. An SVG
// favicon is source — the palette values below are literally greppable
// against `theme.css`, which is what lets the last assertion here hold the
// icon to ADR-8's one surviving constraint (palette is free, accessibility
// and identity are not). The disclosed cost is Safari before 17, which
// ignores `image/svg+xml` icons and re-requests `/favicon.ico`; that is a
// narrower blast radius than an unreviewable binary, and it is recorded
// here rather than discovered later.
import { globSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const WEB_ROOT = join(import.meta.dirname, "..", "..");
const FAVICON_PATH = join(WEB_ROOT, "public", "favicon.svg");

/** Every `.astro` file under `src/` that opens its own document. Astro
 * components are fragments; only a route entry point carries `<html>`, and
 * only a route entry point therefore owns a `<head>` a browser reads. */
function documentEntryPoints(): string[] {
  const files = globSync("src/**/*.astro", { cwd: WEB_ROOT }).map((file) => join(WEB_ROOT, file));
  return files.filter((file) => readFileSync(file, "utf8").includes("<html"));
}

describe("favicon — the one console error every page load carried", () => {
  it("finds every route entry point, so this scan cannot silently cover fewer pages than exist", () => {
    const entries = documentEntryPoints();
    // Not an exact-set assertion on paths: the point of the scan is that it
    // keeps finding NEW routes. The floor is the three that exist today, so
    // a glob that silently stopped matching (a moved directory, a changed
    // extension) fails here instead of passing vacuously over zero files.
    expect(entries.length).toBeGreaterThanOrEqual(3);
  });

  it("declares an SVG icon in EVERY document that has a head, not just the one that was noticed", () => {
    const entries = documentEntryPoints();
    for (const entry of entries) {
      const source = readFileSync(entry, "utf8");
      expect(source, `${entry} opens an <html> document but declares no icon`).toMatch(
        /<link\s+rel="icon"\s+href="\/favicon\.svg"\s+type="image\/svg\+xml"\s*\/>/,
      );
    }
  });

  it("ships the file those links point at, as real reviewable SVG source", () => {
    const svg = readFileSync(FAVICON_PATH, "utf8");
    expect(svg).toMatch(/^<svg\b/);
    expect(svg).toContain('xmlns="http://www.w3.org/2000/svg"');
    // A square viewBox: browsers rasterise a favicon into a square box, and
    // a non-square drawing would be letterboxed into it rather than filling
    // the tab's icon slot.
    expect(svg).toMatch(/viewBox="0 0 (\d+) \1"/);
  });

  it("is well-formed XML, because a browser parses a standalone .svg as XML and not as HTML", () => {
    // A REAL defect this caught. The first draft of this icon explained its
    // palette in an XML comment that named the design tokens it used —
    // `--color-accent`, `--color-bg`. XML forbids `--` inside a comment, so
    // the file was not well-formed and Chromium refused to render it as an
    // image at all. Nothing said so: inlined into an HTML page it parsed
    // fine (HTML parsing is lenient), the file served with a 200 and
    // `image/svg+xml`, and the only symptom was a tab with no icon — the
    // same symptom as having no favicon, which is the thing this whole file
    // exists to prevent.
    //
    // Checked structurally rather than with a parser dependency: these are
    // the two well-formedness rules a hand-written SVG realistically trips,
    // and the first is the one that already bit.
    const svg = readFileSync(FAVICON_PATH, "utf8");

    for (const [, body] of svg.matchAll(/<!--([\s\S]*?)-->/g)) {
      expect(body, "an XML comment contains a double hyphen, which XML forbids").not.toContain("--");
    }

    // Character data outside comments must not carry a raw `&` or `<`.
    const withoutComments = svg.replace(/<!--[\s\S]*?-->/g, "");
    expect(withoutComments).not.toMatch(/&(?!(?:[a-zA-Z][a-zA-Z0-9]*|#\d+|#x[0-9a-fA-F]+);)/);
    // Every `<` opens a tag: it is followed by a name, a `/` or a `!`.
    expect(withoutComments).not.toMatch(/<(?![a-zA-Z/!?])/);
  });

  it("adapts to a dark browser theme rather than assuming a light one", () => {
    // Browser chrome is the one surface this project's own `data-theme`
    // attribute cannot reach: the icon renders in the tab strip, outside
    // the document. `prefers-color-scheme` is the only signal available
    // there, and it is evaluated INSIDE the SVG by the browser that paints
    // it — which is why the icon has to carry its own stylesheet.
    const svg = readFileSync(FAVICON_PATH, "utf8");
    expect(svg).toContain("prefers-color-scheme: dark");
  });

  it("paints only colours the design-token file already authors (ADR-8)", () => {
    // ADR-8 lifted the vetoed-hue list, so this is NOT a neutrality check.
    // It is an identity check: the icon is the site's smallest possible
    // reproduction of itself, and an icon painted in a hue that appears
    // nowhere in `theme.css` would be a second, undocumented palette. Both
    // themes' accent and background values are already authored there.
    const svg = readFileSync(FAVICON_PATH, "utf8");
    const theme = readFileSync(join(WEB_ROOT, "src", "styles", "theme.css"), "utf8").toLowerCase();
    const hexes = [...svg.toLowerCase().matchAll(/#[0-9a-f]{3,8}\b/g)].map((m) => m[0]);
    expect(hexes.length, "the icon paints nothing at all").toBeGreaterThan(0);
    for (const hex of hexes) {
      expect(theme, `${hex} is painted by the favicon but authored nowhere in theme.css`).toContain(hex);
    }
  });
});
