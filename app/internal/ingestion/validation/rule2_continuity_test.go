package validation_test

// Task 4.5 (RED): rule 2 (continuity). Spec data-validation, "Rule 2 —
// continuity": the newest period must be the next expected one for the
// series frequency, and no new undocumented gap may appear. Period
// labels are normalised (indicators.NormalizePeriodLabel, task 4.5's
// other half, already GREEN) before observations ever reach this rule
// — the scenarios below build Observations FROM raw INE/Eurostat-shaped
// labels precisely to prove the whole pipeline is source-independent
// end to end, not just the parser in isolation.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func mustPeriod(t *testing.T, raw string) indicators.Period {
	t.Helper()
	p, err := indicators.NormalizePeriodLabel(raw)
	if err != nil {
		t.Fatalf("NormalizePeriodLabel(%q): %v", raw, err)
	}
	return p
}

func obsAt(p indicators.Period, value float64) indicators.Observation {
	v := value
	return indicators.Observation{Period: p, Value: &v}
}

func TestRule2Continuity_SkippedPeriodFailsNamingIt(t *testing.T) {
	prior := []indicators.Observation{obsAt(mustPeriod(t, "2026-Q1"), 10)}
	incoming := []indicators.Observation{obsAt(mustPeriod(t, "T3 2026"), 12)} // no Q2 delivered

	ctx := validation.SeriesContext{Prior: prior}
	findings := validation.Rule2Continuity(ctx, incoming)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Period != "2026-Q2" {
		t.Errorf("expected the failure to name period 2026-Q2, got %q (message: %q)", findings[0].Period, findings[0].Message)
	}
}

func TestRule2Continuity_AllowlistedGapPasses(t *testing.T) {
	prior := []indicators.Observation{obsAt(mustPeriod(t, "2026-Q1"), 10)}
	incoming := []indicators.Observation{obsAt(mustPeriod(t, "2026-Q3"), 12)} // reproduces exactly the documented gap

	ctx := validation.SeriesContext{
		Prior: prior,
		Validation: config.ValidationConfig{
			Continuity: config.ContinuityConfig{DocumentedGaps: []string{"2026-Q2"}},
		},
	}
	findings := validation.Rule2Continuity(ctx, incoming)

	if len(findings) != 0 {
		t.Fatalf("expected no findings for an allowlisted gap, got %+v", findings)
	}
}

func TestRule2Continuity_NextExpectedPeriodPasses(t *testing.T) {
	prior := []indicators.Observation{obsAt(mustPeriod(t, "M06 2026"), 5)}
	incoming := []indicators.Observation{obsAt(mustPeriod(t, "2026-07"), 6)}

	ctx := validation.SeriesContext{Prior: prior}
	findings := validation.Rule2Continuity(ctx, incoming)

	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestRule2Continuity_NoPriorObservationPassesTrivially(t *testing.T) {
	incoming := []indicators.Observation{obsAt(mustPeriod(t, "2026-Q1"), 10)}

	ctx := validation.SeriesContext{}
	findings := validation.Rule2Continuity(ctx, incoming)

	if len(findings) != 0 {
		t.Fatalf("expected no findings on a series' first run, got %+v", findings)
	}
}
