package postgres_test

// Task 4.15 (RED, effect half): ApplyGate wires the pure
// validation.Gate decision to the slice-2 writer (PR 2b's
// ObservationWriter) against a REAL postgres:17-alpine container --
// exactly the same testcontainers harness PR 2a/2b already established
// (TestMain, newTx, seedSeries/seedIngestionRun from
// observation_writer_test.go, reused unmodified from this same test
// package).
//
// Proves, for real:
//   - "A failed run leaves the published datum untouched": a Block
//     verdict performs ZERO observation writes and the current row
//     stays byte-identical to what it was before the run.
//   - The failed run IS still recorded, with a failed outcome and its
//     original raw_file_hash intact (spec "the run is recorded with a
//     failed outcome and its raw file hash").
//   - "All rules passing publishes": a Publish verdict calls the writer
//     for real and the new value becomes current.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func TestApplyGate_BlockLeavesCurrentObservationUntouchedAndRecordsFailedOutcome(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(0.6),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("seeding the already-published observation: %v", err)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	findings := []validation.Finding{
		{Rule: "rule3-plausibility", Severity: validation.SeverityBlock, Message: "out of range"},
	}
	candidates := []postgres.ObservationInput{{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(99.9),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	}}

	result, err := postgres.ApplyGate(ctx, tx, run2, findings, candidates)
	if err != nil {
		t.Fatalf("ApplyGate: %v", err)
	}
	if result.Outcome != validation.GateBlock {
		t.Fatalf("expected GateBlock, got %v", result.Outcome)
	}
	if len(result.Published) != 0 {
		t.Fatalf("expected zero published observations on a blocked run, got %d", len(result.Published))
	}

	var count int
	row := tx.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id=$1 AND period=$2`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly the one pre-existing observation row (zero writes), got %d rows", count)
	}

	var value float64
	var isCurrent bool
	row = tx.QueryRow(ctx, `SELECT value, is_current FROM observation WHERE series_id=$1 AND period=$2 AND version=1`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&value, &isCurrent); err != nil {
		t.Fatalf("reading the observation after a blocked run: %v", err)
	}
	if value != 0.6 || !isCurrent {
		t.Fatalf("expected the original published observation (0.6, current) to be untouched, got value=%v current=%v", value, isCurrent)
	}

	var outcome, rawFileHash string
	row = tx.QueryRow(ctx, `SELECT outcome, raw_file_hash FROM ingestion_run WHERE id=$1`, run2)
	if err := row.Scan(&outcome, &rawFileHash); err != nil {
		t.Fatalf("reading run2's recorded outcome: %v", err)
	}
	if outcome != string(postgres.RunOutcomeValidationFailed) {
		t.Errorf("expected run2's outcome to be recorded as %q, got %q", postgres.RunOutcomeValidationFailed, outcome)
	}
	if rawFileHash != "hash-run2" {
		t.Errorf("expected run2's raw_file_hash to remain hash-run2 (the run's original audit trail), got %q", rawFileHash)
	}
}

func TestApplyGate_AllPassPublishes(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	candidates := []postgres.ObservationInput{{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(0.65),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run,
	}}

	result, err := postgres.ApplyGate(ctx, tx, run, nil, candidates)
	if err != nil {
		t.Fatalf("ApplyGate: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected GatePublish for an all-pass run, got %v", result.Outcome)
	}
	if len(result.Published) != 1 || result.Published[0].Value == nil || *result.Published[0].Value != 0.65 {
		t.Fatalf("expected the candidate observation to be published, got %+v", result.Published)
	}

	var value float64
	var isCurrent bool
	row := tx.QueryRow(ctx, `SELECT value, is_current FROM observation WHERE series_id=$1 AND period=$2 AND version=1`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&value, &isCurrent); err != nil {
		t.Fatalf("reading the published observation: %v", err)
	}
	if value != 0.65 || !isCurrent {
		t.Fatalf("expected the new observation to be current with value 0.65, got value=%v current=%v", value, isCurrent)
	}

	var outcome string
	row = tx.QueryRow(ctx, `SELECT outcome FROM ingestion_run WHERE id=$1`, run)
	if err := row.Scan(&outcome); err != nil {
		t.Fatalf("reading run's recorded outcome: %v", err)
	}
	if outcome != string(postgres.RunOutcomeSucceeded) {
		t.Errorf("expected run's outcome to be recorded as %q, got %q", postgres.RunOutcomeSucceeded, outcome)
	}
}
