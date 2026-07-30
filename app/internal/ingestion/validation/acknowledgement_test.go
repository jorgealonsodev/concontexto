package validation_test

// RED for the acknowledgement registry's PURE half. Spec data-validation,
// "Acknowledged findings".
//
// The gap this closes: `types.go` has declared SeverityBlockRequiresSignoff
// since PR 4b and `rule4_revision.go` documents at length why it exists --
// "a deep revision is either legitimate (a national-accounts methodology
// revision) or a parser bug silently rewriting history -- those two cases
// are indistinguishable to the machine, so SeverityBlockRequiresSignoff
// hands the decision to a human rather than guessing either way" -- while
// gate.go treated it identically to SeverityBlock and NOTHING anywhere
// resolved it. The comment described handing a decision to a human and
// gave the human no way to hand it back.
//
// Every scenario below exists to pin one design constraint that makes this
// mechanism safe rather than a blanket mute button:
//
//   - an acknowledgement is scoped to (series, period, rule) by EXACT
//     equality on all three, so it can never be written as "ignore rule 3
//     for this series" or "ignore everything in 2020" (the widening tests);
//   - it PINS the value the human reviewed, so a later revision of that
//     datum invalidates it rather than silently carrying the old approval
//     onto a number nobody looked at (the stale tests);
//   - it is severity-agnostic: it resolves SeverityBlock and
//     SeverityBlockRequiresSignoff alike, which is what finally gives rule
//     4's documented intent its other half;
//   - it is NOT rule-open: only the rules whose findings are genuinely
//     human judgement calls admit one (the non-acknowledgeable-rule test).

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// The real ocupados-epa numbers this mechanism exists for: INE
// DATOS_SERIE/EPA387796 reports 2020-Q1 = 19681.3 and 2020-Q2 = 18607.2
// (thousands of employed persons), a period-over-period fall of 1074.1
// against the series' configured max_delta_abs of 1000 -- the COVID-19
// lockdown quarter. Using the real values here keeps the unit test and
// config/reconocimientos.yaml describing the same fact.
const (
	ocupadosQ1 = 19681.3
	ocupadosQ2 = 18607.2
)

func quarter(year, ordinal int) indicators.Period {
	return indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: year, Ordinal: ordinal}
}

func ackFloat(v float64) *float64 { return &v }

// covidAck is the acknowledgement every scenario below starts from: the
// exact shape config/reconocimientos.yaml projects into the database.
func covidAck() validation.Acknowledgement {
	return validation.Acknowledgement{
		ID:       "ocupados-epa-2020-q2-covid",
		SeriesID: "ocupados-epa",
		Period:   "2020-Q2",
		Rule:     "rule3-plausibility",
		Value:    ocupadosQ2,
		By:       "Jorge Alonso",
		Note:     "COVID-19 lockdown quarter; the fall is real economics, not a methodology change.",
	}
}

// covidIncoming is the observation set the acknowledged finding concerns.
func covidIncoming() []indicators.Observation {
	return []indicators.Observation{
		{Period: quarter(2020, 1), Value: ackFloat(ocupadosQ1), Status: indicators.ObservationStatusDefinitive},
		{Period: quarter(2020, 2), Value: ackFloat(ocupadosQ2), Status: indicators.ObservationStatusDefinitive},
	}
}

func plausibilityFinding(period string) validation.Finding {
	return validation.Finding{
		Rule: "rule3-plausibility", Severity: validation.SeverityBlock, Period: period,
		Message: "period-over-period change of 1074.1 exceeds the configured threshold 1000 with no recorded break at " + period,
	}
}

// TestGateWithAcknowledgements_ResolvesAMatchingBlockingFinding is the
// headline behaviour: a finding an acknowledgement covers stops blocking,
// and the run publishes under an outcome that is NOT the ordinary
// "publish" -- spec "An acknowledged publish is distinguishable from a
// clean one".
func TestGateWithAcknowledgements_ResolvesAMatchingBlockingFinding(t *testing.T) {
	findings := []validation.Finding{plausibilityFinding("2020-Q2")}

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{covidAck()}, covidIncoming())

	if result.Outcome != validation.GatePublishOverridden {
		t.Fatalf("expected GatePublishOverridden, got %v", result.Outcome)
	}
	if len(result.Overridden) != 1 {
		t.Fatalf("expected exactly one recorded override, got %d: %+v", len(result.Overridden), result.Overridden)
	}
	if result.Overridden[0].AcknowledgementID != "ocupados-epa-2020-q2-covid" {
		t.Errorf("expected the override to name the acknowledgement that resolved it, got %q",
			result.Overridden[0].AcknowledgementID)
	}
	if result.Overridden[0].AcknowledgedBy != "Jorge Alonso" {
		t.Errorf("expected the override to name the human who acknowledged it, got %q",
			result.Overridden[0].AcknowledgedBy)
	}
	if len(result.UnresolvedFindings()) != 0 {
		t.Errorf("expected no unresolved findings, got %+v", result.UnresolvedFindings())
	}
	// The ORIGINAL finding must survive verbatim in Findings: spec "The
	// gate reports every failure, not only the first" is not weakened by an
	// acknowledgement, and an operator must still see the magnitude.
	var sawOriginal bool
	for _, f := range result.Findings {
		if f.Rule == "rule3-plausibility" && f.Severity == validation.SeverityBlock {
			sawOriginal = true
		}
	}
	if !sawOriginal {
		t.Errorf("expected the acknowledged finding to stay reported verbatim in Findings, got %+v", result.Findings)
	}
}

// TestGateWithAcknowledgements_ResolvesBlockRequiresSignoffAlike is rule
// 4's other half: the severity the codebase has declared since PR 4b
// finally has a resolution path, and it is the SAME path SeverityBlock
// takes -- the mechanism is severity-agnostic.
func TestGateWithAcknowledgements_ResolvesBlockRequiresSignoffAlike(t *testing.T) {
	findings := []validation.Finding{{
		Rule: "rule4-revision", Severity: validation.SeverityBlockRequiresSignoff, Period: "2020-Q2",
		Message: "period 2020-Q2 is being revised 8 period(s) before the latest known period — requires human sign-off before publication",
	}}
	ack := covidAck()
	ack.ID = "ocupados-epa-2020-q2-revision"
	ack.Rule = "rule4-revision"

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{ack}, covidIncoming())

	if result.Outcome != validation.GatePublishOverridden {
		t.Fatalf("expected SeverityBlockRequiresSignoff to be resolvable by an acknowledgement, got %v", result.Outcome)
	}
	if len(result.Overridden) != 1 {
		t.Fatalf("expected one override, got %+v", result.Overridden)
	}
}

// TestGateWithAcknowledgements_NeverWidensBeyondItsPeriod: an
// acknowledgement of 2020-Q2 must not resolve the same rule at 2020-Q3.
// This is the mechanism's most important safety property -- a registry
// that can be widened to "everything in 2020" is worse than the gap it
// fills.
func TestGateWithAcknowledgements_NeverWidensBeyondItsPeriod(t *testing.T) {
	findings := []validation.Finding{plausibilityFinding("2020-Q3")}
	incoming := append(covidIncoming(),
		indicators.Observation{Period: quarter(2020, 3), Value: ackFloat(ocupadosQ2), Status: indicators.ObservationStatusDefinitive})

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{covidAck()}, incoming)

	if result.Outcome != validation.GateBlock {
		t.Fatalf("an acknowledgement of 2020-Q2 must NOT resolve a finding at 2020-Q3, got %v", result.Outcome)
	}
	if len(result.Overridden) != 0 {
		t.Errorf("expected zero overrides, got %+v", result.Overridden)
	}
}

// TestGateWithAcknowledgements_NeverWidensBeyondItsRule: the same period,
// a different rule, is a different finding and a different human
// judgement.
func TestGateWithAcknowledgements_NeverWidensBeyondItsRule(t *testing.T) {
	findings := []validation.Finding{{
		Rule: "rule4-revision", Severity: validation.SeverityBlockRequiresSignoff, Period: "2020-Q2",
		Message: "deep revision",
	}}

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{covidAck()}, covidIncoming())

	if result.Outcome != validation.GateBlock {
		t.Fatalf("a rule3-plausibility acknowledgement must NOT resolve a rule4-revision finding, got %v", result.Outcome)
	}
}

// TestGateWithAcknowledgements_NeverWidensBeyondItsSeries: an
// acknowledgement carries a series and only ever applies to that series,
// even if an identically-shaped finding appears elsewhere.
func TestGateWithAcknowledgements_NeverWidensBeyondItsSeries(t *testing.T) {
	findings := []validation.Finding{plausibilityFinding("2020-Q2")}

	result := validation.GateWithAcknowledgements("parados-epa", findings,
		[]validation.Acknowledgement{covidAck()}, covidIncoming())

	if result.Outcome != validation.GateBlock {
		t.Fatalf("an ocupados-epa acknowledgement must NOT resolve a parados-epa finding, got %v", result.Outcome)
	}
}

// TestGateWithAcknowledgements_StaleWhenTheAcknowledgedValueChanged is the
// staleness guard, and the reason Acknowledgement carries Value at all.
// The human's statement is "I looked at 2020-Q2 = 18607.2 and confirm it".
// If a later run reports a DIFFERENT number at that period, that statement
// is no longer about the datum in front of the gate, so it must not
// resolve anything -- and the operator must be told WHY, rather than
// watching the original finding reappear for no visible reason.
func TestGateWithAcknowledgements_StaleWhenTheAcknowledgedValueChanged(t *testing.T) {
	findings := []validation.Finding{plausibilityFinding("2020-Q2")}
	revised := []indicators.Observation{
		{Period: quarter(2020, 1), Value: ackFloat(ocupadosQ1), Status: indicators.ObservationStatusDefinitive},
		{Period: quarter(2020, 2), Value: ackFloat(18500.0), Status: indicators.ObservationStatusDefinitive},
	}

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{covidAck()}, revised)

	if result.Outcome != validation.GateBlock {
		t.Fatalf("a revised value must invalidate the acknowledgement pinned to the old one, got %v", result.Outcome)
	}
	if len(result.Overridden) != 0 {
		t.Errorf("expected zero overrides for a stale acknowledgement, got %+v", result.Overridden)
	}
	stale := findingByRule(result.Findings, "acknowledgement-stale")
	if stale == nil {
		t.Fatalf("expected an explicit acknowledgement-stale finding, got %+v", result.Findings)
	}
	if stale.Severity != validation.SeverityBlock {
		t.Errorf("expected the stale finding to block, got %q", stale.Severity)
	}
	for _, want := range []string{"ocupados-epa-2020-q2-covid", "18607.2", "18500"} {
		if !containsSubstring(stale.Message, want) {
			t.Errorf("expected the stale message to name %q so an operator can act on it, got %q", want, stale.Message)
		}
	}
}

// TestGateWithAcknowledgements_StaleWhenThePeriodIsAbsentFromTheRun: an
// acknowledgement pinned to a value that this run does not deliver at all
// cannot be checked, so it must fail closed exactly like a changed value.
func TestGateWithAcknowledgements_StaleWhenThePeriodIsAbsentFromTheRun(t *testing.T) {
	findings := []validation.Finding{plausibilityFinding("2020-Q2")}
	withoutQ2 := []indicators.Observation{
		{Period: quarter(2020, 1), Value: ackFloat(ocupadosQ1), Status: indicators.ObservationStatusDefinitive},
	}

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{covidAck()}, withoutQ2)

	if result.Outcome != validation.GateBlock {
		t.Fatalf("an acknowledgement whose period this run does not carry must fail closed, got %v", result.Outcome)
	}
	if findingByRule(result.Findings, "acknowledgement-stale") == nil {
		t.Errorf("expected an explicit acknowledgement-stale finding, got %+v", result.Findings)
	}
}

// TestGateWithAcknowledgements_UnusedAcknowledgementIsReportedButNeverBlocks:
// an acknowledgement that resolves nothing is editorial debt, not a
// failure. Blocking on it would punish exactly the good outcome (the
// underlying condition went away); staying silent would let the registry
// accumulate approvals nobody can account for.
func TestGateWithAcknowledgements_UnusedAcknowledgementIsReportedButNeverBlocks(t *testing.T) {
	result := validation.GateWithAcknowledgements("ocupados-epa", nil,
		[]validation.Acknowledgement{covidAck()}, covidIncoming())

	if result.Outcome != validation.GatePublish {
		t.Fatalf("an unused acknowledgement must leave an otherwise clean run at plain GatePublish, got %v", result.Outcome)
	}
	unused := findingByRule(result.Findings, "acknowledgement-unused")
	if unused == nil {
		t.Fatalf("expected an acknowledgement-unused finding, got %+v", result.Findings)
	}
	if unused.Severity != validation.SeverityInfo {
		t.Errorf("expected the unused finding to be advisory only, got %q", unused.Severity)
	}
}

// TestGateWithAcknowledgements_AnUnsignedAcknowledgementResolvesNothing is
// the assertion that protects this mechanism's MEANING, as distinct from
// its mechanics.
//
// An acknowledgement's entire authority comes from a named human having put
// their name to it. A record nobody has signed is a PROPOSAL: it may carry
// perfect research, a verified citation and a correctly pinned figure, and
// it still must not move the gate one inch, because the decision
// rule4_revision.go says belongs to a human has not been made. The
// alternative -- letting a well-argued unsigned record publish -- would let
// whoever wrote the argument override the guard, which is precisely the
// thing a sign-off exists to prevent.
//
// This is defence in depth: ingestion.ReconcileEditorialConfig already
// refuses to project an unsigned record into the database at all (the same
// discipline that refuses to project an unconfirmed break date), so the
// gate should never see one. It refuses anyway, because a mechanism whose
// safety depends on one layer never being bypassed is not safe.
func TestGateWithAcknowledgements_AnUnsignedAcknowledgementResolvesNothing(t *testing.T) {
	findings := []validation.Finding{plausibilityFinding("2020-Q2")}
	unsigned := covidAck()
	unsigned.By = "" // nobody signed it

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{unsigned}, covidIncoming())

	if result.Outcome != validation.GateBlock {
		t.Fatalf("an unsigned acknowledgement carries no authority and must resolve nothing, got %v", result.Outcome)
	}
	if len(result.Overridden) != 0 {
		t.Errorf("expected zero overrides for an unsigned record, got %+v", result.Overridden)
	}
	if len(result.UnresolvedFindings()) != 1 {
		t.Errorf("expected the original finding to stand unresolved, got %+v", result.UnresolvedFindings())
	}
}

// TestGateWithAcknowledgements_NeverResolvesANonAcknowledgeableRule:
// defence in depth behind validate-config. Only the rules whose findings
// are genuine human judgement calls (rule 3, rule 4) admit an
// acknowledgement; a schema-drift or decode failure is a broken pipeline,
// and "a human confirmed the data is correct" is not a true statement
// about a header fingerprint mismatch.
func TestGateWithAcknowledgements_NeverResolvesANonAcknowledgeableRule(t *testing.T) {
	findings := []validation.Finding{{
		Rule: "rule1-schema", Severity: validation.SeverityBlock, Period: "2020-Q2",
		Message: "expected field missing",
	}}
	ack := covidAck()
	ack.Rule = "rule1-schema"

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{ack}, covidIncoming())

	if result.Outcome != validation.GateBlock {
		t.Fatalf("rule1-schema must not be acknowledgeable, got %v", result.Outcome)
	}
}

// TestGateWithAcknowledgements_OneUnresolvedFindingStillBlocks: an
// acknowledgement resolves the finding it names and nothing else, so a
// second, un-acknowledged violation in the same run still blocks the whole
// run.
func TestGateWithAcknowledgements_OneUnresolvedFindingStillBlocks(t *testing.T) {
	findings := []validation.Finding{
		plausibilityFinding("2020-Q2"),
		{Rule: "rule2-continuity", Severity: validation.SeverityBlock, Period: "2020-Q3", Message: "missing period"},
	}

	result := validation.GateWithAcknowledgements("ocupados-epa", findings,
		[]validation.Acknowledgement{covidAck()}, covidIncoming())

	if result.Outcome != validation.GateBlock {
		t.Fatalf("an un-acknowledged second violation must still block, got %v", result.Outcome)
	}
	unresolved := result.UnresolvedFindings()
	if len(unresolved) != 1 || unresolved[0].Rule != "rule2-continuity" {
		t.Fatalf("expected exactly the un-acknowledged rule2-continuity finding to remain unresolved, got %+v", unresolved)
	}
}

// TestGate_WithoutAcknowledgementsIsUnchanged pins the compatibility
// promise: every existing caller keeps its exact previous verdict, and an
// ordinary clean run is still plain GatePublish, never the overridden
// outcome.
func TestGate_WithoutAcknowledgementsIsUnchanged(t *testing.T) {
	if got := validation.Gate(nil); got.Outcome != validation.GatePublish || len(got.Overridden) != 0 {
		t.Errorf("expected a clean plain GatePublish, got %+v", got)
	}
	blocked := validation.Gate([]validation.Finding{plausibilityFinding("2020-Q2")})
	if blocked.Outcome != validation.GateBlock {
		t.Errorf("expected GateBlock with no acknowledgements available, got %v", blocked.Outcome)
	}
	if len(blocked.UnresolvedFindings()) != 1 {
		t.Errorf("with no acknowledgements every finding is unresolved, got %+v", blocked.UnresolvedFindings())
	}
}

// TestAcknowledgeableRules_AreExactlyRule3AndRule4 links the allowlist
// config.AcknowledgeableRules publishes to the rule names the REAL rules
// actually emit, rather than comparing two string literals to each other.
// Rule 3 and rule 4 are the only two rules whose blocking findings state
// "this number is surprising and I cannot tell a legitimate cause from a
// broken parser" -- the exact question rule4_revision.go's own comment
// says belongs to a human. Every other rule's finding is a defect in the
// pipeline or in configuration, with its own correct remedy.
func TestAcknowledgeableRules_AreExactlyRule3AndRule4(t *testing.T) {
	max := 40.0
	ctx := validation.SeriesContext{
		Series:     indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
		Validation: config.ValidationConfig{Plausibility: config.PlausibilityConfig{Max: &max}, Revision: config.RevisionConfig{MaxBackwardPeriods: 1}},
		Prior: []indicators.Observation{
			{Period: quarter(2026, 1), Value: ackFloat(10)},
			{Period: quarter(2026, 2), Value: ackFloat(10)},
		},
	}
	incoming := []indicators.Observation{
		{Period: quarter(2026, 1), Value: ackFloat(50)},
		{Period: quarter(2026, 2), Value: ackFloat(10)},
	}

	emitted := map[string]bool{}
	for _, f := range validation.Rule3Plausibility(ctx, incoming) {
		emitted[f.Rule] = true
	}
	for _, f := range validation.Rule4Revision(ctx, incoming) {
		emitted[f.Rule] = true
	}
	if !emitted["rule3-plausibility"] || !emitted["rule4-revision"] {
		t.Fatalf("fixture must make BOTH rule 3 and rule 4 fire, got %v", emitted)
	}

	allowed := map[string]bool{}
	for _, r := range config.AcknowledgeableRules() {
		allowed[r] = true
	}
	for rule := range emitted {
		if !allowed[rule] {
			t.Errorf("rule %q fires a blocking finding a human must adjudicate but is not acknowledgeable", rule)
		}
	}
	if len(allowed) != len(emitted) {
		t.Errorf("the acknowledgeable allowlist must be exactly the rules proven above (%v), got %v", emitted, allowed)
	}
}

func findingByRule(findings []validation.Finding, rule string) *validation.Finding {
	for i := range findings {
		if findings[i].Rule == rule {
			return &findings[i]
		}
	}
	return nil
}
