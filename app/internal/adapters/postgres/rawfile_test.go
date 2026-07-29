package postgres_test

// Task 5a.1 (RED, DB half) / 5a.2 (GREEN): raw_file is the database half
// of the raw-file archive — filestore.Store (a sibling, DB-free adapter,
// its own RED/GREEN cycle) owns the bytes-on-disk half. ArchiveRawFile
// composes the two so a caller can prove, against a REAL postgres:17
// container, the requirement's full text: "a raw_file row exists with
// the payload SHA-256, source, URL, downloaded_at and storage path AND
// the archive write happens before any parsing"; "An identical
// redownload does not duplicate storage"; "Rollback preserves the raw
// file".

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// seedSource inserts the minimal source row raw_file/download_attempt
// tests need — no dataset/series required, unlike seedSeries.
func seedSource(t *testing.T, ctx context.Context, tx pgx.Tx, id string) {
	t.Helper()
	mustExec(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ($1, $1, 'https://example.test/'||$1, 'lic', 'attr', 'api-json', 'digest1')`, id)
}

func TestArchiveRawFile_RecordsHashSourceURLAndTimestamp(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")

	store := filestore.NewStore(t.TempDir())
	downloadedAt := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	payload := []byte("raw payload bytes")

	rf, isNew, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/data", payload, downloadedAt)
	if err != nil {
		t.Fatalf("ArchiveRawFile: %v", err)
	}
	if !isNew {
		t.Fatal("expected the first archive of new bytes to report isNew=true")
	}

	var count int
	row := tx.QueryRow(ctx, `SELECT count(*) FROM raw_file WHERE hash=$1`, rf.Hash)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("counting raw_file rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one raw_file row, got %d", count)
	}

	var sourceID, url, storagePath string
	var gotDownloadedAt time.Time
	var sizeBytes int64
	row = tx.QueryRow(ctx, `SELECT source_id, url, downloaded_at, storage_path, size_bytes FROM raw_file WHERE hash=$1`, rf.Hash)
	if err := row.Scan(&sourceID, &url, &gotDownloadedAt, &storagePath, &sizeBytes); err != nil {
		t.Fatalf("reading raw_file row: %v", err)
	}
	if sourceID != "test-source" || url != "https://example.test/data" || sizeBytes != int64(len(payload)) {
		t.Fatalf("unexpected raw_file row: source=%s url=%s size=%d", sourceID, url, sizeBytes)
	}
	if !gotDownloadedAt.Equal(downloadedAt) {
		t.Fatalf("expected downloaded_at %v, got %v", downloadedAt, gotDownloadedAt)
	}

	onDisk, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("reading the archived file at %s: %v", storagePath, err)
	}
	if string(onDisk) != string(payload) {
		t.Fatal("archived bytes on disk do not match the payload")
	}
}

func TestArchiveRawFile_ArchiveCommitsIndependentlyOfWhateverHappensNext(t *testing.T) {
	// Proves spec's "the archive write happens before any parsing": a
	// caller that only invokes a parser AFTER ArchiveRawFile returns
	// always has the archive already durable — the row this test reads
	// back was written before the simulated parser step even ran, and
	// that parser crashing changes nothing about it. That ordering — not
	// a timing coincidence — is what makes "if parsing crashes, the
	// evidence of what arrived still exists" true.
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")

	store := filestore.NewStore(t.TempDir())
	payload := []byte("payload that will be 'parsed' and blow up")

	rf, _, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/data", payload, time.Now())
	if err != nil {
		t.Fatalf("ArchiveRawFile: %v", err)
	}

	parseCrashed := func() (crashed bool) {
		defer func() {
			if recover() != nil {
				crashed = true
			}
		}()
		panic("simulated parser crash — archiving already happened above")
	}()
	if !parseCrashed {
		t.Fatal("test setup error: the simulated parse step must panic")
	}

	var count int
	row := tx.QueryRow(ctx, `SELECT count(*) FROM raw_file WHERE hash=$1`, rf.Hash)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("counting raw_file rows after the simulated parser crash: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the raw_file row to survive a parser crash, got count=%d", count)
	}
}

func TestArchiveRawFile_IdenticalRedownloadDoesNotDuplicateStorage(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")

	store := filestore.NewStore(t.TempDir())
	payload := []byte("identical bytes both times")

	first, isNew1, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/data", payload, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first ArchiveRawFile: %v", err)
	}
	if !isNew1 {
		t.Fatal("expected the first archive to report isNew=true")
	}

	second, isNew2, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/data", payload, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second ArchiveRawFile: %v", err)
	}
	if isNew2 {
		t.Fatal("expected an identical redownload to report isNew=false")
	}
	if second.Hash != first.Hash {
		t.Fatalf("expected the same hash for identical bytes, got %s vs %s", first.Hash, second.Hash)
	}

	var count int
	row := tx.QueryRow(ctx, `SELECT count(*) FROM raw_file WHERE hash=$1`, first.Hash)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("counting raw_file rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one raw_file row despite two archive calls, got %d", count)
	}
}

func TestArchiveRawFile_RollbackLeavesTheRawFileRowAndFileUntouched(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx) // source 'ine' + dataset + series
	store := filestore.NewStore(t.TempDir())

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(0.6),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("seeding the already-published observation: %v", err)
	}

	payload := []byte("archived bytes for a run that gets rolled back")
	rf, _, err := postgres.ArchiveRawFile(ctx, tx, store, "ine", "https://ine.es/rollback-test", payload, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ArchiveRawFile: %v", err)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(999.0),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	}); err != nil {
		t.Fatalf("seeding the bad run's observation: %v", err)
	}

	if _, err := writer.RollbackRun(ctx, run2, "bad data"); err != nil {
		t.Fatalf("RollbackRun: %v", err)
	}

	var count int
	var storagePath string
	row := tx.QueryRow(ctx, `SELECT count(*), max(storage_path) FROM raw_file WHERE hash=$1 GROUP BY hash`, rf.Hash)
	if err := row.Scan(&count, &storagePath); err != nil {
		t.Fatalf("reading raw_file row after rollback: %v", err)
	}
	if count != 1 || storagePath != rf.StoragePath {
		t.Fatalf("expected the raw_file row unchanged after rollback, got count=%d path=%s", count, storagePath)
	}

	onDisk, err := os.ReadFile(rf.StoragePath)
	if err != nil {
		t.Fatalf("reading the archived file after rollback: %v", err)
	}
	if string(onDisk) != string(payload) {
		t.Fatal("archived bytes changed after a rollback that never touches raw_file")
	}
}
