package validation_test

// Task 1.5 (RED) / 1.6 (GREEN): Rule2Continuity audits the WHOLE
// delivered series (prior + incoming) under its declared cadence, per
// segment, rather than returning nil merely because no prior data exists
// or walking forward from priorLatest alone (spec data-validation,
// "Rule 2 — continuity", MODIFIED requirement).

import (
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func quarterlyObs(t *testing.T, year, quarter int, value float64) indicators.Observation {
	t.Helper()
	return obsAt(mustPeriod(t, quarterlyLabel(year, quarter)), value)
}

func quarterlyLabel(year, quarter int) string {
	return indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: year, Ordinal: quarter}.String()
}

func TestRule2Continuity_FirstRunAuditsInteriorGap(t *testing.T) {
	// spec: "The first run audits the interior rather than passing by
	// default" -- no prior data at all, and the delivered history itself
	// has an interior gap (2020-Q3 missing).
	var incoming []indicators.Observation
	for _, q := range []int{1, 2, 4} { // Q3 skipped
		incoming = append(incoming, quarterlyObs(t, 2020, q, 10))
	}
	incoming = append(incoming, quarterlyObs(t, 2021, 1, 11))

	ctx := validation.SeriesContext{Series: indicators.Series{Frequency: indicators.FrequencyQuarterly}}
	findings := validation.Rule2Continuity(ctx, incoming)

	if len(findings) == 0 {
		t.Fatal("expected the first run to fail on its own interior gap")
	}
	found := false
	for _, f := range findings {
		if f.Period == "2020-Q3" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a finding naming the missing period 2020-Q3, got %+v", findings)
	}
}

func TestRule2Continuity_LaterRunAuditsInteriorNotOnlyTail(t *testing.T) {
	// spec: "A later run audits the interior, not only the tail" -- the
	// STORED history already has an interior gap; the new incoming tail
	// is perfectly contiguous. Fase 0's rule (walking forward from
	// priorLatest only) would have passed this; the corrected rule must
	// not.
	var prior []indicators.Observation
	for _, q := range []int{1, 2, 4} { // Q3 2020 missing from what was already stored
		prior = append(prior, quarterlyObs(t, 2020, q, 10))
	}
	incoming := []indicators.Observation{quarterlyObs(t, 2021, 1, 11)}

	ctx := validation.SeriesContext{Series: indicators.Series{Frequency: indicators.FrequencyQuarterly}, Prior: prior}
	findings := validation.Rule2Continuity(ctx, incoming)

	found := false
	for _, f := range findings {
		if f.Period == "2020-Q3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the interior gap in the STORED history to be flagged, got %+v", findings)
	}
}

func TestRule2Continuity_DeclaredCadenceContradictingObservedHistoryFailsClosed(t *testing.T) {
	// spec: "A declared cadence contradicting the observed history fails
	// closed" -- a uniformly-quarterly-declared series (no
	// cadence_segments) whose delivered history is semiannual across its
	// span.
	var incoming []indicators.Observation
	for y := 2000; y <= 2020; y++ {
		incoming = append(incoming,
			quarterlyObs(t, y, 1, 100),
			quarterlyObs(t, y, 3, 101),
		)
	}

	ctx := validation.SeriesContext{Series: indicators.Series{Frequency: indicators.FrequencyQuarterly}}
	findings := validation.Rule2Continuity(ctx, incoming)

	if len(findings) == 0 {
		t.Fatal("expected a uniformly-declared cadence to fail closed against a semiannual history")
	}
	if !strings.Contains(findings[0].Message, "quarterly") {
		t.Errorf("expected the finding to name the declared cadence, got: %q", findings[0].Message)
	}
}

func TestRule2Continuity_CorrectlyDeclaredSegmentedCadencePasses(t *testing.T) {
	// spec: "A correctly declared segmented cadence passes".
	var incoming []indicators.Observation
	for y := 2000; y <= 2020; y++ {
		incoming = append(incoming,
			quarterlyObs(t, y, 1, 100),
			quarterlyObs(t, y, 3, 101),
		)
	}
	// Continue the quarterly (dense) era from 2021-Q1 onward.
	for _, q := range []int{1, 2, 3, 4} {
		incoming = append(incoming, quarterlyObs(t, 2021, q, 102))
	}

	segFrom, err := indicators.ParseCadenceSegment("2000-Q1", "2020-Q4", "semiannual", []int{1, 3})
	if err != nil {
		t.Fatalf("ParseCadenceSegment: %v", err)
	}
	segRest, err := indicators.ParseCadenceSegment("2021-Q1", "", "quarterly", nil)
	if err != nil {
		t.Fatalf("ParseCadenceSegment: %v", err)
	}

	ctx := validation.SeriesContext{
		Series: indicators.Series{
			Frequency:       indicators.FrequencyQuarterly,
			CadenceSegments: []indicators.CadenceSegment{segFrom, segRest},
		},
	}
	findings := validation.Rule2Continuity(ctx, incoming)

	if len(findings) != 0 {
		t.Fatalf("expected a correctly declared segmented cadence to pass, got %+v", findings)
	}
}
