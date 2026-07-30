package validation

import (
	"sort"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// Task 4.16 (GREEN, pure half): the publish gate's DECISION. Spec
// data-validation, "Publish gate": validation failure MUST NOT publish;
// all rules passing publishes. Gate is deliberately just this: a pure
// reduction over the findings every rule already produced, with NO
// knowledge of how -- or whether -- to actually write anything. The
// EFFECT (calling the slice-2 ObservationWriter, recording the run's
// terminal outcome) is postgres.ApplyGate's job, kept in the postgres
// adapter package specifically so this package's purity guard (task
// 4.1) never has to make an exception for it: "the gate itself may
// touch the writer, but the decision function stays pure and separate
// from the effect."
//
// Gate never truncates to the first blocking finding (spec "The gate
// reports every failure, not only the first"): GateResult.Findings is
// exactly the findings slice it was given, in full -- the caller (who
// runs every applicable rule and concatenates their output before
// calling Gate) is what makes "reports every failure" true; Gate's only
// added value is the Publish/Block verdict.

// GateOutcome is the publish gate's verdict.
type GateOutcome string

const (
	GatePublish GateOutcome = "publish"
	GateBlock   GateOutcome = "block"

	// GatePublishOverridden is a publish that happened ONLY because a
	// human acknowledgement resolved a finding that would otherwise have
	// blocked it (spec data-validation, "An acknowledged publish is
	// distinguishable from a clean one"). It exists as a third outcome,
	// rather than collapsing into GatePublish, for one reason: an operator
	// reading the pipeline log must never confuse "this run validated"
	// with "a human overrode a guard so this run could proceed". Those are
	// different claims about the data, and only one of them is a clean
	// bill of health. Every downstream check that asks "did this run
	// block?" (postgres.ApplyGate, ingestion.logAndAlertRun,
	// the ingest command's export gate) tests against GateBlock, so a
	// third publishing outcome changes none of their behaviour.
	GatePublishOverridden GateOutcome = "publish-overridden"
)

// GateResult is Gate's full verdict: the outcome plus every finding fed
// into it, so a caller can report an operator-facing audit trail (spec
// "the recorded outcome lists both violations").
type GateResult struct {
	Outcome GateOutcome

	// Findings is every finding, verbatim and complete -- including any an
	// acknowledgement resolved, at their ORIGINAL severity. An
	// acknowledgement changes what the gate DECIDES, never what it
	// REPORTS: spec "The gate reports every failure, not only the first"
	// is not weakened by one, and an operator must still be able to read
	// the magnitude that was acknowledged out of the log line.
	Findings []Finding

	// Overridden lists each finding an acknowledgement resolved, with the
	// record and the human that resolved it. Empty on every run with no
	// acknowledgements in play, which is every run this codebase made
	// before the registry existed.
	Overridden []Override
}

// UnresolvedFindings is every finding an acknowledgement did NOT resolve
// -- the set that actually decided this run's outcome. Callers reporting
// "which rules failed" must use this rather than Findings, so an
// acknowledged finding is never reported as a failure of a run that
// published.
func (r GateResult) UnresolvedFindings() []Finding {
	if len(r.Overridden) == 0 {
		return r.Findings
	}
	overridden := make(map[string]bool, len(r.Overridden))
	for _, o := range r.Overridden {
		overridden[o.Finding.Rule+"\x00"+o.Finding.Period+"\x00"+o.Finding.Message] = true
	}
	out := make([]Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if overridden[f.Rule+"\x00"+f.Period+"\x00"+f.Message] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// blocks reports whether f's severity is one the gate treats as
// blocking. SeverityInfo is advisory only and never blocks publication.
func (f Finding) blocks() bool {
	return f.Severity == SeverityBlock || f.Severity == SeverityBlockRequiresSignoff
}

// Gate is documented at length above and in the package's method table
// (spec "A failed run leaves the published datum untouched" / "All
// rules passing publishes" / "The gate reports every failure, not only
// the first").
//
// It is now the no-acknowledgements case of GateWithAcknowledgements,
// which keeps every pre-existing call site (and its exact verdict)
// unchanged: with no acknowledgements there is nothing to resolve, so the
// outcome can only ever be GatePublish or GateBlock, precisely as before.
func Gate(findings []Finding) GateResult {
	return GateWithAcknowledgements("", findings, nil, nil)
}

// GateWithAcknowledgements is the publish gate with the editorial
// acknowledgement registry in play (spec data-validation, "Acknowledged
// findings"). seriesID scopes the acknowledgements -- one is only ever
// consulted for the series it names -- and incoming is this run's
// candidate observations, which the staleness check reads the pinned
// value back out of.
//
// The order of operations is load-bearing:
//
//  1. Acknowledgements are matched against findings and partitioned into
//     resolved, stale and unused (acknowledgement.go).
//  2. Findings gains the stale/unused findings, so they travel the same
//     audited path every rule's output travels rather than being logged
//     out of band.
//  3. The verdict is a reduction over the UNRESOLVED blocking findings
//     only -- including any acknowledgement-stale finding step 1 raised,
//     which is what makes a stale record fail closed.
//
// Nothing here is rule-specific: the gate matches on (series, period,
// rule) and never branches on which rule a finding came from, so
// SeverityBlock and SeverityBlockRequiresSignoff resolve identically and
// rule 4's documented intent finally has its other half. WHICH rules admit
// an acknowledgement at all is a separate, deliberately closed policy
// question owned by config.AcknowledgeableRules.
func GateWithAcknowledgements(seriesID string, findings []Finding, acks []Acknowledgement, incoming []indicators.Observation) GateResult {
	resolved, extra := applyAcknowledgements(seriesID, findings, acks, incoming)

	all := findings
	if len(extra) > 0 {
		all = make([]Finding, 0, len(findings)+len(extra))
		all = append(all, findings...)
		all = append(all, extra...)
	}

	result := GateResult{Outcome: GatePublish, Findings: all}
	for _, o := range resolved {
		result.Overridden = append(result.Overridden, o)
	}
	sortOverrides(result.Overridden)

	for i, f := range all {
		if !f.blocks() {
			continue
		}
		if _, isResolved := resolved[i]; isResolved {
			continue
		}
		result.Outcome = GateBlock
		return result
	}

	if len(result.Overridden) > 0 {
		result.Outcome = GatePublishOverridden
	}
	return result
}

// sortOverrides gives Overridden a deterministic order (map iteration is
// not), so a run's recorded outcome and its log line are reproducible --
// the same reason ingestion.sixRules runs in a fixed order.
func sortOverrides(overrides []Override) {
	sort.SliceStable(overrides, func(i, j int) bool {
		if overrides[i].Finding.Period != overrides[j].Finding.Period {
			return overrides[i].Finding.Period < overrides[j].Finding.Period
		}
		return overrides[i].Finding.Rule < overrides[j].Finding.Rule
	})
}
