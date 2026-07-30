package validation

// The acknowledgement registry's PURE half (spec data-validation,
// "Acknowledged findings"). This file is what finally gives
// SeverityBlockRequiresSignoff its other half: rule4_revision.go has said
// since PR 4b that the severity "hands the decision to a human rather than
// guessing either way", while gate.go treated it exactly like
// SeverityBlock and nothing anywhere resolved it, so every blocked series
// stayed blocked forever.
//
// Everything here is a pure reduction over data the caller already
// resolved -- no I/O, no clock -- so the package's purity guard
// (purity_test.go) needs no exception. Resolving the acknowledgements
// themselves out of the database is postgres.ResolveActiveAcknowledgements
// ForSeries's job, exactly as ctx.Breaks is pre-resolved for rule 3.
//
// WHY THE PINNED VALUE IS THE STALENESS GUARD. An acknowledgement is a
// human sentence: "I looked at ocupados-epa 2020-Q2 = 18607.2 and confirm
// the fall is real." The dangerous failure mode is that sentence quietly
// outliving the number it is about -- a later run revises 2020-Q2, and an
// approval nobody re-examined silently covers a datum nobody reviewed.
// Three candidates were considered for what to pin:
//
//   - An EXPIRY DATE. Rejected: the calendar is unrelated to whether the
//     datum changed. It re-blocks a correct, unchanged value the day it
//     lapses, and keeps covering a revised value until then -- wrong in
//     both directions at once.
//   - A HASH OF THE FINDING'S MESSAGE. It does close the delta case (a
//     rule 3 message embeds the magnitude), but it welds an editorial
//     record to a Go format string: rewording any message would silently
//     invalidate every acknowledgement in the repository, and no reviewer
//     could author or check the pin by hand.
//   - THE OBSERVED VALUE AT THE ACKNOWLEDGED PERIOD. Chosen. It is
//     precisely the fact the human reviewed, it is human-readable and
//     reviewable in the same four-eyes PR that adds it, and any change to
//     it at all -- exact equality, no tolerance -- fails closed.
//
// Exact float equality is deliberate and safe: both sides are the same
// deterministic decimal-literal parse (the source's own JSON number and
// the YAML pin), so an unrevised datum compares equal bit for bit, and any
// difference whatsoever IS the revision this guard exists to catch. A
// tolerance here would be a licence for small silent rewrites.
//
// AND IT IS NEVER SILENT. A stale acknowledgement does not merely fail to
// apply -- it raises its own blocking acknowledgement-stale finding naming
// the record, the pinned value and the current one. Otherwise an operator
// would see the original finding reappear months later with no indication
// that a human approval had gone stale underneath it.
//
// DOCUMENTED RESIDUAL. Pinning the acknowledged period's own value does
// NOT cover the case where a NEIGHBOURING period is revised, changing a
// delta-based finding's magnitude while the acknowledged period's value is
// untouched. Closing that would require pinning each rule's private input
// set, which would make this registry rule-specific -- the one property
// the task's constraints rule out. The mitigation is disclosure, not
// silence: the acknowledged finding is still reported verbatim in
// GateResult.Findings (with its recomputed magnitude) and printed in every
// run's structured log, so the change is visible in the log line rather
// than hidden by the override.

import (
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// Rule names for the two findings this file raises itself. They are
// findings, not log lines, so they travel the same audited path
// (GateResult.Findings -> pipelinelog verdicts -> the operator) every
// rule's output already travels.
const (
	// RuleAcknowledgementStale marks an acknowledgement whose pinned value
	// no longer matches the datum in front of the gate. It BLOCKS: the
	// human's confirmation is no longer about this number.
	RuleAcknowledgementStale = "acknowledgement-stale"

	// RuleAcknowledgementUnused marks an acknowledgement that resolved no
	// finding this run. Advisory only -- the underlying condition going
	// away is the good outcome, and blocking on it would punish exactly
	// that -- but reported, so the registry cannot silently accumulate
	// approvals nobody can account for.
	RuleAcknowledgementUnused = "acknowledgement-unused"
)

// Acknowledgement is one editorial record that a named human reviewed one
// specific blocking finding and confirmed the underlying datum.
//
// SeriesID, Period and Rule are the scope, matched by EXACT equality on
// all three. There is no wildcard, no range and no "all rules" form
// anywhere in this type or in the YAML that feeds it: an acknowledgement
// is scoped exactly as narrowly as the finding it resolves, because a
// mechanism able to blanket-disable a guard is worse than the gap it
// fills.
//
// Value is the pinned observed value; see this file's doc comment for why
// it, and not an expiry date, is what makes an acknowledgement unable to
// outlive the thing it acknowledged.
//
// By is the human who SIGNED the record, and it is the field that carries
// the record's authority — see signed() below.
type Acknowledgement struct {
	ID        string
	SeriesID  string
	Period    string
	Rule      string
	Value     float64
	By        string
	Note      string
	SourceURL string
}

// signed reports whether a named human has put their name to this record.
//
// An acknowledgement's authority does not come from its argument being
// good; it comes from a person having accepted responsibility for it.
// rule4_revision.go's whole stated purpose is to hand a decision to a
// HUMAN rather than let the machine guess, so a record nobody signed is a
// proposal — however thorough its research, however solid its citation —
// and it must move nothing.
//
// This is defence in depth. ingestion.ReconcileEditorialConfig already
// refuses to project an unsigned record into the database at all (the same
// discipline that refuses to project an unconfirmed break date), so in
// practice the gate never sees one. It checks anyway, because a mechanism
// whose safety depends on one layer never being bypassed is not safe: the
// database can be hand-edited, a future caller can build these values from
// somewhere else, and the cost of being sure here is one comparison.
func (a Acknowledgement) signed() bool { return a.By != "" }

// covers reports whether a is scoped to exactly the finding f of series
// seriesID. All three comparisons are exact string equality -- never a
// prefix, a glob or a range -- so "2020-Q2" can never resolve "2020-Q3"
// and a rule3-plausibility record can never resolve a rule4-revision
// finding.
func (a Acknowledgement) covers(seriesID string, f Finding) bool {
	return a.SeriesID == seriesID && a.Period == f.Period && a.Rule == f.Rule
}

// Override is one resolved finding together with the acknowledgement that
// resolved it. It is what makes an overridden publish auditable: the
// recorded outcome carries WHICH finding was overridden, by WHICH record,
// approved by WHOM -- never a bare "published".
type Override struct {
	Finding           Finding
	AcknowledgementID string
	AcknowledgedBy    string
}

// applyAcknowledgements is Gate's acknowledgement pass: it partitions
// findings into those an acknowledgement resolves and those it does not,
// and appends its own stale/unused findings. Pure; called only by
// GateWithAcknowledgements.
func applyAcknowledgements(seriesID string, findings []Finding, acks []Acknowledgement, incoming []indicators.Observation) (resolved map[int]Override, extra []Finding) {
	resolved = map[int]Override{}
	used := make([]bool, len(acks))

	for i, f := range findings {
		if !f.blocks() {
			continue // an advisory finding never needed resolving
		}
		for j, a := range acks {
			if !a.covers(seriesID, f) {
				continue
			}
			// An unsigned record carries no authority at all: it is not
			// stale, not unused, not consulted. It simply is not an
			// acknowledgement yet. See Acknowledgement.signed().
			if !a.signed() {
				continue
			}
			// Defence in depth behind validate-config's allowlist: a row
			// that somehow reached the database naming a rule that is not a
			// human judgement call resolves nothing, silently or otherwise.
			if !config.IsAcknowledgeableRule(a.Rule) {
				continue
			}
			used[j] = true

			current, present := valueAt(incoming, a.Period)
			if !present || current != a.Value {
				extra = append(extra, staleFinding(a, current, present))
				break // stale: f stays blocking, and the operator is told why
			}
			resolved[i] = Override{Finding: f, AcknowledgementID: a.ID, AcknowledgedBy: a.By}
			break
		}
	}

	for j, a := range acks {
		if used[j] || a.SeriesID != seriesID {
			continue
		}
		// An unsigned record is deliberately NOT reported as unused. "Ready
		// to retire" is the wrong advice for a draft that is waiting on a
		// reviewer, and the right place to surface it is the reconcile's
		// PendingAcknowledgementIDs, which says what is actually true: a
		// human has not signed it yet.
		if !a.signed() {
			continue
		}
		extra = append(extra, Finding{
			Rule: RuleAcknowledgementUnused, Severity: SeverityInfo, Period: a.Period,
			Message: fmt.Sprintf("acknowledgement %q (%s at %s, acknowledged by %s) resolved no finding in this run; the condition it covers no longer occurs, so the record is ready to retire from config/reconocimientos.yaml",
				a.ID, a.Rule, a.Period, a.By),
		})
	}
	return resolved, extra
}

// staleFinding builds the blocking finding raised when an
// acknowledgement's pinned value no longer describes the datum in front of
// the gate. It names the record, both values (or the absence), and the
// remedy, because the whole point of raising it is that an operator must
// never have to guess why a previously-published series started blocking
// again.
func staleFinding(a Acknowledgement, current float64, present bool) Finding {
	if !present {
		return Finding{
			Rule: RuleAcknowledgementStale, Severity: SeverityBlock, Period: a.Period,
			Message: fmt.Sprintf("acknowledgement %q pins %s = %v, but this run delivers no value at %s at all, so the acknowledgement cannot be verified and does not apply; re-review the period and update or retire the record in config/reconocimientos.yaml",
				a.ID, a.Period, a.Value, a.Period),
		}
	}
	return Finding{
		Rule: RuleAcknowledgementStale, Severity: SeverityBlock, Period: a.Period,
		Message: fmt.Sprintf("acknowledgement %q pins %s = %v but this run delivers %v: the datum was revised after it was acknowledged, so the acknowledgement no longer applies; re-review the revised value and update or retire the record in config/reconocimientos.yaml",
			a.ID, a.Period, a.Value, current),
	}
}

// valueAt returns the observed value at the period whose canonical label
// is period, and whether one is present. A nil value (a withdrawal) counts
// as absent: there is no number for a pin to match, and failing closed is
// the correct direction.
func valueAt(incoming []indicators.Observation, period string) (float64, bool) {
	for _, o := range incoming {
		if o.Period.String() == period && o.Value != nil {
			return *o.Value, true
		}
	}
	return 0, false
}
