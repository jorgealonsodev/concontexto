// indicator-page spec, "The three page states of PRD §6.1.3": fresh data
// (normal, no banner); source failure/validation-not-passed (the last valid
// datum is served with a banner naming the date of the last correct
// update); discontinued series (a permanent banner, with a successor link
// when configured). "The chart MUST NOT be hidden in any state" — enforced
// by `IndicatorPage.astro` always composing the chart regardless of this
// module's return value, never by this module itself (a pure string
// producer has no rendering authority to hide anything).
//
// This state is DELIBERATELY separate from the export artifact's own
// `freshness` field (design.md D-2's two-state semaphore: "fresh" |
// "source-pending", both about whether the SOURCE has published the
// expected period). A series can be `freshness: "fresh"` while its most
// recent run still failed validation and is showing an earlier value — the
// two axes are independent facts.
//
// verify-report CRITICAL-4 remediation: the EXPORT ARTIFACT is now the
// source of truth for this state (`SeriesDoc.pageState`, required by
// `lib/export/schema.ts`). It was previously a hand-maintained constant in
// `content/indicators/methodology.ts` with all six series pinned to
// `{ kind: "fresh" }`, which meant a real validation failure produced no
// banner on the live site without a source edit and a redeploy — the whole
// point of the requirement. `content/indicators/methodology.ts` keeps only
// its genuinely editorial fields (measures, doesNotMeasure, the
// next-publication label); the page state moved to the data path.
//
// The banner copy stays HERE rather than in the artifact: the Go half
// carries facts (which state, which date, which successor) and this layer
// owns the Spanish wording, so a copy change never requires regenerating
// data and a data change never rewrites reader-facing prose.
import type { ArtifactPageState } from "../export/schema";
import { es } from "../../i18n/es";

/** The presentation-layer view of the artifact's page state: the same three
 * states, with each arm carrying ONLY the field that applies to it.
 *
 * `lastCorrectUpdate` is nullable on purpose. The Go half emits
 * `kind: "validation-failure"` with a null date when a series' very FIRST
 * ingestion run failed validation — there has never been a correct update to
 * name. Substituting any stand-in date (run start, extraction instant,
 * today) would make the banner assert a provenance fact that never happened
 * (principle P4), and downgrading the state to "fresh" would silently
 * suppress a recorded failure. Both refusals are correct, so this type must
 * be able to represent "a failure, with no date" and
 * `pageStateBannerCopy` must have wording for it. */
export type PageState =
  | { kind: "fresh" }
  | { kind: "validation-failure"; lastCorrectUpdate: string | null }
  | { kind: "discontinued"; successorSlug: string | null };

/** Derives the page state from the artifact's own `pageState` object.
 *
 * Deliberately NOT the identity function over `ArtifactPageState`, even
 * though the schema's discriminated union already guarantees the same three
 * arms: this drops each arm's inapplicable nulls, so a rendering caller
 * cannot reach for `lastCorrectUpdate` on a discontinued page or
 * `successorSlug` on a fresh one. Nothing is inferred, defaulted or
 * reclassified here — every value comes from the artifact, or is absent
 * because the artifact says it is absent. */
export function pageStateFromArtifact(artifactPageState: ArtifactPageState): PageState {
  switch (artifactPageState.kind) {
    case "fresh":
      return { kind: "fresh" };
    case "validation-failure":
      return { kind: "validation-failure", lastCorrectUpdate: artifactPageState.lastCorrectUpdate };
    case "discontinued":
      return { kind: "discontinued", successorSlug: artifactPageState.successorSlug };
  }
}

/** The banner text for a page state, or `null` for the fresh (normal) state
 * — a page renders no banner at all in that case.
 *
 * The date, when there is one, is printed VERBATIM as the artifact's own
 * `YYYY-MM-DD` calendar date (the schema enforces that shape). This matches
 * the convention `MethodologySheetFields.astro` already applies to
 * `extractedAt`: a machine date is rendered as the fact it is, rather than
 * reformatted into prose this codebase has no locale-formatting vocabulary
 * for. */
export function pageStateBannerCopy(state: PageState): string | null {
  switch (state.kind) {
    case "fresh":
      return null;
    case "validation-failure":
      // A null date is a recorded fact, not a missing value to paper over —
      // see this module's own type doc. The dateless wording states the same
      // failure without naming a date and without implying a previous
      // correct update.
      return state.lastCorrectUpdate === null
        ? es.page.validationFailureBannerNoDate
        : es.page.validationFailureBanner(state.lastCorrectUpdate);
    case "discontinued":
      // The successor LINK is composed by the page template, not folded into
      // this string: the banner sentence is identical whether or not a
      // successor exists, and only the template can turn a slug into a route.
      return es.page.discontinuedBanner;
  }
}
