package validation_test

// Task 4.9 (RED): rule 4 (revision consistency). Spec data-validation,
// "Rule 4 — revision consistency": a run that revises any period older
// than the last N periods (default 4, overridable per series) raises a
// human-review alert and blocks publication until a human signs off.
// The three GWT scenarios this test proves, verbatim:
//
//   - "A revision within the default window passes" (N=4, third-most-
//     recent period revised)
//   - "A deep revision blocks publication" (N=4, nine periods back)
//   - "A per-series override widens the window" (N=12, nine periods
//     back passes)
//
// Plus two extra cases for triangulation, matching the thoroughness of
// rule2/rule3's own test suites: a brand-new period (no prior value at
// all) is never a "revision" no matter how old, and a series' first run
// (empty Prior) cannot violate this rule trivially.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func ptrRule4(v float64) *float64 { return &v }

func TestRule4Revision_WithinDefaultWindowPasses(t *testing.T) {
	ctx := validation.SeriesContext{
		Series: indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
		// no Validation.Revision override -> N defaults to 4
		Prior: []indicators.Observation{
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2028, Ordinal: 1}, Value: ptrRule4(100)},
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2027, Ordinal: 4}, Value: ptrRule4(99)},
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2027, Ordinal: 3}, Value: ptrRule4(95)}, // third-most-recent period
		},
	}
	// Revises the third-most-recent period (2027-Q3) with a different value.
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2027, Ordinal: 3}, Value: ptrRule4(96)},
	}

	findings := validation.Rule4Revision(ctx, incoming)
	if len(findings) != 0 {
		t.Fatalf("expected a revision within the default window to pass, got findings: %+v", findings)
	}
}

func TestRule4Revision_NinePeriodsBackBlocksPublication(t *testing.T) {
	ctx := validation.SeriesContext{
		Series: indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
		Prior: []indicators.Observation{
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2028, Ordinal: 1}, Value: ptrRule4(100)},
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2025, Ordinal: 4}, Value: ptrRule4(50)}, // nine periods before 2028-Q1
		},
	}
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2025, Ordinal: 4}, Value: ptrRule4(51)},
	}

	findings := validation.Rule4Revision(ctx, incoming)
	if len(findings) != 1 {
		t.Fatalf("expected exactly one blocking finding for a nine-period-back revision, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != validation.SeverityBlockRequiresSignoff {
		t.Errorf("expected SeverityBlockRequiresSignoff (human-review alert), got %q", findings[0].Severity)
	}
	if findings[0].Period != "2025-Q4" {
		t.Errorf("expected the finding to name period 2025-Q4, got %q", findings[0].Period)
	}
}

func TestRule4Revision_PerSeriesOverrideWidensTheWindow(t *testing.T) {
	ctx := validation.SeriesContext{
		Series:     indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
		Validation: config.ValidationConfig{Revision: config.RevisionConfig{MaxBackwardPeriods: 12}},
		Prior: []indicators.Observation{
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2028, Ordinal: 1}, Value: ptrRule4(100)},
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2025, Ordinal: 4}, Value: ptrRule4(50)},
		},
	}
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2025, Ordinal: 4}, Value: ptrRule4(51)},
	}

	findings := validation.Rule4Revision(ctx, incoming)
	if len(findings) != 0 {
		t.Fatalf("expected a nine-period-back revision to pass under a per-series N=12 override, got findings: %+v", findings)
	}
}

func TestRule4Revision_BrandNewPeriodIsNeverARevision(t *testing.T) {
	ctx := validation.SeriesContext{
		Series: indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
		Prior: []indicators.Observation{
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2028, Ordinal: 1}, Value: ptrRule4(100)},
		},
	}
	// A period far in the past with NO prior value at all is a first-time
	// fill, not a revision -- however old it is.
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2010, Ordinal: 1}, Value: ptrRule4(10)},
	}

	findings := validation.Rule4Revision(ctx, incoming)
	if len(findings) != 0 {
		t.Fatalf("expected a brand-new historical period to never be flagged as a revision, got findings: %+v", findings)
	}
}

func TestRule4Revision_SeriesFirstRunPassesTrivially(t *testing.T) {
	ctx := validation.SeriesContext{
		Series: indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
		// Prior is empty: this series has never published before.
	}
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2028, Ordinal: 1}, Value: ptrRule4(100)},
	}

	findings := validation.Rule4Revision(ctx, incoming)
	if len(findings) != 0 {
		t.Fatalf("expected a series' first run to trivially pass rule 4, got findings: %+v", findings)
	}
}
