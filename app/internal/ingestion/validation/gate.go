package validation

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
)

// GateResult is Gate's full verdict: the outcome plus every finding fed
// into it, so a caller can report an operator-facing audit trail (spec
// "the recorded outcome lists both violations").
type GateResult struct {
	Outcome  GateOutcome
	Findings []Finding
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
func Gate(findings []Finding) GateResult {
	result := GateResult{Outcome: GatePublish, Findings: findings}
	for _, f := range findings {
		if f.blocks() {
			result.Outcome = GateBlock
			break
		}
	}
	return result
}
