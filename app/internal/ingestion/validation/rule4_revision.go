package validation

// Task 4.10 (GREEN): rule 4 (revision consistency). Spec data-validation,
// "Rule 4 — revision consistency": a run that revises any period older
// than the last N periods raises a human-review alert and blocks
// publication of that run until a human signs off. N defaults to 4
// (config.RevisionConfig's own doc comment: "resolved by the validation
// rule itself in Phase 4, not by this loader") and is overridable per
// series via config.ValidationConfig.Revision.MaxBackwardPeriods.
//
// Why this rule BLOCKS rather than warns (see the package's method
// table): a deep revision is either legitimate (a national-accounts
// methodology revision) or a parser bug silently rewriting history —
// those two cases are indistinguishable to the machine, so
// SeverityBlockRequiresSignoff hands the decision to a human rather
// than guessing either way.
//
// "Revises" is load-bearing: this rule only fires when ctx.Prior already
// holds a value at the same period that DIFFERS from the incoming one.
// A brand-new period being filled in for the first time is never a
// revision, however old it is — rule 2 (continuity) is what judges
// whether a gap in history is acceptable, not this rule.

import (
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// defaultMaxBackwardPeriods is applied when a series' config leaves
// Validation.Revision.MaxBackwardPeriods unset (spec "N MUST default to
// 4").
const defaultMaxBackwardPeriods = 4

// priorValueAt finds the value prior held at period p, or found=false
// if prior has no observation for that period at all.
func priorValueAt(prior []indicators.Observation, p indicators.Period) (*float64, bool) {
	for _, o := range prior {
		if o.Period.Equal(p) {
			return o.Value, true
		}
	}
	return nil, false
}

// valuesDiffer reports whether a and b represent different observed
// values, nil-safe (nil means "no value" -- e.g. a withdrawal).
func valuesDiffer(a, b *float64) bool {
	if a == nil || b == nil {
		return a != b
	}
	return *a != *b
}

// backDistance counts the Period.Next() steps from p to latest. The
// caller always derives latest as the maximum period across prior and
// incoming, so latest is never before p and this always terminates.
func backDistance(p, latest indicators.Period) int {
	distance := 0
	for cursor := p; !cursor.Equal(latest); cursor = cursor.Next() {
		distance++
	}
	return distance
}

// Rule4Revision is documented at length above and in the package's
// method table (spec "A revision within the default window passes" /
// "A deep revision blocks publication" / "A per-series override widens
// the window").
func Rule4Revision(ctx SeriesContext, incoming []indicators.Observation) []Finding {
	n := ctx.Validation.Revision.MaxBackwardPeriods
	if n <= 0 {
		n = defaultMaxBackwardPeriods
	}

	all := make([]indicators.Observation, 0, len(ctx.Prior)+len(incoming))
	all = append(all, ctx.Prior...)
	all = append(all, incoming...)
	latest, ok := latestPeriod(all)
	if !ok {
		return nil
	}

	var findings []Finding
	for _, o := range incoming {
		priorValue, hadPrior := priorValueAt(ctx.Prior, o.Period)
		if !hadPrior || !valuesDiffer(priorValue, o.Value) {
			continue // not a revision: brand-new period, or an identical resubmission
		}

		distance := backDistance(o.Period, latest)
		if distance < n {
			continue
		}

		findings = append(findings, Finding{
			Rule:     "rule4-revision",
			Severity: SeverityBlockRequiresSignoff,
			Period:   o.Period.String(),
			Message: fmt.Sprintf(
				"period %s is being revised %d period(s) before the latest known period %s, beyond the configured window of %d — requires human sign-off before publication",
				o.Period.String(), distance, latest.String(), n),
		})
	}
	return findings
}
