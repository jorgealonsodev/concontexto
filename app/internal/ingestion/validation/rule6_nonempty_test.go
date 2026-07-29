package validation_test

// Task 4.13 (RED): rule 6 (non-empty result). Spec data-validation,
// "Rule 6 — non-empty result": a run that yields zero observations MUST
// fail validation and MUST NOT be published -- a distinct failure class
// from a transport error, verified live: a dead Eurostat dimension code
// (ECOICOP v1's retired coicop18=CP00 vs the live all-items TOTAL)
// returns HTTP 200 with a structurally valid but empty JSON-stat
// payload. Without this rule, a renamed dimension code would silently
// publish nothing while every other health signal stayed green.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func TestRule6NonEmpty_StructurallyValidEmptyPayloadFails(t *testing.T) {
	ctx := validation.SeriesContext{Series: indicators.Series{Slug: "prc-hicp-minr"}}
	incoming := []indicators.Observation{} // decoded successfully, but zero observations

	findings := validation.Rule6NonEmpty(ctx, incoming)
	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding for a zero-observation payload, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != validation.SeverityBlock {
		t.Errorf("expected SeverityBlock, got %q", findings[0].Severity)
	}
	if findings[0].Rule != "rule6-non-empty" {
		t.Errorf("expected a rule identifier distinct from any transport-error class, got %q", findings[0].Rule)
	}
}

func TestRule6NonEmpty_NilIncomingAlsoFails(t *testing.T) {
	ctx := validation.SeriesContext{Series: indicators.Series{Slug: "prc-hicp-minr"}}

	findings := validation.Rule6NonEmpty(ctx, nil)
	if len(findings) != 1 {
		t.Fatalf("expected a nil incoming slice to fail the same as an empty one, got %d findings", len(findings))
	}
}

func TestRule6NonEmpty_NonEmptyPayloadPasses(t *testing.T) {
	ctx := validation.SeriesContext{Series: indicators.Series{Slug: "prc-hicp-minr"}}
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 6}, Value: ptrRule4(2.3)},
	}

	findings := validation.Rule6NonEmpty(ctx, incoming)
	if len(findings) != 0 {
		t.Fatalf("expected a non-empty payload to pass rule 6, got findings: %+v", findings)
	}
}
