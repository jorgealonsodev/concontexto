package validation

// Task 4.14 (GREEN): rule 6 (non-empty result). Spec data-validation,
// "Rule 6 — non-empty result": a run that yields zero observations MUST
// fail validation, classified DISTINCTLY from a transport error. This
// rule was added after live verification, not present in PRD §9.3's
// original five (see the package's method table) -- a dead Eurostat
// dimension code (ECOICOP v1's retired coicop18=CP00 vs the live
// all-items TOTAL) returns HTTP 200 with a structurally valid,
// well-formed JSON-stat payload whose "value" object is simply empty.
// Every earlier rule in this package only ever inspects individual
// incoming observations; none of them fires against an empty slice --
// without this rule, a renamed dimension code would silently publish
// nothing while every other health signal stayed green.
//
// The distinctness from a transport error is structural, not a flag on
// this Finding: a transport error (design.md's sourceerr.FailureClass,
// slice 5a/6, not built yet) means the adapter never even produced an
// `incoming` slice to validate -- the run would already have failed
// before reaching this rule. Rule6NonEmpty only ever fires when the
// payload WAS successfully decoded and IS structurally valid, just
// empty; its Rule field ("rule6-non-empty") is never confusable with a
// FailureClass value.

import (
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// Rule6NonEmpty is documented at length above and in the package's
// method table (spec "A structurally valid empty payload fails").
func Rule6NonEmpty(ctx SeriesContext, incoming []indicators.Observation) []Finding {
	if len(incoming) > 0 {
		return nil
	}
	return []Finding{{
		Rule:     "rule6-non-empty",
		Severity: SeverityBlock,
		Message: fmt.Sprintf(
			"series %q produced a structurally valid payload with zero observations -- distinct from a transport error; nothing will be written or published for this run",
			ctx.Series.Slug),
	}}
}
