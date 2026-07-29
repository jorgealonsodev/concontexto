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
	return applyGate(ctx, db, NewObservationWriter(db), ingestionRunID, findings, candidates)
}

// ApplyGateWithWriter is ApplyGate's injectable-writer variant, used only
// by tests that need to observe write CALLS -- an interaction property a
// real database's resulting state cannot distinguish from "wrote then
// rolled back" -- rather than resulting state alone (see
// ObservationWriterPort's doc comment). No production code calls this;
// app/internal/ingestion.IngestSeries always calls ApplyGate, which
// always constructs the real writer.
func ApplyGateWithWriter(ctx context.Context, db DBTX, writer ObservationWriterPort, ingestionRunID int64, findings []validation.Finding, candidates []ObservationInput) (GateApplyResult, error) {
	return applyGate(ctx, db, writer, ingestionRunID, findings, candidates)
}

func applyGate(ctx context.Context, db DBTX, writer ObservationWriterPort, ingestionRunID int64, findings []validation.Finding, candidates []ObservationInput) (GateApplyResult, error) {
	result := GateApplyResult{GateResult: validation.Gate(findings)}

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

	if err := recordRunOutcome(ctx, db, ingestionRunID, RunOutcomeSucceeded); err != nil {
		return result, err
	}
	return result, nil
}
