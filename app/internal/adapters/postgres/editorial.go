package postgres

// Task 7.4/7.5, 7.6/7.7, 7.8/7.9: series_break and event are a projection
// of config/{rupturas,eventos,gobiernos}.yaml (design.md "editorial YAML
// is reconciled -- never hand-edited -- into series_break and event").
// ReconcileBreaks/ReconcileEvents each run ONE transactional, idempotent
// FULL RECONCILE (spec editorial-config, "Editorial YAML is authoritative
// and reconciled transactionally"):
//   - an entry present in the input but absent from the live rows is
//     INSERTed;
//   - an entry present in both, whose ConfigDigest differs (or whose row
//     is currently retired), is UPDATEd in place -- retired_at is cleared,
//     and NO retirement is ever recorded for an in-place edit (spec
//     "Editing a description updates rather than replaces");
//   - a live row absent from the input is soft-retired (retired_at=now())
//     -- NEVER hard-deleted (principle P7);
//   - a row whose content is byte-identical to what is already live is
//     left completely untouched, so an identical re-run issues zero
//     writes (spec "Re-running the reconcile changes nothing").
//
// The whole diff is computed in Go against ONE read of the current live
// rows, then applied as precise per-row statements inside a single
// transaction -- never a blind "upsert everything". That is what makes
// "zero rows changed" on a repeat run a literal, directly-assertable
// count rather than an artifact of ON CONFLICT DO NOTHING, and what makes
// a partial failure (e.g. two desired entries colliding on the same
// natural key) roll back to a byte-identical pre-reconcile state (spec "A
// failed reconcile leaves no partial state") -- the transaction's single
// Commit at the very end is the only point anything durable happens.
//
// A retired row that reappears in the input (spec "Reverting the YAML
// restores the prior projection") is resolved by natural key to the SAME
// row -- never a fresh INSERT. Migration 0001's schema has no separate
// history table, so that persisted identity (the row was retired, not
// deleted, and is the exact row un-retired) IS what "the retirement
// history remains recorded" means here.

import (
	"context"
	"fmt"
	"time"
)

// SeriesBreakInput is one already-digested break entry ready to
// reconcile (ingestion.ReconcileEditorialConfig computes ConfigDigest
// from the source YAML). It carries NO dismissible, optional-visibility
// or default-hidden field anywhere -- spec "Breaks are stored as
// non-dismissible" (principle P4) -- guarded by
// TestSeriesBreakTypes_ExposeNoDismissibleAttribute in editorial_test.go.
type SeriesBreakInput struct {
	BreakKey     string
	ScopeKind    string // series | dataset | source
	ScopeRef     string
	Date         time.Time
	Kind         string
	NoteMD       string
	SourceURL    string
	ConfigDigest string
}

func (in SeriesBreakInput) key() seriesBreakKey {
	return seriesBreakKey{in.BreakKey, in.ScopeKind, in.ScopeRef}
}

// SeriesBreak is one persisted series_break row.
type SeriesBreak struct {
	BreakKey     string
	ScopeKind    string
	ScopeRef     string
	Date         time.Time
	Kind         string
	NoteMD       string
	SourceURL    *string
	ConfigDigest string
	RetiredAt    *time.Time
}

type seriesBreakKey struct{ BreakKey, ScopeKind, ScopeRef string }

// ReconcileCounts is one reconcile call's effect, precise enough to prove
// "an identical reconcile changes zero rows" as a literal zero-valued
// struct (spec "Re-running the reconcile changes nothing").
type ReconcileCounts struct {
	Inserted int
	Updated  int
	Retired  int
}

// ReconcileBreaks reconciles desired (every entry currently declared in
// rupturas.yaml that ingestion.ReconcileEditorialConfig chose to project
// -- an unconfirmed-date entry is never passed here, see that function's
// doc comment) against series_break, per this file's package doc comment.
func ReconcileBreaks(ctx context.Context, db TxBeginner, desired []SeriesBreakInput) (ReconcileCounts, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return ReconcileCounts{}, fmt.Errorf("postgres: beginning break reconcile: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	counts, err := reconcileBreaksTx(ctx, tx, desired)
	if err != nil {
		return ReconcileCounts{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconcileCounts{}, fmt.Errorf("postgres: committing break reconcile: %w", err)
	}
	return counts, nil
}

func reconcileBreaksTx(ctx context.Context, tx DBTX, desired []SeriesBreakInput) (ReconcileCounts, error) {
	live, err := allSeriesBreaks(ctx, tx)
	if err != nil {
		return ReconcileCounts{}, err
	}

	var counts ReconcileCounts
	seen := map[seriesBreakKey]bool{}
	for _, in := range desired {
		k := in.key()
		if seen[k] {
			return ReconcileCounts{}, fmt.Errorf("postgres: reconciling series_break: duplicate key %+v in one reconcile batch", k)
		}
		seen[k] = true

		current, exists := live[k]
		switch {
		case !exists:
			if _, err := tx.Exec(ctx, `INSERT INTO series_break (break_key, scope_kind, scope_ref, date, kind, note_md, source_url, config_digest)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
				in.BreakKey, in.ScopeKind, in.ScopeRef, in.Date, in.Kind, in.NoteMD, nullableString(in.SourceURL), in.ConfigDigest); err != nil {
				return ReconcileCounts{}, fmt.Errorf("postgres: inserting series_break %q: %w", in.BreakKey, err)
			}
			counts.Inserted++
		case current.ConfigDigest != in.ConfigDigest || current.RetiredAt != nil:
			if _, err := tx.Exec(ctx, `UPDATE series_break SET date=$4, kind=$5, note_md=$6, source_url=$7, config_digest=$8, retired_at=NULL
				WHERE break_key=$1 AND scope_kind=$2 AND scope_ref=$3`,
				in.BreakKey, in.ScopeKind, in.ScopeRef, in.Date, in.Kind, in.NoteMD, nullableString(in.SourceURL), in.ConfigDigest); err != nil {
				return ReconcileCounts{}, fmt.Errorf("postgres: updating series_break %q: %w", in.BreakKey, err)
			}
			counts.Updated++
		}
	}

	for k, current := range live {
		if seen[k] || current.RetiredAt != nil {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE series_break SET retired_at=now()
			WHERE break_key=$1 AND scope_kind=$2 AND scope_ref=$3`, k.BreakKey, k.ScopeKind, k.ScopeRef); err != nil {
			return ReconcileCounts{}, fmt.Errorf("postgres: retiring series_break %q: %w", k.BreakKey, err)
		}
		counts.Retired++
	}
	return counts, nil
}

// EditorialCounts is one ReconcileEditorial call's effect, one
// ReconcileCounts per reconciled table.
//
// It replaces the previous (ReconcileCounts, ReconcileCounts, error)
// return, which could not absorb a third editorial table without every
// call site changing shape anyway. Naming the tables also removes the
// positional ambiguity a third bare return value would have introduced.
type EditorialCounts struct {
	Breaks           ReconcileCounts
	Events           ReconcileCounts
	Acknowledgements ReconcileCounts
}

// ReconcileEditorial reconciles series_break, event AND
// validation_acknowledgement inside ONE transaction (task: closing PR 7a's
// disclosed gap — spec editorial-config's "A failed reconcile leaves no
// partial state" scenario is written against ONE reconcile, not "the
// breaks half of one reconcile"; two independent top-level transactions
// could leave an already-committed breaks change durable while the events
// half failed). It reuses reconcileBreaksTx/reconcileEventsTx/
// reconcileAcknowledgementsTx (the same per-table diff logic each table's
// own exported entry point uses) against a single shared tx, so a
// natural-key collision in ANY of the three rolls back ALL of them.
//
// validation_acknowledgement joined this transaction rather than getting
// one of its own for exactly the reason the events half did: an
// acknowledgement resolves a finding about a series whose breaks are
// reconciled in the same pass, and a half-applied editorial state is a
// state no reviewer ever approved.
func ReconcileEditorial(ctx context.Context, db TxBeginner, breaks []SeriesBreakInput, events []EventInput, acknowledgements []AcknowledgementInput) (EditorialCounts, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return EditorialCounts{}, fmt.Errorf("postgres: beginning editorial reconcile: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var counts EditorialCounts
	if counts.Breaks, err = reconcileBreaksTx(ctx, tx, breaks); err != nil {
		return EditorialCounts{}, err
	}
	if counts.Events, err = reconcileEventsTx(ctx, tx, events); err != nil {
		return EditorialCounts{}, err
	}
	if counts.Acknowledgements, err = reconcileAcknowledgementsTx(ctx, tx, acknowledgements); err != nil {
		return EditorialCounts{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EditorialCounts{}, fmt.Errorf("postgres: committing editorial reconcile: %w", err)
	}
	return counts, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const seriesBreakColumns = `break_key, scope_kind, scope_ref, date, kind, note_md, source_url, config_digest, retired_at`

func scanSeriesBreak(row interface{ Scan(...any) error }) (SeriesBreak, error) {
	var sb SeriesBreak
	if err := row.Scan(&sb.BreakKey, &sb.ScopeKind, &sb.ScopeRef, &sb.Date, &sb.Kind, &sb.NoteMD, &sb.SourceURL, &sb.ConfigDigest, &sb.RetiredAt); err != nil {
		return SeriesBreak{}, err
	}
	return sb, nil
}

// allSeriesBreaks returns EVERY series_break row, retired or not -- the
// full state reconcileBreaksTx needs to diff against.
func allSeriesBreaks(ctx context.Context, tx DBTX) (map[seriesBreakKey]SeriesBreak, error) {
	rows, err := tx.Query(ctx, `SELECT `+seriesBreakColumns+` FROM series_break`)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing series_break rows: %w", err)
	}
	defer rows.Close()

	out := map[seriesBreakKey]SeriesBreak{}
	for rows.Next() {
		sb, err := scanSeriesBreak(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scanning series_break row: %w", err)
		}
		out[seriesBreakKey{sb.BreakKey, sb.ScopeKind, sb.ScopeRef}] = sb
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating series_break rows: %w", err)
	}
	return out, nil
}

// ListSeriesBreaks returns every series_break row (retired included) --
// the read side used by tests and by any future audit/admin surface.
func ListSeriesBreaks(ctx context.Context, db DBTX) ([]SeriesBreak, error) {
	m, err := allSeriesBreaks(ctx, db)
	if err != nil {
		return nil, err
	}
	out := make([]SeriesBreak, 0, len(m))
	for _, sb := range m {
		out = append(out, sb)
	}
	return out, nil
}

// ResolveActiveBreaksForSeries expands scope (spec "A family-scoped
// break applies to every member": series ⊂ dataset ⊂ source) into every
// currently-active series_break that applies to seriesID -- WITHOUT the
// break ever being duplicated in storage: a single dataset- or
// source-scoped row resolves for every series under it (spec "it is
// stored once, not once per series").
func ResolveActiveBreaksForSeries(ctx context.Context, db DBTX, seriesID string) ([]SeriesBreak, error) {
	var datasetID, sourceID string
	row := db.QueryRow(ctx, `SELECT d.id, d.source_id FROM series s JOIN dataset d ON d.id = s.dataset_id WHERE s.id=$1`, seriesID)
	if err := row.Scan(&datasetID, &sourceID); err != nil {
		return nil, fmt.Errorf("postgres: resolving dataset/source for series %q: %w", seriesID, err)
	}

	rows, err := db.Query(ctx, `SELECT `+seriesBreakColumns+`
		FROM series_break
		WHERE retired_at IS NULL AND (
			(scope_kind='series'  AND scope_ref=$1) OR
			(scope_kind='dataset' AND scope_ref=$2) OR
			(scope_kind='source'  AND scope_ref=$3)
		)
		ORDER BY date`, seriesID, datasetID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("postgres: resolving active breaks for series %q: %w", seriesID, err)
	}
	defer rows.Close()

	var out []SeriesBreak
	for rows.Next() {
		sb, err := scanSeriesBreak(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scanning resolved break for series %q: %w", seriesID, err)
		}
		out = append(out, sb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating resolved breaks for series %q: %w", seriesID, err)
	}
	return out, nil
}

// EventInput is one already-digested eventos.yaml/gobiernos.yaml entry
// ready to reconcile.
//
// ScopeKind/ScopeRef and SourceURL are migration 0007's columns. Every
// entry the registry holds today is scoped 'global' -- a change of
// government and a worldwide shock apply wherever the calendar does -- but
// 'global' is now a value the registry STATES rather than a property of the
// schema, which is what closed the gap ListActiveEvents used to disclose in
// its own doc comment (a seriesID accepted and not read).
type EventInput struct {
	ID           string
	Group        string
	Name         string
	DateStart    time.Time
	DateEnd      *time.Time
	NoteMD       string
	ScopeKind    string // global | series | dataset | source
	ScopeRef     string // id within ScopeKind; empty when global
	SourceURL    string
	ConfigDigest string
}

// Event is one persisted event row.
type Event struct {
	ID           string
	Group        string
	Name         string
	DateStart    time.Time
	DateEnd      *time.Time
	NoteMD       *string
	ScopeKind    string
	ScopeRef     string
	SourceURL    *string
	ConfigDigest string
	RetiredAt    *time.Time
}

const eventColumns = `id, event_group, name, date_start, date_end, note_md, scope_kind, scope_ref, source_url, config_digest, retired_at`

func scanEvent(row interface{ Scan(...any) error }) (Event, error) {
	var ev Event
	if err := row.Scan(&ev.ID, &ev.Group, &ev.Name, &ev.DateStart, &ev.DateEnd, &ev.NoteMD,
		&ev.ScopeKind, &ev.ScopeRef, &ev.SourceURL, &ev.ConfigDigest, &ev.RetiredAt); err != nil {
		return Event{}, err
	}
	return ev, nil
}

// ReconcileEvents reconciles desired against event, applying the exact
// same insert/update-in-place/soft-retire discipline as ReconcileBreaks
// (this file's package doc comment) -- event's natural key is its bare
// id (migration 0001: `id text PRIMARY KEY`), so no scope tuple is
// needed here.
func ReconcileEvents(ctx context.Context, db TxBeginner, desired []EventInput) (ReconcileCounts, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return ReconcileCounts{}, fmt.Errorf("postgres: beginning event reconcile: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	counts, err := reconcileEventsTx(ctx, tx, desired)
	if err != nil {
		return ReconcileCounts{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconcileCounts{}, fmt.Errorf("postgres: committing event reconcile: %w", err)
	}
	return counts, nil
}

func reconcileEventsTx(ctx context.Context, tx DBTX, desired []EventInput) (ReconcileCounts, error) {
	live, err := allEvents(ctx, tx)
	if err != nil {
		return ReconcileCounts{}, err
	}

	var counts ReconcileCounts
	seen := map[string]bool{}
	for _, in := range desired {
		if seen[in.ID] {
			return ReconcileCounts{}, fmt.Errorf("postgres: reconciling event: duplicate id %q in one reconcile batch", in.ID)
		}
		seen[in.ID] = true

		current, exists := live[in.ID]
		switch {
		case !exists:
			if _, err := tx.Exec(ctx, `INSERT INTO event (id, event_group, name, date_start, date_end, note_md, scope_kind, scope_ref, source_url, config_digest)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				in.ID, in.Group, in.Name, in.DateStart, in.DateEnd, nullableString(in.NoteMD),
				in.ScopeKind, in.ScopeRef, nullableString(in.SourceURL), in.ConfigDigest); err != nil {
				return ReconcileCounts{}, fmt.Errorf("postgres: inserting event %q: %w", in.ID, err)
			}
			counts.Inserted++
		case current.ConfigDigest != in.ConfigDigest || current.RetiredAt != nil:
			if _, err := tx.Exec(ctx, `UPDATE event SET event_group=$2, name=$3, date_start=$4, date_end=$5, note_md=$6,
				scope_kind=$7, scope_ref=$8, source_url=$9, config_digest=$10, retired_at=NULL
				WHERE id=$1`,
				in.ID, in.Group, in.Name, in.DateStart, in.DateEnd, nullableString(in.NoteMD),
				in.ScopeKind, in.ScopeRef, nullableString(in.SourceURL), in.ConfigDigest); err != nil {
				return ReconcileCounts{}, fmt.Errorf("postgres: updating event %q: %w", in.ID, err)
			}
			counts.Updated++
		}
	}

	for id, current := range live {
		if seen[id] || current.RetiredAt != nil {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE event SET retired_at=now() WHERE id=$1`, id); err != nil {
			return ReconcileCounts{}, fmt.Errorf("postgres: retiring event %q: %w", id, err)
		}
		counts.Retired++
	}
	return counts, nil
}

func allEvents(ctx context.Context, tx DBTX) (map[string]Event, error) {
	rows, err := tx.Query(ctx, `SELECT `+eventColumns+` FROM event`)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing event rows: %w", err)
	}
	defer rows.Close()

	out := map[string]Event{}
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scanning event row: %w", err)
		}
		out[ev.ID] = ev
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating event rows: %w", err)
	}
	return out, nil
}

// ListEvents returns every event row (retired included).
func ListEvents(ctx context.Context, db DBTX) ([]Event, error) {
	m, err := allEvents(ctx, db)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(m))
	for _, ev := range m {
		out = append(out, ev)
	}
	return out, nil
}
