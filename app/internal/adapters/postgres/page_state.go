package postgres

// Remediation B (verify-report CRITICAL-4), DB half: the read that closes
// publishing-export's "AND the series carries the state that drives PRD
// §6.1.3's validation banner".
//
// Deliberately narrow. This function answers ONE database fact -- did
// this series' most recent ingestion run fail validation, and when did
// it last succeed -- and nothing else. It does NOT decide the page's
// reader-facing state: "discontinued" is editorial configuration with no
// row in this schema to read, and composing the two axes into one state
// is publishing.SeriesPageState's job (export.go). Keeping the decision
// out of the adapter is the same split gate.go already established
// between validation.Gate (pure decision) and ApplyGate (effect).
//
// This reads a column that has been written since task 4.16 and never
// read back: ApplyGate records outcome='validation-failed' on every
// Block verdict. The gap CRITICAL-4 names was never a missing write.

import (
	"context"
	"fmt"
	"time"
)

// ValidationOutcome is one series' validation-axis state, as recorded by
// the publish gate.
//
// LastSucceededAt is the "date of the last correct update" the
// indicator-page spec's banner names verbatim ("Última actualización
// correcta: {fecha}"). It is nil when the series has never had a
// succeeded run at all -- a real possibility for a series whose very
// first run failed validation, and one the caller must handle rather
// than print an empty or invented date (P4: never fabricate).
type ValidationOutcome struct {
	LatestRunFailedValidation bool
	LastSucceededAt           *time.Time
}

// SeriesValidationOutcome resolves seriesID's validation-axis state.
//
// "Latest" is ordered by started_at, not by id: id ordering happens to
// agree today because runs are created in start order, but started_at is
// the fact the question is actually about, and the column is NOT NULL.
//
// A 'pending' run is not a failure. CreateIngestionRun inserts that
// placeholder before validation has decided anything, and a crash in
// between leaves it there permanently (see RunOutcomePending's own doc
// comment). Reporting it as a validation failure would put a banner on
// the page asserting that the SOURCE published a datum failing our
// checks -- a claim about a third party the data does not support.
// Only the explicit 'validation-failed' terminal outcome counts.
func SeriesValidationOutcome(ctx context.Context, db DBTX, seriesID string) (ValidationOutcome, error) {
	row := db.QueryRow(ctx, `
		SELECT
			(SELECT outcome FROM ingestion_run
			  WHERE series_id = $1 ORDER BY started_at DESC, id DESC LIMIT 1),
			(SELECT max(started_at) FROM ingestion_run
			  WHERE series_id = $1 AND outcome = $2)`,
		seriesID, string(RunOutcomeSucceeded))

	var latestOutcome *string
	var lastSucceededAt *time.Time
	if err := row.Scan(&latestOutcome, &lastSucceededAt); err != nil {
		return ValidationOutcome{}, fmt.Errorf("postgres: resolving the validation outcome for series %s: %w", seriesID, err)
	}

	return ValidationOutcome{
		LatestRunFailedValidation: latestOutcome != nil && *latestOutcome == string(RunOutcomeValidationFailed),
		LastSucceededAt:           lastSucceededAt,
	}, nil
}
