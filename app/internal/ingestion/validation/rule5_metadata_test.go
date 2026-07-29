package validation_test

// Task 4.11 (RED): rule 5 (metadata completeness). Spec data-validation,
// "Rule 5 — metadata completeness": a series MUST NOT be published
// unless source, unit, frequency and licence are all present (PRD
// §9.3.5). This is the rule that makes principle P2 (total
// traceability) structurally enforced rather than aspirational: an
// observation with no licence has no attribution, and unattributable
// data must never reach a page.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func completeSeries() indicators.Series {
	return indicators.Series{
		Slug:      "test-series",
		Unit:      "%",
		Frequency: indicators.FrequencyQuarterly,
		Source:    "ine",
		Licence:   "CC BY 4.0",
	}
}

func TestRule5MetadataCompleteness_MissingLicenceFailsAndNothingPublishes(t *testing.T) {
	series := completeSeries()
	series.Licence = "" // the one field this test omits
	ctx := validation.SeriesContext{Series: series}
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}, Value: ptrRule4(1.0)},
	}

	findings := validation.Rule5MetadataCompleteness(ctx, incoming)
	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding for a missing licence, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != validation.SeverityBlock {
		t.Errorf("expected SeverityBlock, got %q", findings[0].Severity)
	}
	if findings[0].Rule != "rule5-metadata-completeness" {
		t.Errorf("expected rule5-metadata-completeness, got %q", findings[0].Rule)
	}
}

func TestRule5MetadataCompleteness_CompleteSeriesPasses(t *testing.T) {
	ctx := validation.SeriesContext{Series: completeSeries()}
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}, Value: ptrRule4(1.0)},
	}

	findings := validation.Rule5MetadataCompleteness(ctx, incoming)
	if len(findings) != 0 {
		t.Fatalf("expected a fully-described series to pass rule 5, got findings: %+v", findings)
	}
}

func TestRule5MetadataCompleteness_MissingMultipleFieldsNamesEachOne(t *testing.T) {
	// Only Slug set: source, unit, frequency and licence are all absent.
	ctx := validation.SeriesContext{Series: indicators.Series{Slug: "bare-series"}}
	incoming := []indicators.Observation{}

	findings := validation.Rule5MetadataCompleteness(ctx, incoming)
	if len(findings) != 4 {
		t.Fatalf("expected one finding per missing field (source, unit, frequency, licence), got %d: %+v", len(findings), findings)
	}
}
