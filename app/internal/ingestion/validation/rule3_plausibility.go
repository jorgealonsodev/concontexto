package validation

// Task 4.8 (GREEN): rule 3 (plausibility). Spec data-validation, "Rule 3
// — plausibility": each value is checked against a per-series absolute
// min/max, and the period-over-period change is checked against a
// per-series threshold, EXEMPTED exactly at a period recorded as a
// break.
//
// Rule3Plausibility CONSUMES already-resolved breaks via ctx.Breaks —
// it does NOT load config/rupturas.yaml itself. That editorial YAML and
// its reconcile step land in slice 7; this rule must not, and does not,
// depend on that work existing (indicators.Break's doc comment explains
// the same boundary from the domain-type side).

import (
	"fmt"
	"math"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func breakAt(breaks []indicators.Break, p indicators.Period) bool {
	for _, b := range breaks {
		if b.Period.Equal(p) {
			return true
		}
	}
	return false
}

// previousValue finds the value at the period immediately before p
// within obs (prior + incoming, combined), by frequency-aware Period
// arithmetic — never by array position, since neither slice is
// guaranteed sorted.
func previousValue(obs []indicators.Observation, p indicators.Period) (float64, bool) {
	target := p.Previous()
	for _, o := range obs {
		if o.Period.Equal(target) && o.Value != nil {
			return *o.Value, true
		}
	}
	return 0, false
}

// Rule3Plausibility is documented at length in the package's method
// table (spec "An out-of-range value fails" / "A large jump at a
// recorded break passes" / "The same jump away from a break fails" —
// this last pair is the asymmetry that is the entire point of the
// rule: it is what stops a methodology change from being published as
// if it were real economic movement).
func Rule3Plausibility(ctx SeriesContext, incoming []indicators.Observation) []Finding {
	var findings []Finding
	plaus := ctx.Validation.Plausibility

	all := make([]indicators.Observation, 0, len(ctx.Prior)+len(incoming))
	all = append(all, ctx.Prior...)
	all = append(all, incoming...)

	for _, o := range incoming {
		if o.Value == nil {
			continue
		}

		if plaus.Min != nil && *o.Value < *plaus.Min {
			findings = append(findings, Finding{
				Rule: "rule3-plausibility", Severity: SeverityBlock, Period: o.Period.String(),
				Message: fmt.Sprintf("value %v is below the configured minimum %v", *o.Value, *plaus.Min),
			})
		}
		if plaus.Max != nil && *o.Value > *plaus.Max {
			findings = append(findings, Finding{
				Rule: "rule3-plausibility", Severity: SeverityBlock, Period: o.Period.String(),
				Message: fmt.Sprintf("value %v is above the configured maximum %v", *o.Value, *plaus.Max),
			})
		}

		if plaus.MaxDeltaAbs == nil {
			continue
		}
		prev, ok := previousValue(all, o.Period)
		if !ok {
			continue
		}
		delta := math.Abs(*o.Value - prev)
		if delta <= *plaus.MaxDeltaAbs {
			continue
		}
		if breakAt(ctx.Breaks, o.Period) {
			continue // exempted: a recorded break explains the jump
		}
		findings = append(findings, Finding{
			Rule: "rule3-plausibility", Severity: SeverityBlock, Period: o.Period.String(),
			Message: fmt.Sprintf("period-over-period change of %v exceeds the configured threshold %v with no recorded break at %s", delta, *plaus.MaxDeltaAbs, o.Period.String()),
		})
	}

	return findings
}
