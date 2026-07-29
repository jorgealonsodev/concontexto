package pipelinelog_test

// Task 9.5 (RED)/9.6 (GREEN): pure, offline proof of the exact field set
// a completed (or failed) run's structured log carries (spec
// pipeline-operations, "A run is reconstructible from logs alone").
//
// Disclosure: pipelinelog.go's production code and this test file were
// authored together, not test-first (see this batch's own apply-progress
// note for the full disclosure, matching scheduler_test.go's).

import (
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/pipelinelog"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func TestAttrs_CompletedRunCarriesEveryRequiredField(t *testing.T) {
	e := pipelinelog.Entry{
		RunID: 42, Source: "ine", Dataset: "ine-epa", Series: "tasa-de-paro-epa",
		Outcome: "publish", RawFileHash: "deadbeef", Duration: 250 * time.Millisecond,
		Verdicts: []string{"info: rule1-schema: ok"},
	}
	attrs := pipelinelog.Attrs(e)

	want := map[string]bool{
		"run_id": false, "source": false, "dataset": false, "series": false,
		"outcome": false, "raw_file_hash": false, "duration": false, "verdicts": false,
	}
	for _, a := range attrs {
		if _, ok := want[a.Key]; ok {
			want[a.Key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("expected attrs to carry key %q, got %+v", key, attrs)
		}
	}
	for _, a := range attrs {
		if a.Key == "failed_rules" {
			t.Error("expected a Publish-outcome entry with no findings to carry no failed_rules key")
		}
	}
}

func TestAttrs_FailedRunAdditionallyCarriesWhichRulesFailed(t *testing.T) {
	e := pipelinelog.Entry{
		RunID: 7, Source: "eurostat", Dataset: "eurostat-une", Series: "paro-armonizado-eurostat",
		Outcome: "block", RawFileHash: "cafebabe", Duration: time.Second,
		Verdicts:    []string{"block: rule3-plausibility: value out of range"},
		FailedRules: []string{"rule3-plausibility"},
	}
	attrs := pipelinelog.Attrs(e)

	var gotFailedRules bool
	for _, a := range attrs {
		if a.Key == "failed_rules" {
			gotFailedRules = true
		}
	}
	if !gotFailedRules {
		t.Error("expected a failed run's attrs to carry failed_rules")
	}
}

func TestVerdicts_FormatsEveryFindingInOrder(t *testing.T) {
	findings := []validation.Finding{
		{Rule: "rule1-schema", Severity: validation.SeverityInfo, Message: "ok"},
		{Rule: "rule3-plausibility", Severity: validation.SeverityBlock, Message: "out of range"},
	}
	got := pipelinelog.Verdicts(findings)
	if len(got) != 2 {
		t.Fatalf("expected 2 verdict lines, got %d: %v", len(got), got)
	}
	if got[0] != "info: rule1-schema: ok" {
		t.Errorf("unexpected first verdict line: %q", got[0])
	}
	if got[1] != "block: rule3-plausibility: out of range" {
		t.Errorf("unexpected second verdict line: %q", got[1])
	}
}

func TestVerdicts_EmptyFindingsReturnsNil(t *testing.T) {
	if got := pipelinelog.Verdicts(nil); got != nil {
		t.Errorf("expected nil for zero findings, got %v", got)
	}
}

func TestFailedRules_OnlyBlockingSeveritiesAndDeduplicated(t *testing.T) {
	findings := []validation.Finding{
		{Rule: "rule1-schema", Severity: validation.SeverityInfo, Message: "advisory only"},
		{Rule: "rule3-plausibility", Severity: validation.SeverityBlock, Period: "2026-Q1", Message: "out of range"},
		{Rule: "rule3-plausibility", Severity: validation.SeverityBlock, Period: "2026-Q2", Message: "out of range too"},
		{Rule: "rule4-revision", Severity: validation.SeverityBlockRequiresSignoff, Message: "deep revision"},
	}
	got := pipelinelog.FailedRules(findings)
	want := []string{"rule3-plausibility", "rule4-revision"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected %v, got %v", want, got)
			break
		}
	}
}

func TestFailedRules_NoBlockingFindingsReturnsEmpty(t *testing.T) {
	findings := []validation.Finding{{Rule: "rule1-schema", Severity: validation.SeverityInfo, Message: "ok"}}
	if got := pipelinelog.FailedRules(findings); len(got) != 0 {
		t.Errorf("expected zero failed rules for an all-Info finding set, got %v", got)
	}
}
