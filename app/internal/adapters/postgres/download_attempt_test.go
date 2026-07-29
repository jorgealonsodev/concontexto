package postgres_test

// Task 5a.3 (RED) / 5a.4 (GREEN): download_attempt is why "we checked
// and nothing changed" is distinguishable from "we could not check"
// (spec raw-file-archive, "Download attempts are logged independently
// of content").

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestRecordDownloadAttempt_UnchangedRedownloadRecordsSuccessWithExistingHash(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")
	mustExec(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ('existing-hash', 'test-source', 'https://example.test/data', now(), '/app_data/existing-hash', 10)`)

	hash := "existing-hash"
	attempt, err := postgres.RecordDownloadAttempt(ctx, tx, postgres.DownloadAttempt{
		SourceID: "test-source", URL: "https://example.test/data",
		AttemptedAt:   time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		ResultingHash: &hash, Outcome: postgres.OutcomeUnchanged,
	})
	if err != nil {
		t.Fatalf("RecordDownloadAttempt: %v", err)
	}
	if attempt.ID == 0 {
		t.Fatal("expected a non-zero id")
	}

	var outcome, resultingHash string
	row := tx.QueryRow(ctx, `SELECT outcome, resulting_hash FROM download_attempt WHERE id=$1`, attempt.ID)
	if err := row.Scan(&outcome, &resultingHash); err != nil {
		t.Fatalf("reading download_attempt row: %v", err)
	}
	if outcome != string(postgres.OutcomeUnchanged) || resultingHash != hash {
		t.Fatalf("unexpected row: outcome=%s hash=%s", outcome, resultingHash)
	}

	var rawFileCount int
	row = tx.QueryRow(ctx, `SELECT count(*) FROM raw_file WHERE hash=$1`, hash)
	if err := row.Scan(&rawFileCount); err != nil {
		t.Fatalf("counting raw_file rows: %v", err)
	}
	if rawFileCount != 1 {
		t.Fatalf("expected no new raw_file row from an unchanged attempt, got count=%d", rawFileCount)
	}
}

func TestRecordDownloadAttempt_FailedAttemptRecordsFailureWithNullHash(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")

	attempt, err := postgres.RecordDownloadAttempt(ctx, tx, postgres.DownloadAttempt{
		SourceID: "test-source", URL: "https://example.test/data",
		AttemptedAt:   time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		ResultingHash: nil, Outcome: postgres.OutcomeRetryableTransport,
	})
	if err != nil {
		t.Fatalf("RecordDownloadAttempt: %v", err)
	}

	var outcome string
	var resultingHash *string
	row := tx.QueryRow(ctx, `SELECT outcome, resulting_hash FROM download_attempt WHERE id=$1`, attempt.ID)
	if err := row.Scan(&outcome, &resultingHash); err != nil {
		t.Fatalf("reading download_attempt row: %v", err)
	}
	if outcome != string(postgres.OutcomeRetryableTransport) {
		t.Fatalf("expected outcome %s, got %s", postgres.OutcomeRetryableTransport, outcome)
	}
	if resultingHash != nil {
		t.Fatalf("expected a null resulting_hash on a failed attempt, got %v", *resultingHash)
	}
}

func TestRecordDownloadAttempt_NewFileRecordsSuccessWithTheNewHash(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")
	mustExec(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ('new-hash', 'test-source', 'https://example.test/data', now(), '/app_data/new-hash', 10)`)

	hash := "new-hash"
	attempt, err := postgres.RecordDownloadAttempt(ctx, tx, postgres.DownloadAttempt{
		SourceID: "test-source", URL: "https://example.test/data",
		AttemptedAt:   time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		ResultingHash: &hash, Outcome: postgres.OutcomeNewFile,
	})
	if err != nil {
		t.Fatalf("RecordDownloadAttempt: %v", err)
	}
	if attempt.Outcome != postgres.OutcomeNewFile {
		t.Fatalf("expected outcome %s, got %s", postgres.OutcomeNewFile, attempt.Outcome)
	}
	if attempt.ResultingHash == nil || *attempt.ResultingHash != hash {
		t.Fatalf("expected the returned attempt to carry the new hash %s, got %v", hash, attempt.ResultingHash)
	}
}
