package validation_test

// Task 4.7 (RED): rule 3 (plausibility). Spec data-validation, "Rule 3
// — plausibility": each value is checked against a per-series absolute
// min/max, and the period-over-period change is checked against a
// per-series threshold, EXEMPTED exactly at a period recorded as a
// break. The exemption is keyed to break dates (here: already-resolved
// Break.Period, per SeriesContext's contract — task 4.8's "consumes
// resolved Breaks"), and the asymmetry between the break/no-break cases
// is the entire point of the rule: it is what stops a methodology
// change from being published as if it were real economic movement.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func ptr(f float64) *float64 { return &f }

func TestRule3Plausibility_OutOfRangeValueFails(t *testing.T) {
	ctx := validation.SeriesContext{
		Validation: config.ValidationConfig{
			Plausibility: config.PlausibilityConfig{Min: ptr(0), Max: ptr(40)},
		},
	}
	incoming := []indicators.Observation{obsAt(mustPeriod(t, "2026-Q1"), 412.0)}

	findings := validation.Rule3Plausibility(ctx, incoming)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(findings), findings)
	}
	if !containsSubstring(findings[0].Message, "maximum") {
		t.Errorf("expected the failure to name the range violation, got message %q", findings[0].Message)
	}
}

func TestRule3Plausibility_LargeJumpAtRecordedBreakPasses(t *testing.T) {
	breakPeriod := mustPeriod(t, "2026-Q2")
	ctx := validation.SeriesContext{
		Validation: config.ValidationConfig{
			Plausibility: config.PlausibilityConfig{MaxDeltaAbs: ptr(5)},
		},
		Prior:  []indicators.Observation{obsAt(mustPeriod(t, "2026-Q1"), 10)},
		Breaks: []indicators.Break{{ID: "TESTCOD001-break", Period: breakPeriod, Kind: "methodology"}},
	}
	incoming := []indicators.Observation{obsAt(breakPeriod, 100)} // delta = 90, far above threshold

	findings := validation.Rule3Plausibility(ctx, incoming)

	if len(findings) != 0 {
		t.Fatalf("expected the break to exempt this boundary, got findings: %+v", findings)
	}
}

func TestRule3Plausibility_SameJumpWithNoRecordedBreakFails(t *testing.T) {
	period := mustPeriod(t, "2026-Q2")
	ctx := validation.SeriesContext{
		Validation: config.ValidationConfig{
			Plausibility: config.PlausibilityConfig{MaxDeltaAbs: ptr(5)},
		},
		Prior: []indicators.Observation{obsAt(mustPeriod(t, "2026-Q1"), 10)},
		// No Breaks recorded at all — same jump, no exemption.
	}
	incoming := []indicators.Observation{obsAt(period, 100)} // identical delta to the passing case above

	findings := validation.Rule3Plausibility(ctx, incoming)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding (no break to exempt this boundary), got %d: %+v", len(findings), findings)
	}
	if findings[0].Period != "2026-Q2" {
		t.Errorf("expected the failure to be tied to period 2026-Q2, got %q", findings[0].Period)
	}
}

func TestRule3Plausibility_WithinRangeAndDeltaPasses(t *testing.T) {
	ctx := validation.SeriesContext{
		Validation: config.ValidationConfig{
			Plausibility: config.PlausibilityConfig{Min: ptr(0), Max: ptr(40), MaxDeltaAbs: ptr(5)},
		},
		Prior: []indicators.Observation{obsAt(mustPeriod(t, "2026-Q1"), 10)},
	}
	incoming := []indicators.Observation{obsAt(mustPeriod(t, "2026-Q2"), 12)}

	findings := validation.Rule3Plausibility(ctx, incoming)

	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}
