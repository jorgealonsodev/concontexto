package postgres_test

// Task 2.16 (RED) / 2.17 (GREEN): rolling back a bad ingestion run
// re-points is_current to the prior version, records a reason on the
// demoted row, and never deletes the observation or touches the raw
// file / its hash (spec data-model-vintages, "Bad-run rollback never
// deletes", principle P7).

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestObservationWriter_RollbackRestoresThePriorCurrentVersion(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-good", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision (v1, good): %v", err)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-bad", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	bad, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(999.9),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	})
	if err != nil {
		t.Fatalf("WriteRevision (v2, bad): %v", err)
	}
	if bad.Version != 2 || !bad.IsCurrent {
		t.Fatalf("expected the bad revision to be current version 2, got version=%d current=%v", bad.Version, bad.IsCurrent)
	}

	reason := "source served corrupted values for this run"
	restored, err := writer.RollbackRun(ctx, run2, reason)
	if err != nil {
		t.Fatalf("RollbackRun: %v", err)
	}
	if len(restored) != 1 {
		t.Fatalf("expected exactly one restored observation, got %d", len(restored))
	}
	if restored[0].Version != 1 || !restored[0].IsCurrent {
		t.Errorf("expected version 1 to be restored as current, got version=%d current=%v", restored[0].Version, restored[0].IsCurrent)
	}
	if restored[0].Value == nil || *restored[0].Value != 11.5 {
		t.Errorf("expected the restored version's value to be unchanged (11.5), got %v", restored[0].Value)
	}
	if restored[0].SupersededAt != nil {
		t.Error("expected the restored current version to have superseded_at cleared")
	}

	// The rolled-back version must still exist, unchanged in value, no
	// longer current, and carrying the rollback reason.
	var v2Value float64
	var v2Current bool
	var v2Reason *string
	row := tx.QueryRow(ctx, `SELECT value, is_current, rollback_reason FROM observation
		WHERE series_id=$1 AND period=$2 AND version=2`, "tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&v2Value, &v2Current, &v2Reason); err != nil {
		t.Fatalf("reading rolled-back v2: %v", err)
	}
	if v2Value != 999.9 {
		t.Errorf("expected the rolled-back version to keep its original value 999.9, got %v", v2Value)
	}
	if v2Current {
		t.Error("expected the rolled-back version to no longer be current")
	}
	if v2Reason == nil || *v2Reason != reason {
		t.Errorf("expected the rollback reason to be recorded on v2, got %v", v2Reason)
	}

	// The raw file and its hash must be untouched.
	var rawFileCount int
	row = tx.QueryRow(ctx, `SELECT count(*) FROM raw_file WHERE hash='hash-bad'`)
	if err := row.Scan(&rawFileCount); err != nil {
		t.Fatalf("counting raw_file: %v", err)
	}
	if rawFileCount != 1 {
		t.Errorf("expected the raw file for the rolled-back run to remain unchanged, got count=%d", rawFileCount)
	}

	// Every observation row must still exist — rollback never deletes.
	var totalRows int
	row = tx.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id=$1 AND period=$2`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&totalRows); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if totalRows != 2 {
		t.Errorf("expected both versions to still exist after rollback, got %d rows", totalRows)
	}
}
