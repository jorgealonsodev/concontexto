package postgres

// Task 4.1/4.2: the events half of design D-2's read list (the breaks
// half, ResolveActiveBreaksForSeries, already existed in editorial.go).
//
// THIS FUNCTION USED NOT TO READ seriesID AT ALL, and its own comment here
// explained why: migration 0001's event table carried no scope columns, so
// every active event was global by construction, and the parameter existed
// only "to leave room for a real per-series scope in a future slice".
// Migration 0007 adds those columns and this is that slice — the parameter
// is now load-bearing, and the widening rule below is
// ResolveActiveBreaksForSeries', reused rather than re-invented.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListActiveEvents returns every currently active (non-retired) event whose
// SCOPE covers seriesID (spec publishing-export, "the editorial events and
// government entries that apply to it" — now a real filter rather than a
// disclosed approximation of one).
//
// FOUR SCOPES, ONE QUERY, NO DUPLICATION IN STORAGE. 'global' covers every
// series (a change of government and a worldwide shock are facts about the
// calendar) and is what every entry the registry holds today declares;
// 'series', 'dataset' and 'source' widen exactly as a break's do, so one
// dataset-scoped entry resolves for every series under that dataset and is
// stored once (spec editorial-config, "it is stored once, not once per
// series").
//
// AN UNKNOWN SERIES IS NOT AN ERROR, and that differs deliberately from
// ResolveActiveBreaksForSeries, which fails when the series/dataset join
// finds nothing. The difference is what each function would cost if it were
// wrong. A break that failed to resolve would let a page render values
// across a methodological rupture with no warning, so failing loudly is
// right there. Here, an unresolvable series simply has no dataset and no
// source, which means no dataset- or source-scoped entry can cover it —
// while 'global' entries and an entry naming the slug directly still can.
// Returning those and no others is the honest answer, and it keeps a
// configuration mistake from taking down an export that has nothing to do
// with it.
//
// An unconfirmed (date_status="unconfirmed") entry never reaches this table
// at all: ingestion.ReconcileEditorialConfig already declines to project one
// (reconcile.go), recording its id in PendingEventIDs instead — so this
// function never needs to skip one itself.
func ListActiveEvents(ctx context.Context, db DBTX, seriesID string) ([]Event, error) {
	// Empty strings, not NULLs, when the chain is unknown: the predicate
	// below compares scope_ref (NOT NULL) for equality, and no configured
	// dataset or source id is ever the empty string, so an unresolved chain
	// simply matches nothing rather than needing a second query shape.
	var datasetID, sourceID string
	row := db.QueryRow(ctx, `SELECT d.id, d.source_id FROM series s JOIN dataset d ON d.id = s.dataset_id WHERE s.id=$1`, seriesID)
	if err := row.Scan(&datasetID, &sourceID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: resolving dataset/source for series %q: %w", seriesID, err)
	}

	rows, err := db.Query(ctx, `SELECT `+eventColumns+`
		FROM event
		WHERE retired_at IS NULL AND (
			scope_kind='global' OR
			(scope_kind='series'  AND scope_ref=$1) OR
			(scope_kind='dataset' AND scope_ref=$2) OR
			(scope_kind='source'  AND scope_ref=$3)
		)
		ORDER BY date_start, id`, seriesID, datasetID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing active events for series %q: %w", seriesID, err)
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
