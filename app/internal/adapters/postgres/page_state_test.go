package postgres_test

// Remediation B (verify-report CRITICAL-4), DB half. The read port that
// closes publishing-export's scenario clause "AND the series carries the
// state that drives PRD §6.1.3's validation banner".
//
// The fact this reads was ALREADY being written before this slice:
// ApplyGate (gate.go) records ingestion_run.outcome='validation-failed'
// on every Block verdict, and has since task 4.16. Nothing ever read it
// back for the reader-facing side, so `web/src/content/indicators/
// methodology.ts` carried all six series hard-coded to
// `pageState: { kind: "fresh" }` and a real validation failure produced
// no banner without a source edit and a redeploy. This port is the
// missing read, not a new write.
//
// Reuses this package's established testcontainers harness (TestMain,
// newTx) and the seedSeries/seedIngestionRun helpers from
// observation_writer_test.go, unmodified.

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// seedRunWithOutcome records one raw_file + ingestion_run pair with an
// explicit terminal outcome — seedIngestionRun's own helper always
// writes 'succeeded', which is exactly the case this port must
// distinguish from.
func seedRunWithOutcome(
	t *testing.T, ctx context.Context, tx pgx.Tx,
	hash string, startedAt time.Time, outcome postgres.RunOutcome,
) int64 {
	t.Helper()
	mustExec(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ($1, 'ine', 'https://ine.es/data', $2, '/app_data/'||$1, 100)`, hash, startedAt)

	row := tx.QueryRow(ctx, `INSERT INTO ingestion_run (dataset_id, series_id, started_at, finished_at, raw_file_hash, outcome)
		VALUES ('ine-epa', 'tasa-de-paro-epa', $1, $1, $2, $3) RETURNING id`, startedAt, hash, string(outcome))
	var id int64
	if err := row.Scan(&id); err != nil {
		t.Fatalf("seedRunWithOutcome: %v", err)
	}
	return id
}

func TestSeriesValidationOutcome_LatestRunFailedValidationReportsTheLastSucceededRunsDate(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	lastGood := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	seedRunWithOutcome(t, ctx, tx, "hash-good", lastGood, postgres.RunOutcomeSucceeded)
	seedRunWithOutcome(t, ctx, tx, "hash-bad", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), postgres.RunOutcomeValidationFailed)

	got, err := postgres.SeriesValidationOutcome(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("SeriesValidationOutcome: %v", err)
	}
	if !got.LatestRunFailedValidation {
		t.Fatal("expected the latest run to be reported as validation-failed")
	}
	if got.LastSucceededAt == nil {
		t.Fatal("expected the last succeeded run's timestamp, got nil")
	}
	if !got.LastSucceededAt.Equal(lastGood) {
		t.Fatalf("expected the last CORRECT update %v, got %v", lastGood, *got.LastSucceededAt)
	}
}

func TestSeriesValidationOutcome_ASucceededLatestRunIsNotAFailure(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	seedRunWithOutcome(t, ctx, tx, "hash-bad", time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), postgres.RunOutcomeValidationFailed)
	seedRunWithOutcome(t, ctx, tx, "hash-good", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), postgres.RunOutcomeSucceeded)

	got, err := postgres.SeriesValidationOutcome(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("SeriesValidationOutcome: %v", err)
	}
	// A series that failed validation in the past and has since recovered
	// is NOT in the banner state — the spec's scenario is scoped to "a
	// series whose LATEST run failed validation".
	if got.LatestRunFailedValidation {
		t.Fatal("a recovered series must not be reported as validation-failed")
	}
}

// A run still sitting at 'pending' has reached no terminal verdict —
// CreateIngestionRun inserts that placeholder before validation has had
// a chance to decide anything, and a crash in between leaves it there
// (gate.go's RunOutcomePending doc comment: "an honest, queryable
// 'started but never finished' row"). Reporting it as a validation
// failure would put a banner on the page claiming the source published a
// datum that failed our checks, which is a statement about the source
// that the data does not support (P4/P7: never fabricate).
func TestSeriesValidationOutcome_APendingRunIsNotAValidationFailure(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	seedRunWithOutcome(t, ctx, tx, "hash-good", time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), postgres.RunOutcomeSucceeded)
	seedRunWithOutcome(t, ctx, tx, "hash-pending", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), postgres.RunOutcomePending)

	got, err := postgres.SeriesValidationOutcome(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("SeriesValidationOutcome: %v", err)
	}
	if got.LatestRunFailedValidation {
		t.Fatal("a pending run must not be reported as a validation failure")
	}
}

func TestSeriesValidationOutcome_NoRunsAtAllIsNotAFailure(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	got, err := postgres.SeriesValidationOutcome(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("SeriesValidationOutcome: %v", err)
	}
	if got.LatestRunFailedValidation {
		t.Fatal("a series with no ingestion_run rows at all must not be reported as validation-failed")
	}
	if got.LastSucceededAt != nil {
		t.Fatalf("expected no last-succeeded timestamp, got %v", *got.LastSucceededAt)
	}
}

// The banner names a date. If the very first run a series ever had
// failed validation, there IS no "last correct update" to name — the
// caller must be able to tell that apart from "no failure", so the
// composed page state can fall back rather than print an empty or
// invented date.
func TestSeriesValidationOutcome_AFailureWithNoPriorSuccessReportsNoDate(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	seedRunWithOutcome(t, ctx, tx, "hash-bad", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), postgres.RunOutcomeValidationFailed)

	got, err := postgres.SeriesValidationOutcome(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("SeriesValidationOutcome: %v", err)
	}
	if !got.LatestRunFailedValidation {
		t.Fatal("expected the latest run to be reported as validation-failed")
	}
	if got.LastSucceededAt != nil {
		t.Fatalf("expected no last-succeeded timestamp, got %v", *got.LastSucceededAt)
	}
}

// One series' failure must never leak onto another's page.
func TestSeriesValidationOutcome_IsScopedToOneSeries(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ocupados-epa', 'ine-epa', 'Ocupados', 'personas', 'Q', 'ES', 0, false, 'digest1')`)

	seedRunWithOutcome(t, ctx, tx, "hash-bad", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), postgres.RunOutcomeValidationFailed)

	got, err := postgres.SeriesValidationOutcome(ctx, tx, "ocupados-epa")
	if err != nil {
		t.Fatalf("SeriesValidationOutcome: %v", err)
	}
	if got.LatestRunFailedValidation {
		t.Fatal("tasa-de-paro-epa's failed run must not mark ocupados-epa as validation-failed")
	}
}
