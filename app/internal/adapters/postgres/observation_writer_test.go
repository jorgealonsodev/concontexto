package postgres_test

// Task 2.6 (RED) / 2.7 (GREEN): observation revisions are immutable per
// version — a changed value inserts version = prior+1 while the prior
// row stays intact; an identical resubmission creates no new row (spec
// data-model-vintages, "Observations are immutable per version").
//
// Task 2.8 (RED) / 2.9 (GREEN): the one-transaction promotion invariant
// — exactly one current row ever exists, and a failed insert after the
// prior row was demoted rolls the whole promotion back (spec
// data-model-vintages, "The database enforces exactly one current row").
//
// Task 2.14 (RED) / 2.15 (GREEN): withdrawal is a tombstone (a new
// version with status='W', value NULL), never a delete (spec
// data-model-vintages, "Source withdrawal is representable").

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// seedSeries inserts the minimal source/dataset/series chain every
// observation test needs, reusing migration_test.go's fixture shape.
func seedSeries(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	mustExec(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://ine.es', 'lic', 'attr', 'api-json', 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ('ine-epa', 'ine', 'EPA', 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-epa', 'Tasa de paro', '%', 'Q', 'ES', 2, false, 'digest1')`)
}

// seedIngestionRun records one raw_file + ingestion_run pair (a
// distinct hash per run, as filestore.Put would produce for each
// distinct download) and returns the new ingestion_run.id.
func seedIngestionRun(t *testing.T, ctx context.Context, tx pgx.Tx, hash string, startedAt time.Time) int64 {
	t.Helper()
	mustExec(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ($1, 'ine', 'https://ine.es/data', $2, '/app_data/'||$1, 100)`, hash, startedAt)

	row := tx.QueryRow(ctx, `INSERT INTO ingestion_run (dataset_id, series_id, started_at, finished_at, raw_file_hash, outcome)
		VALUES ('ine-epa', 'tasa-de-paro-epa', $1, $1, $2, 'succeeded') RETURNING id`, startedAt, hash)
	var id int64
	if err := row.Scan(&id); err != nil {
		t.Fatalf("seedIngestionRun: %v", err)
	}
	return id
}

func ptr(v float64) *float64 { return &v }

func TestObservationWriter_RevisionCreatesNewVersionWithoutOverwriting(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)

	first, created, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID:       "tasa-de-paro-epa",
		Period:         "2026-Q1",
		Value:          ptr(0.6),
		Status:         postgres.StatusDefinitive,
		ExtractedAt:    time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run1,
	})
	if err != nil {
		t.Fatalf("WriteRevision (v1): %v", err)
	}
	if !created || first.Version != 1 {
		t.Fatalf("expected v1 to be created as version 1, got created=%v version=%d", created, first.Version)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	second, created, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID:       "tasa-de-paro-epa",
		Period:         "2026-Q1",
		Value:          ptr(0.7),
		Status:         postgres.StatusDefinitive,
		ExtractedAt:    time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	})
	if err != nil {
		t.Fatalf("WriteRevision (v2): %v", err)
	}
	if !created || second.Version != 2 {
		t.Fatalf("expected v2 to be created as version 2, got created=%v version=%d", created, second.Version)
	}
	if !second.IsCurrent {
		t.Error("expected v2 to be current")
	}
	if second.Value == nil || *second.Value != 0.7 {
		t.Errorf("expected current value 0.7, got %v", second.Value)
	}

	// The prior row must still exist, unmodified, and no longer current.
	var v1Value float64
	var v1Current bool
	row := tx.QueryRow(ctx, `SELECT value, is_current FROM observation WHERE series_id=$1 AND period=$2 AND version=1`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&v1Value, &v1Current); err != nil {
		t.Fatalf("reading v1 after v2 was written: %v", err)
	}
	if v1Value != 0.6 {
		t.Errorf("expected v1 to still hold 0.6, got %v", v1Value)
	}
	if v1Current {
		t.Error("expected v1 to no longer be current")
	}
}

func TestObservationWriter_UnchangedValueDoesNotCreateNewVersion(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)

	in := postgres.ObservationInput{
		SeriesID:       "tasa-de-paro-epa",
		Period:         "2026-Q1",
		Value:          ptr(0.6),
		Status:         postgres.StatusDefinitive,
		ExtractedAt:    time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run1,
	}
	if _, created, err := writer.WriteRevision(ctx, in); err != nil || !created {
		t.Fatalf("WriteRevision (v1): created=%v err=%v", created, err)
	}

	// A later run reports the identical value and status.
	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	in.IngestionRunID = run2
	in.ExtractedAt = time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)

	_, created, err := writer.WriteRevision(ctx, in)
	if err != nil {
		t.Fatalf("WriteRevision (resubmission): %v", err)
	}
	if created {
		t.Error("expected no new observation row for an identical resubmission")
	}

	var count int
	row := tx.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id=$1 AND period=$2`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 observation row after an identical resubmission, got %d", count)
	}

	// The run itself must still be recorded, even though it wrote no
	// observation row.
	var runCount int
	row = tx.QueryRow(ctx, `SELECT count(*) FROM ingestion_run WHERE id=$1`, run2)
	if err := row.Scan(&runCount); err != nil {
		t.Fatalf("counting ingestion_run rows: %v", err)
	}
	if runCount != 1 {
		t.Errorf("expected the resubmission's ingestion_run to still be recorded, got count=%d", runCount)
	}
}

func TestObservationWriter_PromotionMovesCurrentFlagAtomically(t *testing.T) {
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
		t.Fatalf("WriteRevision (v1): %v", err)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(0.7),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	}); err != nil {
		t.Fatalf("WriteRevision (v2): %v", err)
	}

	var currentVersion int
	var count int
	row := tx.QueryRow(ctx, `SELECT count(*), max(version) FROM observation
		WHERE series_id=$1 AND period=$2 AND is_current`, "tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&count, &currentVersion); err != nil {
		t.Fatalf("reading current row: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one current row, found %d", count)
	}
	if currentVersion != 2 {
		t.Errorf("expected version 2 to be current, got %d", currentVersion)
	}
}

// TestObservationWriter_FailedPromotionLeavesThePriorRowCurrent proves
// the promotion happens in ONE transaction: if the insert half fails
// (here, by violating the CHECK that value may be NULL only when
// status='W'), the demote-half must roll back too, so the database is
// never left with zero current rows even transiently.
func TestObservationWriter_FailedPromotionLeavesThePriorRowCurrent(t *testing.T) {
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
		t.Fatalf("WriteRevision (v1): %v", err)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	_, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: nil, // invalid: NULL value with status != 'W'
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	})
	if err == nil {
		t.Fatal("expected WriteRevision to fail the CHECK constraint (NULL value with status='D')")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected a *pgconn.PgError, got: %v", err)
	}

	var currentVersion int
	var isCurrent bool
	row := tx.QueryRow(ctx, `SELECT version, is_current FROM observation
		WHERE series_id=$1 AND period=$2 AND is_current`, "tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&currentVersion, &isCurrent); err != nil {
		t.Fatalf("expected v1 to still be current after the failed promotion, but found no current row: %v", err)
	}
	if currentVersion != 1 || !isCurrent {
		t.Errorf("expected v1 to remain current after the failed promotion, got version=%d is_current=%v", currentVersion, isCurrent)
	}

	var totalRows int
	row = tx.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id=$1 AND period=$2`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&totalRows); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if totalRows != 1 {
		t.Errorf("expected the failed promotion to leave exactly 1 observation row (no partial insert), got %d", totalRows)
	}
}

func TestObservationWriter_WithdrawalIsTombstonedNotDeleted(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision (v1): %v", err)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	withdrawal, created, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: nil,
		Status: postgres.StatusWithdrawn, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	})
	if err != nil {
		t.Fatalf("WriteRevision (withdrawal): %v", err)
	}
	if !created || withdrawal.Version != 2 {
		t.Fatalf("expected the withdrawal to create version 2, got created=%v version=%d", created, withdrawal.Version)
	}
	if withdrawal.Status != postgres.StatusWithdrawn || withdrawal.Value != nil {
		t.Errorf("expected the withdrawal row to have status=W and value=NULL, got status=%s value=%v", withdrawal.Status, withdrawal.Value)
	}
	if !withdrawal.IsCurrent {
		t.Error("expected the withdrawal to become the current version")
	}

	var v1Value float64
	var v1Status string
	row := tx.QueryRow(ctx, `SELECT value, status FROM observation WHERE series_id=$1 AND period=$2 AND version=1`,
		"tasa-de-paro-epa", "2026-Q1")
	if err := row.Scan(&v1Value, &v1Status); err != nil {
		t.Fatalf("reading v1 after withdrawal: %v", err)
	}
	if v1Value != 11.5 || v1Status != "D" {
		t.Errorf("expected the prior version to remain unchanged (11.5/D), got %v/%s", v1Value, v1Status)
	}
}
