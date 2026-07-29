package validation

// Task 4.6 (GREEN): rule 2 (continuity). Spec data-validation, "Rule 2
// — continuity": the newest incoming period must be the expected
// successor of the latest prior period, and every period strictly
// between them must be either present or on the per-series
// documented-gap allowlist.
//
// Period arithmetic is entirely indicators.Period's job (Next,
// Compare) — this rule never does its own date/period math beyond
// walking Period.Next() forward, and it never touches the clock: the
// only "reference point" it uses is ctx.Prior, an explicit argument.

import (
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func latestPeriod(obs []indicators.Observation) (indicators.Period, bool) {
	if len(obs) == 0 {
		return indicators.Period{}, false
	}
	latest := obs[0].Period
	for _, o := range obs[1:] {
		if o.Period.After(latest) {
			latest = o.Period
		}
	}
	return latest, true
}

func hasPeriod(obs []indicators.Observation, p indicators.Period) bool {
	for _, o := range obs {
		if o.Period.Equal(p) {
			return true
		}
	}
	return false
}

func isDocumentedGap(gaps []string, p indicators.Period) bool {
	label := p.String()
	for _, g := range gaps {
		if g == label {
			return true
		}
	}
	return false
}

// Rule2Continuity is documented at length in the package's method table
// (spec "A skipped period fails continuity" / "An allowlisted gap
// passes"). With no prior observation at all there is nothing to
// compare against yet, so a series' very first run cannot violate
// continuity.
func Rule2Continuity(ctx SeriesContext, incoming []indicators.Observation) []Finding {
	priorLatest, hadPrior := latestPeriod(ctx.Prior)
	newLatest, hasIncoming := latestPeriod(incoming)
	if !hadPrior || !hasIncoming {
		return nil
	}

	all := make([]indicators.Observation, 0, len(ctx.Prior)+len(incoming))
	all = append(all, ctx.Prior...)
	all = append(all, incoming...)

	var findings []Finding
	for cursor := priorLatest.Next(); !cursor.After(newLatest); cursor = cursor.Next() {
		if hasPeriod(all, cursor) {
			continue
		}
		if isDocumentedGap(ctx.Validation.Continuity.DocumentedGaps, cursor) {
			continue
		}
		findings = append(findings, Finding{
			Rule: "rule2-continuity", Severity: SeverityBlock,
			Period:  cursor.String(),
			Message: fmt.Sprintf("period %s is missing and not on the documented-gap allowlist", cursor.String()),
		})
	}

	return findings
}
