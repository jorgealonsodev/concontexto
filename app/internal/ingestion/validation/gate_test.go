package validation_test

// Task 4.15 (RED, pure half): the publish gate's DECISION. Spec
// data-validation, "Publish gate": on failure nothing is published; on
// success the run's observations become current; and the gate "reports
// every failure, not only the first". This file proves Gate's pure
// verdict logic; the EFFECT half (wired to the slice-2 writer -- zero
// writes on Block, real writes + a recorded outcome on Publish) is
// proven by postgres.ApplyGate's own integration test, deliberately
// kept in the postgres package so this package's purity guard (task
// 4.1) never needs an exception.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func TestGate_NoFindingsPublishes(t *testing.T) {
	result := validation.Gate(nil)
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected GatePublish for zero findings, got %v", result.Outcome)
	}
}

func TestGate_InfoOnlyFindingsStillPublish(t *testing.T) {
	findings := []validation.Finding{
		{Rule: "rule2-continuity", Severity: validation.SeverityInfo, Message: "advisory only"},
	}
	result := validation.Gate(findings)
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected an Info-only finding to still publish, got %v", result.Outcome)
	}
}

func TestGate_AnyBlockingFindingBlocks(t *testing.T) {
	findings := []validation.Finding{
		{Rule: "rule3-plausibility", Severity: validation.SeverityBlock, Message: "out of range"},
	}
	result := validation.Gate(findings)
	if result.Outcome != validation.GateBlock {
		t.Fatalf("expected a single Block finding to block the run, got %v", result.Outcome)
	}
}

func TestGate_BlockRequiresSignoffAlsoBlocks(t *testing.T) {
	findings := []validation.Finding{
		{Rule: "rule4-revision", Severity: validation.SeverityBlockRequiresSignoff, Message: "deep revision"},
	}
	result := validation.Gate(findings)
	if result.Outcome != validation.GateBlock {
		t.Fatalf("expected SeverityBlockRequiresSignoff to block the run, got %v", result.Outcome)
	}
}

// TestGate_ReportsRule2AndRule3TogetherNotOnlyTheFirst is the real,
// end-to-end proof of spec's "The gate reports every failure, not only
// the first": it runs the REAL rule 2 and rule 3 against a fixture that
// genuinely violates both, concatenates their findings exactly as a
// caller running every rule would, and asserts Gate's verdict carries
// BOTH -- not a truncated first-failure-wins result.
func TestGate_ReportsRule2AndRule3TogetherNotOnlyTheFirst(t *testing.T) {
	maxValue := 40.0
	ctx := validation.SeriesContext{
		Series:     indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
		Validation: config.ValidationConfig{Plausibility: config.PlausibilityConfig{Max: &maxValue}},
		Prior: []indicators.Observation{
			{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}, Value: ptrRule4(10)},
		},
	}
	// Skips 2026-Q2 (rule 2 violation) AND is above the configured max
	// of 40 (rule 3 violation).
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 3}, Value: ptrRule4(50)},
	}

	var findings []validation.Finding
	findings = append(findings, validation.Rule2Continuity(ctx, incoming)...)
	findings = append(findings, validation.Rule3Plausibility(ctx, incoming)...)

	if len(findings) != 2 {
		t.Fatalf("test fixture must genuinely violate both rules 2 and 3, got %d findings: %+v", len(findings), findings)
	}

	result := validation.Gate(findings)
	if result.Outcome != validation.GateBlock {
		t.Fatalf("expected GateBlock, got %v", result.Outcome)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("expected the gate to report both violations, not just the first, got %d: %+v", len(result.Findings), result.Findings)
	}
	var sawRule2, sawRule3 bool
	for _, f := range result.Findings {
		switch f.Rule {
		case "rule2-continuity":
			sawRule2 = true
		case "rule3-plausibility":
			sawRule3 = true
		}
	}
	if !sawRule2 || !sawRule3 {
		t.Fatalf("expected both rule2-continuity and rule3-plausibility findings present, got: %+v", result.Findings)
	}
}
