// The break band is emitted by TWO independent renderers, and nothing until
// this file stopped them from drifting apart.
//
// Why two renderers exist at all: `ChartIsland.svelte` RE-RENDERS the chart's
// break list rather than hydrating `IndicatorChart.astro`'s DOM — the
// Astro/Svelte boundary forces it (slice 8's declared risk, carried into
// slice 9a's composition decision). So `BreakBand.astro` and
// `ChartIsland.svelte` each hand-write the same `.break-band` /
// `.break-band__trigger` / `.break-band__tooltip` markup.
//
// Why that is dangerous, demonstrated rather than hypothesised: the tooltip's
// reveal rules originally lived in `BreakBand.astro`'s own Astro-scoped
// `<style>`. Astro scoping is invisible to Svelte by design, so the island's
// copy never received them and its tooltip shipped painted at `opacity: 1`
// over the rest of the page. The rules were moved to
// `src/styles/components.css` (see that file's header), which fixes the
// instance — but NOTHING prevented the next divergence. Rename the class in
// one file, add a `data-testid` to one file, move a rule back into a scoped
// block, and the two renderers part company again in exactly the same way.
//
// So this test asserts three things that must hold TOGETHER:
//
//   1. Both renderers emit the same class names and the same `data-testid`
//      for the same three elements (parity).
//   2. Both emit the same ELEMENT for the trigger, and it is a real
//      `<button>` (see the a11y note on `TRIGGER_TAG` below).
//   3. The shared stylesheet actually carries rules for those exact names,
//      and neither component has taken them back into a scoped `<style>`.
//
// (3) is not redundant with (1). A rename applied consistently to BOTH
// components but not to `components.css` keeps them in perfect parity with
// each other while reproducing the original defect exactly — both tooltips
// unstyled, both painted at rest. Parity alone cannot see that.
//
// Failure messages below deliberately name BOTH files. The entire purpose of
// this guard is that someone editing one of them finds out about the other.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SRC_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../src");

const ASTRO_RENDERER = path.join(SRC_DIR, "components/BreakBand.astro");
const SVELTE_RENDERER = path.join(SRC_DIR, "components/ChartIsland.svelte");
const SHARED_STYLESHEET = path.join(SRC_DIR, "styles/components.css");

/** Repo-relative, so a failure message points at a path a human can open. */
function label(absolutePath: string): string {
  return `src/${path.relative(SRC_DIR, absolutePath)}`;
}

const RENDERERS = [ASTRO_RENDERER, SVELTE_RENDERER] as const;
const BOTH = `${label(ASTRO_RENDERER)} and ${label(SVELTE_RENDERER)}`;

/**
 * The element the trigger MUST be, in BOTH renderers.
 *
 * verify-report WARNING-7: the trigger used to be `<span tabindex="0">`, and
 * every build emitted `a11y_no_noninteractive_tabindex` for the Svelte copy
 * (Astro runs no equivalent check, so the identical `BreakBand.astro` line
 * was silently unreported — the warning was never a Svelte-only problem).
 *
 * A `<button>` rather than a suppression comment or a bare `role`: the
 * element genuinely IS an interactive control. It is focusable on purpose, it
 * is the `aria-describedby` anchor for the tooltip, and activating it reveals
 * supplementary detail — which is what the WAI-ARIA APG tooltip pattern
 * builds on a focusable button. `role="button"` would additionally oblige us
 * to hand-write Enter/Space key handling that the native element already has,
 * on a component whose whole point is shipping no script. Suppressing the
 * warning would have left the accessibility tree saying "static text" about
 * something reachable by Tab.
 *
 * It also fixes a real touch defect rather than only a lint line: a
 * `<span tabindex="0">` does not reliably take focus when tapped on iOS
 * Safari, so a touch reader — who has no hover either — could not reach the
 * tooltip at all. Tapping a `<button>` focuses it, and `:focus-within`
 * reveals the tooltip with no JavaScript.
 *
 * This constant is asserted against BOTH renderers precisely because
 * converting one of them alone would BE the divergence this file exists to
 * catch.
 */
const TRIGGER_TAG = "button";

interface ExtractedElement {
  tag: string;
  classes: string[];
  testId: string | null;
}

/**
 * Extracts the single element whose `class` attribute STARTS WITH `token`.
 *
 * Anchoring on the leading class token rather than the tag name is what lets
 * one extractor read both an `.astro` and a `.svelte` file, and what lets the
 * tag name itself be a thing under test rather than a thing assumed.
 *
 * Requires EXACTLY ONE match per file. A second emitter of the same markup,
 * or a copy-pasted block inside one file, must fail loudly here rather than
 * be silently compared against whichever copy happened to come first — a
 * third divergent renderer is the same bug as two.
 */
function extractElement(source: string, filePath: string, token: string): ExtractedElement {
  // The lookahead is load-bearing: without it, `break-band` also prefix-matches
  // `break-band__trigger` and `break-band__tooltip`, and the container lookup
  // would find three elements instead of one. Requiring whitespace or the
  // closing quote after the token makes it a whole-class-name match.
  const classAttr = new RegExp(`class="(${token}(?=[\\s"])[^"]*)"`, "g");
  const matches = [...source.matchAll(classAttr)];
  expect(
    matches.length,
    `expected exactly one element with a class attribute beginning "${token}" in ${label(filePath)}, found ${matches.length}. ` +
      `This markup is emitted by ${BOTH} and must stay a single element in each.`,
  ).toBe(1);

  const match = matches[0];
  const tagStart = source.lastIndexOf("<", match.index);
  const tagName = /^<([a-zA-Z][a-zA-Z0-9-]*)/.exec(source.slice(tagStart))?.[1];
  expect(tagName, `could not read the tag name of the "${token}" element in ${label(filePath)}`).toBeDefined();

  const openingTag = source.slice(tagStart, source.indexOf(">", match.index));
  return {
    tag: tagName!,
    // Sorted: the two renderers must agree on the SET of classes, not on the
    // order they were typed in. Ordering is not a contract; a missing or
    // extra utility class is.
    classes: match[1].trim().split(/\s+/).sort(),
    testId: /data-testid="([^"]*)"/.exec(openingTag)?.[1] ?? null,
  };
}

function extractFromBoth(token: string): Record<string, ExtractedElement> {
  return Object.fromEntries(
    RENDERERS.map((file) => [label(file), extractElement(readFileSync(file, "utf-8"), file, token)]),
  );
}

/** Every element of the band that both renderers hand-write, and that must
 * therefore stay in parity. */
const SHARED_CLASS_TOKENS = ["break-band", "break-band__trigger", "break-band__tooltip"] as const;

/**
 * The subset of those tokens that the SHARED stylesheet must carry a rule
 * for — the container (which is the `:hover`/`:focus-within` ancestor) and
 * the tooltip (hidden at rest).
 *
 * `break-band__trigger` is deliberately absent: it carries no cross-renderer
 * rule at all, because everything about its appearance is Tailwind utilities
 * written inline in each component. It is a parity and identification hook,
 * not a styling one. Asserting a rule existed for it would demand a rule with
 * nothing to say, so it is excluded here and still fully covered by the
 * parity assertions above.
 */
const STYLED_CLASS_TOKENS = ["break-band", "break-band__tooltip"] as const;

describe("break-band markup parity across its two renderers", () => {
  it.each(SHARED_CLASS_TOKENS)(
    'the "%s" element is identical in both renderers — same tag, same classes, same data-testid',
    (token) => {
      const [astro, svelte] = RENDERERS.map((file) => extractElement(readFileSync(file, "utf-8"), file, token));

      expect(
        svelte.tag,
        `break-band DIVERGENCE on "${token}": ${label(ASTRO_RENDERER)} renders <${astro.tag}>, ` +
          `${label(SVELTE_RENDERER)} renders <${svelte.tag}>. Both files hand-write this markup because the ` +
          `island re-renders the break list instead of hydrating it — change one and you MUST change the other.`,
      ).toBe(astro.tag);

      expect(
        svelte.classes,
        `break-band DIVERGENCE on "${token}": the class lists differ between ${BOTH}.\n` +
          `  ${label(ASTRO_RENDERER)}: ${astro.classes.join(" ")}\n` +
          `  ${label(SVELTE_RENDERER)}: ${svelte.classes.join(" ")}\n` +
          `Shared rules in src/styles/components.css are matched by class name, so a class present in only ` +
          `one renderer means that renderer silently loses the rule — which is how the island's tooltip once ` +
          `shipped painted at opacity 1 over the page.`,
      ).toEqual(astro.classes);

      expect(
        svelte.testId,
        `break-band DIVERGENCE on "${token}": data-testid is ${JSON.stringify(astro.testId)} in ` +
          `${label(ASTRO_RENDERER)} but ${JSON.stringify(svelte.testId)} in ${label(SVELTE_RENDERER)}. ` +
          `The e2e gates select break-band tooltips page-wide, across both renderers at once; a testid on ` +
          `only one of them makes those assertions quietly cover half of what they claim.`,
      ).toBe(astro.testId);
    },
  );

  // Finding 2's fix and Finding 4's guard meet here. Converting the trigger
  // in one renderer only would have created exactly the divergence above, so
  // both were converted, and this pins the outcome in both at once.
  it(`the trigger is a real <${TRIGGER_TAG}> in both renderers, not a focusable non-interactive element`, () => {
    const triggers = extractFromBoth("break-band__trigger");
    for (const [file, trigger] of Object.entries(triggers)) {
      expect(
        trigger.tag,
        `${file} renders the break-band trigger as <${trigger.tag}>. It must be <${TRIGGER_TAG}> in BOTH ` +
          `${BOTH}: the element is keyboard-focusable and anchors the tooltip's aria-describedby, so a ` +
          `noninteractive tag misdescribes it to assistive technology (Svelte reports this as ` +
          `a11y_no_noninteractive_tabindex; Astro reports nothing, which is why the same line went unnoticed there).`,
      ).toBe(TRIGGER_TAG);
    }
  });

  it("neither renderer reintroduces tabindex on the trigger, which a button already has", () => {
    for (const file of RENDERERS) {
      const source = readFileSync(file, "utf-8");
      const triggerBlock = source.slice(
        source.lastIndexOf("<", source.indexOf('class="break-band__trigger')),
        source.indexOf(">", source.indexOf('class="break-band__trigger')),
      );
      expect(
        /tabindex/i.test(triggerBlock),
        `${label(file)} declares tabindex on the break-band trigger. A <${TRIGGER_TAG}> is already in the tab ` +
          `order; an explicit tabindex there is the exact construct verify-report WARNING-7 asked to remove.`,
      ).toBe(false);
    }
  });

  it.each(STYLED_CLASS_TOKENS)(
    'the shared stylesheet still carries rules for ".%s", so neither renderer is styling it alone',
    (token) => {
      const css = readFileSync(SHARED_STYLESHEET, "utf-8");
      expect(
        css.includes(`.${token}`),
        `src/styles/components.css has no rule mentioning ".${token}", but ${BOTH} both emit it. ` +
          `Renaming the class in both components while leaving this stylesheet behind keeps the two renderers ` +
          `in perfect parity AND reproduces the original defect in both at once — which is why this assertion ` +
          `is not covered by the parity checks above.`,
      ).toBe(true);
    },
  );

  it("neither component takes the shared break-band rules back into a component-scoped <style>", () => {
    for (const file of RENDERERS) {
      const source = readFileSync(file, "utf-8");
      const scopedStyles = [...source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((m) => m[1]);
      for (const block of scopedStyles) {
        expect(
          /\.break-band/.test(block),
          `${label(file)} declares a .break-band rule inside a component-scoped <style>. Astro scopes to its ` +
            `component and Svelte scopes to its own, so such a rule reaches ONE of ${BOTH} and never the other. ` +
            `That is the precise mechanism by which the island's tooltip shipped at opacity 1. Shared markup's ` +
            `rules belong in src/styles/components.css.`,
        ).toBe(false);
      }
    }
  });
});
