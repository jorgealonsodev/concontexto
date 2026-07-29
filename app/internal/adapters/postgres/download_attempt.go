package postgres

// Task 5a.3/5a.4 (GREEN): download_attempt is why "we checked and
// nothing changed" is distinguishable from "we could not check" (spec
// raw-file-archive, "Download attempts are logged independently of
// content"; design.md "'unchanged' is why this table exists apart from
// raw_file (amber semaphore)"). Every scheduled attempt gets a row here,
// success or failure, with or without a resolving hash -- raw_file
// alone (keyed by content) cannot express "checked, unchanged" at all,
// since an unchanged check writes no new raw_file row.

import (
	"context"
	"fmt"
	"time"
)

// DownloadOutcome mirrors download_attempt.outcome's documented value
// set (migration 0001's comment). Only the outcomes this batch's scope
// (5a.3/5a.4) needs to produce directly are exercised by tests here; the
// finer SilentEmpty/SchemaDrift/ResponseTooLarge/SourceRefusal
// classification is sourceerr.FailureClass's job (task 5a.13/5a.14, out
// of this PR's scope) -- this type is deliberately the full documented
// domain so that classification can be wired in later without a schema
// or type change.
type DownloadOutcome string

const (
	OutcomeNewFile            DownloadOutcome = "new-file"
	OutcomeUnchanged          DownloadOutcome = "unchanged"
	OutcomeRetryableTransport DownloadOutcome = "retryable-transport"
	OutcomeSourceRefusal      DownloadOutcome = "source-refusal"
	OutcomeSilentEmpty        DownloadOutcome = "silent-empty"
	OutcomeSchemaDrift        DownloadOutcome = "schema-drift"
	OutcomeResponseTooLarge   DownloadOutcome = "response-too-large"
)

// DownloadAttempt is one download_attempt row.
type DownloadAttempt struct {
	ID            int64
	SourceID      string
	URL           string
	AttemptedAt   time.Time
	ResultingHash *string // nil on any failure outcome
	Outcome       DownloadOutcome
}

// RecordDownloadAttempt inserts one download_attempt row. Callers pass a
// nil ResultingHash for any failure outcome (spec "A failed attempt is
// recorded with no hash") and the resolving hash -- new or unchanged --
// for a successful one (spec "An unchanged check is recorded").
func RecordDownloadAttempt(ctx context.Context, db DBTX, in DownloadAttempt) (DownloadAttempt, error) {
	row := db.QueryRow(ctx, `INSERT INTO download_attempt (source_id, url, attempted_at, resulting_hash, outcome)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, in.SourceID, in.URL, in.AttemptedAt, in.ResultingHash, string(in.Outcome))
	if err := row.Scan(&in.ID); err != nil {
		return DownloadAttempt{}, fmt.Errorf("postgres: recording download_attempt for %s: %w", in.URL, err)
	}
	return in, nil
}
