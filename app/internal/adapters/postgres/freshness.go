package postgres

// Task 5a.7/5a.8 (GREEN, DB half): resolves the one fact
// freshness.Resolve needs -- a source's last successful
// download_attempt -- and propagates that source-level verdict to every
// series under it (spec raw-file-archive, "Freshness state is tracked
// per source"; "that state MUST propagate to every series belonging to
// that source's datasets"). The decision itself stays in package
// freshness (task 5a.7's pure half); this file is purely the query, kept
// in package postgres for the same reason gate.go's effect half is: the
// pure package's purity guard (task 4.1: go-list-deps + clock/OS source
// scan) never has to make an exception for a DB-touching function.

import (
	"context"
	"fmt"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
)

// successOutcomes are the download_attempt.outcome values that count as
// "checked and reachable" for freshness purposes -- a new file and an
// unchanged check both prove the source responded; every other outcome
// is some flavor of failure.
var successOutcomes = []string{string(OutcomeNewFile), string(OutcomeUnchanged)}

// LastSuccessfulDownloadAttempt returns the most recent attempted_at
// among sourceID's successful download_attempt rows, or nil if it has
// never succeeded.
func LastSuccessfulDownloadAttempt(ctx context.Context, db DBTX, sourceID string) (*time.Time, error) {
	row := db.QueryRow(ctx, `SELECT max(attempted_at) FROM download_attempt
		WHERE source_id=$1 AND outcome = ANY($2)`, sourceID, successOutcomes)

	var last *time.Time
	if err := row.Scan(&last); err != nil {
		return nil, fmt.Errorf("postgres: reading the last successful download_attempt for %s: %w", sourceID, err)
	}
	return last, nil
}

// SourceFreshness resolves sourceID's freshness state as of asOf (spec
// "Freshness state is tracked per source"). asOf is always an explicit
// parameter, never time.Now() read inside this function, matching
// freshness.Resolve's own contract.
func SourceFreshness(ctx context.Context, db DBTX, sourceID string, asOf time.Time) (freshness.State, error) {
	last, err := LastSuccessfulDownloadAttempt(ctx, db, sourceID)
	if err != nil {
		return "", err
	}
	return freshness.Resolve(last, asOf), nil
}

// SeriesFreshness resolves the freshness state that propagates to
// seriesID from its dataset's source (spec "that state MUST propagate
// to every series belonging to that source's datasets") -- there is no
// per-series freshness row; a series is always exactly as fresh as the
// source it descends from.
func SeriesFreshness(ctx context.Context, db DBTX, seriesID string, asOf time.Time) (freshness.State, error) {
	row := db.QueryRow(ctx, `SELECT d.source_id FROM series s JOIN dataset d ON d.id = s.dataset_id WHERE s.id=$1`, seriesID)
	var sourceID string
	if err := row.Scan(&sourceID); err != nil {
		return "", fmt.Errorf("postgres: resolving the source for series %s: %w", seriesID, err)
	}
	return SourceFreshness(ctx, db, sourceID, asOf)
}
