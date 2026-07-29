package postgres

// Task 5b.6 (GREEN): CreateIngestionRun inserts the initial ingestion_run
// row for a series-scoped fetch, right after the raw payload has already
// been archived (design.md "[Tx1] download_attempt + raw_file +
// ingestion_run(pending)"). Nothing before this batch created that row
// at all -- ApplyGate's recordRunOutcome (task 4.16) only ever UPDATEs an
// already-existing one, which is exactly the row this function creates.

import (
	"context"
	"fmt"
	"time"
)

// CreateIngestionRun inserts a new ingestion_run row with outcome
// RunOutcomePending, already carrying the just-archived raw file's hash
// (raw_file_hash is set from the start, never NULL-then-backfilled --
// the fetch already succeeded by the time this is called, or the caller
// would never have reached here). It returns the new row's id, which the
// caller threads through validation and into ApplyGate to reach a
// terminal outcome.
func CreateIngestionRun(ctx context.Context, db DBTX, datasetID, seriesID string, startedAt time.Time, rawFileHash string) (int64, error) {
	row := db.QueryRow(ctx, `INSERT INTO ingestion_run (dataset_id, series_id, started_at, raw_file_hash, outcome)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		datasetID, seriesID, startedAt, rawFileHash, string(RunOutcomePending))
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("postgres: creating ingestion_run for dataset=%s series=%s: %w", datasetID, seriesID, err)
	}
	return id, nil
}
