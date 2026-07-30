package validation

// Task 4.6 / 1.6 (GREEN): rule 2 (continuity). Spec data-validation,
// "Rule 2 — continuity": the system MUST audit the WHOLE delivered
// series (prior + incoming) for continuity under its declared cadence,
// per segment, and MUST NOT pass merely because no prior data exists
// (MODIFIED requirement, task 1.5/1.6 -- Fase 0's version returned nil on
// the first run and otherwise walked forward from priorLatest alone,
// which is exactly what let the population series' decades of missing
// interior periods through undetected).
//
// The cadence-consistency check itself (a declared cadence that
// contradicts the observed history; an invented period inside a declared
// segment) is NOT reimplemented here -- it delegates to
// indicators.AssertCadence, the same mechanism the INE adapter's
// envelope.go runs over this run's own payload, so the two call sites can
// never silently diverge (design D-4). Once the declared cadence itself
// checks out, this rule walks the whole span period-by-period, skipping
// any period the declared cadence does not expect (indicators.
// CadenceExpects), and flags an expected-but-absent period exactly as
// before: a Finding naming it, unless it is on the per-series
// documented-gap allowlist.
//
// Period arithmetic is entirely indicators.Period's job (Next, Compare)
// — this rule never does its own date/period math, and it never touches
// the clock: the only "reference point" it uses is ctx.Prior, an
// explicit argument.

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

func earliestPeriod(obs []indicators.Observation) (indicators.Period, bool) {
	if len(obs) == 0 {
		return indicators.Period{}, false
	}
	earliest := obs[0].Period
	for _, o := range obs[1:] {
		if o.Period.Before(earliest) {
			earliest = o.Period
		}
	}
	return earliest, true
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

// Rule2Continuity is documented at length in the package's doc comment
// above and the method table (spec "A skipped period fails continuity" /
// "An allowlisted gap passes" / "The first run audits the interior
// rather than passing by default" / "A later run audits the interior,
// not only the tail" / "A declared cadence contradicting the observed
// history fails closed" / "A correctly declared segmented cadence
// passes"). With no incoming observation at all there is nothing this
// run delivered to audit -- rule 6 (non-emptiness) owns that case, not
// this rule.
func Rule2Continuity(ctx SeriesContext, incoming []indicators.Observation) []Finding {
	if _, hasIncoming := latestPeriod(incoming); !hasIncoming {
		return nil
	}

	all := make([]indicators.Observation, 0, len(ctx.Prior)+len(incoming))
	all = append(all, ctx.Prior...)
	all = append(all, incoming...)

	periods := make([]indicators.Period, 0, len(all))
	for _, o := range all {
		periods = append(periods, o.Period)
	}
	freq := periods[0].Frequency
	if ctx.Series.Frequency != "" {
		freq = ctx.Series.Frequency
	}

	if err := indicators.AssertCadence(periods, freq, ctx.Series.CadenceSegments); err != nil {
		return []Finding{{
			Rule: "rule2-continuity", Severity: SeverityBlock,
			Message: err.Error(),
		}}
	}

	earliest, _ := earliestPeriod(all)
	latest, _ := latestPeriod(all)

	var findings []Finding
	for cursor := earliest; !cursor.After(latest); cursor = cursor.Next() {
		if !indicators.CadenceExpects(cursor, freq, ctx.Series.CadenceSegments) {
			continue // not expected under this segment's declared cadence -- not a gap
		}
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
