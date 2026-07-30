package postgres

// Task 4.16 (GREEN, effect half): ApplyGate is the ONLY place that
// decides whether Tx2 (ObservationWriter.WriteRevision, ADR-4) ever
// opens for a run's candidate observations. It delegates the actual
// Publish/Block DECISION entirely to the pure validation.Gate; this
// function's only job is the EFFECT itself:
//   - on Block: touch nothing but the run's own outcome column -- zero
//     observation writes (spec "the current observation is unchanged
//     ... no observation row is written for that run");
//   - on Publish: call the slice-2 writer for every candidate and
//     record a succeeded outcome.
//
// This lives in package postgres, not package validation, specifically
// so the pure package's purity guard (task 4.1: go-list-deps +
// clock/OS source scan) never needs an exception for it -- "the gate
// itself may touch the writer, but the decision function stays pure and
// separate from the effect."

import (
	"context"
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// RunOutcome mirrors ingestion_run.outcome's documented value set
// (migration 0001's comment: 'succeeded'|'validation-failed'|
// 'fetch-failed'|'nothing-new'). ApplyGate only ever writes the first
// two -- 'fetch-failed'/'nothing-new' are recorded upstream of
// validation, before a payload even exists to validate (slice 5a, not
// built here).
type RunOutcome string

const (
	RunOutcomeSucceeded        RunOutcome = "succeeded"
	RunOutcomeValidationFailed RunOutcome = "validation-failed"

	// RunOutcomePending is the placeholder outcome CreateIngestionRun
	// (task 5b.6) inserts a run row with, BEFORE fetching/validation have
	// had a chance to decide the real terminal outcome (design.md
	// "[Tx1] download_attempt + raw_file + ingestion_run(pending)").
	// ApplyGate's recordRunOutcome is the only function allowed to move a
	// row away from this value. A crash between CreateIngestionRun and
	// ApplyGate leaves an honest, queryable "started but never finished"
	// row instead of a fabricated terminal one (principle P7: errors are
	// documented, never erased). There is no CHECK constraint on
	// ingestion_run.outcome, so adding this value needed no migration.
	RunOutcomePending RunOutcome = "pending"

	// RunOutcomeSucceededWithAcknowledgement is a run that published ONLY
	// because a human acknowledgement resolved a finding that would
	// otherwise have blocked it (spec data-validation, "An acknowledged
	// publish is distinguishable from a clean one").
	//
	// It is a distinct persisted value, not a flag alongside 'succeeded',
	// because the requirement is that the RECORDED OUTCOME distinguish the
	// two: querying ingestion_run must be enough to tell "this series
	// validated" from "a human overrode a guard so this series could
	// proceed" -- months later, with no log retention and nobody's memory
	// involved. Like 'pending' it needed no migration; ingestion_run.outcome
	// carries no CHECK constraint.
	//
	// It IS a success for every purpose that asks "did the pipeline produce
	// a datum": SeriesValidationOutcome counts it alongside 'succeeded'
	// when resolving PRD §6.1.3's "last correct update" banner, because the
	// run did write an observation a named human vouched for. What it must
	// never do is disappear into 'succeeded'.
	RunOutcomeSucceededWithAcknowledgement RunOutcome = "succeeded-with-acknowledgement"
)

// recordRunOutcome updates an already-existing ingestion_run row's
// terminal outcome. It never touches raw_file_hash: that column was
// already set when the row was created, before validation ran, so a
// blocked run's audit trail (spec "Publish gate", "recorded ...
// together with its raw file hash") is simply the unchanged row, now
// carrying the failed outcome.
func recordRunOutcome(ctx context.Context, db DBTX, ingestionRunID int64, outcome RunOutcome) error {
	tag, err := db.Exec(ctx, `UPDATE ingestion_run SET outcome=$1, finished_at=now() WHERE id=$2`,
		string(outcome), ingestionRunID)
	if err != nil {
		return fmt.Errorf("postgres: recording run %d outcome %q: %w", ingestionRunID, outcome, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: recording run %d outcome %q: no such ingestion_run", ingestionRunID, outcome)
	}
	return nil
}

// GateApplyResult is ApplyGate's full result: the pure verdict
// (embedded validation.GateResult) plus whichever observations were
// actually published -- empty on Block, by construction, since the loop
// below never runs in that branch.
type GateApplyResult struct {
	validation.GateResult
	Published []Observation
}

// ObservationWriterPort is the narrow write boundary ApplyGate depends
// on. Extracted in slice 8b (tasks 8.10/8.11, malformed-XLSX suite) so a
// test-only spy can substitute for *ObservationWriter and prove "the
// repository receives zero write calls" as a genuine INTERACTION
// property -- something a real database's post-hoc state can never
// prove on its own, because a write followed by a rollback also leaves
// state unchanged. *ObservationWriter (this package's only production
// implementation) is asserted to satisfy it below; nothing about
// ApplyGate's own exported signature changes, so every existing caller
// (app/internal/ingestion.IngestSeries, this package's own tests) is
// unaffected.
type ObservationWriterPort interface {
	WriteRevision(ctx context.Context, in ObservationInput) (Observation, bool, error)
}

var _ ObservationWriterPort = (*ObservationWriter)(nil)

// ApplyGate is documented at length above and in spec data-validation's
// "Publish gate" requirement (scenarios "A failed run leaves the
// published datum untouched" / "All rules passing publishes"). db must
// support Begin (TxBeginner) because the Publish path opens
// ObservationWriter's own atomic promotion transaction (ADR-4); the
// Block path only ever issues one UPDATE. Production code always goes
// through this function, which always writes through the real
// *ObservationWriter -- see ApplyGateWithWriter for the test-only
// injectable-writer variant.
func ApplyGate(ctx context.Context, db TxBeginner, ingestionRunID int64, findings []validation.Finding, candidates []ObservationInput) (GateApplyResult, error) {
	return applyGate(ctx, db, NewObservationWriter(db), ingestionRunID, validation.Gate(findings), candidates)
}

// ApplyGateVerdict is ApplyGate for a caller that has ALREADY made the
// pure decision -- specifically ingestion.IngestSeries, which calls
// validation.GateWithAcknowledgements because resolving the editorial
// acknowledgement registry needs the series id, the run's candidate
// observations and the rows read out of validation_acknowledgement, none
// of which this adapter has any business knowing about.
//
// This keeps the split this file's doc comment describes intact rather
// than eroding it: the DECISION stays pure and in the application layer,
// and this function remains only the EFFECT. ApplyGate above is now simply
// the no-acknowledgements case of it, so every existing call site keeps
// its exact previous behaviour.
func ApplyGateVerdict(ctx context.Context, db TxBeginner, ingestionRunID int64, verdict validation.GateResult, candidates []ObservationInput) (GateApplyResult, error) {
	return applyGate(ctx, db, NewObservationWriter(db), ingestionRunID, verdict, candidates)
}

// ApplyGateWithWriter is ApplyGate's injectable-writer variant, used only
// by tests that need to observe write CALLS -- an interaction property a
// real database's resulting state cannot distinguish from "wrote then
// rolled back" -- rather than resulting state alone (see
// ObservationWriterPort's doc comment). No production code calls this;
// app/internal/ingestion.IngestSeries always calls ApplyGate, which
// always constructs the real writer.
func ApplyGateWithWriter(ctx context.Context, db DBTX, writer ObservationWriterPort, ingestionRunID int64, findings []validation.Finding, candidates []ObservationInput) (GateApplyResult, error) {
	return applyGate(ctx, db, writer, ingestionRunID, validation.Gate(findings), candidates)
}

func applyGate(ctx context.Context, db DBTX, writer ObservationWriterPort, ingestionRunID int64, verdict validation.GateResult, candidates []ObservationInput) (GateApplyResult, error) {
	result := GateApplyResult{GateResult: verdict}

	if result.Outcome == validation.GateBlock {
		if err := recordRunOutcome(ctx, db, ingestionRunID, RunOutcomeValidationFailed); err != nil {
			return result, err
		}
		return result, nil
	}

	for _, candidate := range candidates {
		obs, _, err := writer.WriteRevision(ctx, candidate)
		if err != nil {
			return result, fmt.Errorf("postgres: applying gate for run %d: %w", ingestionRunID, err)
		}
		result.Published = append(result.Published, obs)
	}

	// An acknowledged publish writes exactly the same observations a clean
	// one does -- the datum is the datum -- and differs only in what the
	// run RECORDS about how it got there. Keeping the difference in the
	// outcome column, rather than in what is written, is what lets the two
	// be told apart forever without changing a single published number.
	outcome := RunOutcomeSucceeded
	if result.Outcome == validation.GatePublishOverridden {
		outcome = RunOutcomeSucceededWithAcknowledgement
	}
	if err := recordRunOutcome(ctx, db, ingestionRunID, outcome); err != nil {
		return result, err
	}
	return result, nil
}
