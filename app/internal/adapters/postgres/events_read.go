package postgres

// Task 4.1/4.2: the events half of design D-2's read list (the breaks
// half, ResolveActiveBreaksForSeries, already existed in editorial.go).
// See events_read_test.go's package doc comment for why ListActiveEvents
// does not filter on seriesID today -- migration 0001's event table
// carries no scope columns at all, unlike series_break, so every
// currently active event is, by the schema this change inherited, global.

import (
	"context"
	"fmt"
)

// ListActiveEvents returns every currently active (non-retired) event
// (spec publishing-export, "the editorial events and government entries
// that apply to it"). seriesID is accepted for signature parity with
// ResolveActiveBreaksForSeries and to leave room for a real per-series
// scope in a future slice, but is not read -- see this file's package doc
// comment. An unconfirmed (date_status="unconfirmed") entry never reaches
// this table at all: ingestion.ReconcileEditorialConfig already declines
// to project one (reconcile.go), recording its id in PendingEventIDs
// instead -- so this function never needs to skip one itself.
func ListActiveEvents(ctx context.Context, db DBTX, seriesID string) ([]Event, error) {
	_ = seriesID
	rows, err := db.Query(ctx, `SELECT `+eventColumns+` FROM event WHERE retired_at IS NULL ORDER BY date_start, id`)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing active events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scanning an active event row: %w", err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating active event rows: %w", err)
	}
	return out, nil
}
