package validation

// Task 4.12 (GREEN): rule 5 (metadata completeness). Spec
// data-validation, "Rule 5 — metadata completeness": a series MUST NOT
// be published unless source, unit, frequency and licence are all
// present (PRD §9.3.5), evaluated before anything is published. This is
// the rule that makes principle P2 (total traceability) structurally
// enforced rather than aspirational: an observation with no licence has
// no attribution, and unattributable data must never reach a page.
//
// Rule5MetadataCompleteness checks ctx.Series only -- it never re-parses
// config/sources/*.yaml or config/series/*.yaml itself. Whichever
// caller builds SeriesContext is responsible for resolving Source and
// Licence onto indicators.Series (this batch adds both fields to
// Series for exactly that purpose; see indicators/series.go).

import (
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// Rule5MetadataCompleteness is documented at length above and in the
// package's method table (spec "A series missing licence cannot be
// published ... nothing is published for that series").
func Rule5MetadataCompleteness(ctx SeriesContext, incoming []indicators.Observation) []Finding {
	var findings []Finding

	required := [...]struct {
		field string
		value string
	}{
		{"source", ctx.Series.Source},
		{"unit", ctx.Series.Unit},
		{"frequency", string(ctx.Series.Frequency)},
		{"licence", ctx.Series.Licence},
	}

	for _, r := range required {
		if r.value != "" {
			continue
		}
		findings = append(findings, Finding{
			Rule:     "rule5-metadata-completeness",
			Severity: SeverityBlock,
			Message: fmt.Sprintf(
				"series %q is missing required metadata field %q; a series cannot be published without it",
				ctx.Series.Slug, r.field),
		})
	}

	return findings
}
