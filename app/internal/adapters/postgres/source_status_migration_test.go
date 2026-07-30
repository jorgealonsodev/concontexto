package postgres_test

// Task 2a.3 (RED): migration 0003_observation_source_status is additive
// and reversible (spec data-model-vintages, "The source_status migration
// is additive and reversible"). Mirrors migration_test.go's own
// introspection style (TestMigrationUp_EventTableUsesEventGroupNotReservedWord)
// for column-existence assertions.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestMigrationUp_SourceStatusColumnIsAdditiveNullableAndPreservesExistingRows(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)

	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	// Seed a row exactly like a Fase 0 deployment would have written
	// BEFORE this migration existed: no source_status value supplied.
	mustExec(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://ine.es', 'lic', 'attr', 'api-json', 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ('ine-epa', 'ine', 'EPA', 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-epa', 'Tasa de paro', '%', 'Q', 'ES', 2, false, 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ('deadbeef', 'ine', 'https://ine.es/data', now(), '/app_data/deadbeef', 100)`)
	mustExec(t, ctx, tx, `INSERT INTO ingestion_run (dataset_id, series_id, started_at, finished_at, raw_file_hash, outcome)
		VALUES ('ine-epa', 'tasa-de-paro-epa', now(), now(), 'deadbeef', 'succeeded')`)
	mustExec(t, ctx, tx, `INSERT INTO observation (series_id, period, version, value, status, extracted_at, ingestion_run_id, is_current)
		VALUES ('tasa-de-paro-epa', '2026-Q1', 1, 11.5, 'D', now(), 1, true)`)

	var value float64
	var status string
	var sourceStatus *string
	row := tx.QueryRow(ctx, `SELECT value, status, source_status FROM observation WHERE series_id=$1 AND period=$2`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&value, &status, &sourceStatus); err != nil {
		t.Fatalf("reading the pre-existing row after migration: %v", err)
	}
	if value != 11.5 || status != "D" {
		t.Errorf("expected the pre-existing row's value/status to survive unchanged, got value=%v status=%q", value, status)
	}
	if sourceStatus != nil {
		t.Errorf("expected source_status to be NULL on a row written before this migration, got %v", *sourceStatus)
	}
}

func TestMigrationDown_SourceStatusColumnDropsWithoutLosingAnyObservation(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)

	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	provisional := "Provisional"
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusProvisional, SourceStatus: &provisional,
		ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision: %v", err)
	}

	// Down rolls back exactly one step at a time (runner.go's own
	// contract). Two later migrations now sit on top of 0003 --
	// 0005_series_discontinued (Remediation B) and 0004_source_licence_url
	// (slice 4), neither touching this test's own column -- so the third
	// Down is the one that reverts 0003, the migration this test actually
	// asserts against.
	if err := runner.Down(ctx); err != nil {
		t.Fatalf("Down (0005): %v", err)
	}
	if err := runner.Down(ctx); err != nil {
		t.Fatalf("Down (0004): %v", err)
	}
	if err := runner.Down(ctx); err != nil {
		t.Fatalf("Down (0003): %v", err)
	}

	// The column itself must be gone (a real DROP, not merely nulled).
	var columnExists bool
	row := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema='public' AND table_name='observation' AND column_name='source_status')`)
	if err := row.Scan(&columnExists); err != nil {
		t.Fatalf("checking source_status column existence: %v", err)
	}
	if columnExists {
		t.Error("expected source_status column to be dropped by Down, it still exists")
	}

	// Every observation row -- value, status, version -- must still exist.
	var value float64
	var status string
	var version int
	row = tx.QueryRow(ctx, `SELECT value, status, version FROM observation WHERE series_id=$1 AND period=$2`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&value, &status, &version); err != nil {
		t.Fatalf("expected the observation row to survive Down, got: %v", err)
	}
	if value != 11.5 || status != "P" || version != 1 {
		t.Errorf("expected value=11.5 status=P version=1 to survive Down, got value=%v status=%q version=%d", value, status, version)
	}
}
