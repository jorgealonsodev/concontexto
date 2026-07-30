// indicator-page spec, "The three page states of PRD §6.1.3" — task 9a.7
// (RED), reopened by verify-report CRITICAL-4. Pure functions under test:
// the derivation of the presentation-layer page state FROM the export
// artifact's own `pageState` object (previously a hand-maintained constant
// in `content/indicators/methodology.ts` — the defect), and the exact
// banner copy per derived state.
//
// This state stays separate from the artifact's two-state `freshness` field
// (design.md D-2: freshness and "did the last run pass validation" are two
// different axes — a series can be freshness "fresh" and still be showing a
// stale, last-known-good value because its most recent run failed
// validation).
import { describe, expect, it } from "vitest";
import { pageStateBannerCopy, pageStateFromArtifact } from "../../src/lib/indicator/pageState";
import { es } from "../../src/i18n/es";

describe("pageStateFromArtifact — the artifact is the source of truth", () => {
  it("derives the fresh state, dropping the arm-inapplicable nulls", () => {
    expect(pageStateFromArtifact({ kind: "fresh", lastCorrectUpdate: null, successorSlug: null })).toEqual({
      kind: "fresh",
    });
  });

  it("derives a validation failure carrying the artifact's own last-correct-update date", () => {
    expect(
      pageStateFromArtifact({ kind: "validation-failure", lastCorrectUpdate: "2026-07-29", successorSlug: null }),
    ).toEqual({ kind: "validation-failure", lastCorrectUpdate: "2026-07-29" });
  });

  // The load-bearing case: a series whose very FIRST ingestion run failed
  // validation has no correct update to name, and the Go half deliberately
  // emits the failure with a null date rather than fabricating one
  // (principle P4) or downgrading the state to "fresh". The derivation must
  // carry the null through, never coerce it to a string and never silently
  // reclassify the state.
  it("derives a validation failure with NO date, without inventing one and without downgrading to fresh", () => {
    const state = pageStateFromArtifact({
      kind: "validation-failure",
      lastCorrectUpdate: null,
      successorSlug: null,
    });
    expect(state).toEqual({ kind: "validation-failure", lastCorrectUpdate: null });
  });

  it("derives a discontinued state with its successor slug", () => {
    expect(
      pageStateFromArtifact({ kind: "discontinued", lastCorrectUpdate: null, successorSlug: "ocupados-epa" }),
    ).toEqual({ kind: "discontinued", successorSlug: "ocupados-epa" });
  });

  it("derives a discontinued state with no successor", () => {
    expect(pageStateFromArtifact({ kind: "discontinued", lastCorrectUpdate: null, successorSlug: null })).toEqual({
      kind: "discontinued",
      successorSlug: null,
    });
  });
});

describe("pageStateBannerCopy", () => {
  it("renders no banner for the fresh (normal) state", () => {
    expect(pageStateBannerCopy({ kind: "fresh" })).toBeNull();
  });

  it("renders the EXACT spec-mandated validation-failure banner naming the last correct update date", () => {
    // The SENTENCE is the spec's own verbatim string and is still pinned
    // character for character. What changed is only the value substituted
    // into `{fecha}`: the artifact's bare `2026-07-29` was a machine date
    // sitting in a Spanish sentence, and `lib/format/date.ts` now names the
    // day the way the rest of the site names one.
    const copy = pageStateBannerCopy({ kind: "validation-failure", lastCorrectUpdate: "2026-07-29" });
    expect(copy).toBe(
      "Última actualización correcta: 29 de julio de 2026. La fuente ha publicado un dato que no ha superado nuestra validación automática; estamos revisándolo",
    );
  });

  it("leaves the artifact's own calendar date untouched in the STATE, formatting only the copy", () => {
    // The boundary this change must not cross: `pageStateFromArtifact` still
    // carries the machine value verbatim, so anything that consumes the
    // state rather than the sentence keeps reading `2026-07-29`.
    const state = pageStateFromArtifact({ kind: "validation-failure", lastCorrectUpdate: "2026-07-29", successorSlug: null });
    expect(state).toEqual({ kind: "validation-failure", lastCorrectUpdate: "2026-07-29" });
  });

  // Same recorded fact, different sentence: with no correct update to name,
  // the copy MUST NOT open with "Última actualización correcta" (which would
  // assert a previous correct update that never happened) and MUST NOT
  // contain any date at all.
  it("renders a dateless validation-failure banner when the artifact names no last correct update — mutation-checked", () => {
    const copy = pageStateBannerCopy({ kind: "validation-failure", lastCorrectUpdate: null });
    expect(copy).toBe(es.page.validationFailureBannerNoDate);
    expect(copy).not.toContain("Última actualización correcta");
    expect(copy).not.toMatch(/\d/);
    // It still states the fact the banner exists to state.
    expect(copy).toContain("no ha superado nuestra validación automática");
  });

  it("never emits the string \"null\" for a dateless validation failure", () => {
    expect(pageStateBannerCopy({ kind: "validation-failure", lastCorrectUpdate: null })).not.toContain("null");
    expect(pageStateBannerCopy({ kind: "validation-failure", lastCorrectUpdate: null })).not.toContain("undefined");
  });

  it("renders a permanent discontinued banner", () => {
    const copy = pageStateBannerCopy({ kind: "discontinued", successorSlug: null });
    expect(copy).not.toBeNull();
    expect(copy!.length).toBeGreaterThan(0);
  });

  it("renders the same discontinued banner whether or not a successor is configured — the link is separate", () => {
    expect(pageStateBannerCopy({ kind: "discontinued", successorSlug: "ocupados-epa" })).toBe(
      pageStateBannerCopy({ kind: "discontinued", successorSlug: null }),
    );
  });
});
