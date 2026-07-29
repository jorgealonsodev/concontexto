package postgres

// Task 2.7/2.9/2.15/2.17 (GREEN): the append-only observation writer.
// Owns the version chain and the is_current promotion invariant (ADR-4,
// design.md "postgres.ObservationWriter [Tx2, atomic]"): flip prior
// current -> false + superseded_at; INSERT version = prior+1; new row
// is_current = true — in the SAME transaction, so the one_current_row
// partial unique index can never observe two current rows even
// transiently. Withdrawal (spec "Source withdrawal is representable")
// reuses this exact path with Value=nil, Status=StatusWithdrawn: no
// special case needed, the schema's CHECK constraint already only
// allows a NULL value for status='W'. Rollback (spec "Bad-run rollback
// never deletes") re-points is_current back to the prior version and
// records why, still never deleting a row or touching raw_file.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// ObservationStatus mirrors observation.status: provisional, definitive
// or withdrawn (design.md schema).
type ObservationStatus string

const (
	StatusProvisional ObservationStatus = "P"
	StatusDefinitive  ObservationStatus = "D"
	StatusWithdrawn   ObservationStatus = "W"
)

// ObservationInput is the domain-facing input for writing one observed
// value. It carries no pgx types — Clean/Hexagonal: the domain stays
// free of the driver, only this adapter file does.
type ObservationInput struct {
	SeriesID       string
	Period         string
	Value          *float64 // nil only valid when Status == StatusWithdrawn
	Status         ObservationStatus
	ExtractedAt    time.Time
	IngestionRunID int64
}

// Observation is one persisted (series_id, period, version) row.
type Observation struct {
	SeriesID       string
	Period         string
	Version        int
	Value          *float64
	Status         ObservationStatus
	ExtractedAt    time.Time
	IngestionRunID int64
	IsCurrent      bool
	SupersededAt   *time.Time
	RollbackReason *string
}

// observationColumnsUnqualified is used by INSERT/UPDATE ... RETURNING,
// where no table alias exists.
const observationColumnsUnqualified = `series_id, period, version, value, status, extracted_at, ingestion_run_id, is_current, superseded_at, rollback_reason`

// observationColumnsQualified is used by SELECT queries that alias
// observation as "o" (required once a query joins another table that
// also has a series_id/period-shaped column, e.g. ingestion_run).
const observationColumnsQualified = `o.series_id, o.period, o.version, o.value, o.status, o.extracted_at, o.ingestion_run_id, o.is_current, o.superseded_at, o.rollback_reason`

func scanObservation(row pgx.Row) (Observation, error) {
	var obs Observation
	var status string
	if err := row.Scan(&obs.SeriesID, &obs.Period, &obs.Version, &obs.Value, &status,
		&obs.ExtractedAt, &obs.IngestionRunID, &obs.IsCurrent, &obs.SupersededAt, &obs.RollbackReason); err != nil {
		return Observation{}, err
	}
	obs.Status = ObservationStatus(status)
	return obs, nil
}

// currentObservation returns the current row for (seriesID, period), or
// found=false if none exists yet.
func currentObservation(ctx context.Context, db DBTX, seriesID, period string) (Observation, bool, error) {
	row := db.QueryRow(ctx, `SELECT `+observationColumnsQualified+`
		FROM observation o WHERE o.series_id=$1 AND o.period=$2 AND o.is_current`, seriesID, period)
	obs, err := scanObservation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Observation{}, false, nil
	}
	if err != nil {
		return Observation{}, false, fmt.Errorf("postgres: reading current observation for %s/%s: %w", seriesID, period, err)
	}
	return obs, true, nil
}

// ListCurrentObservations returns the FULL current vintage for seriesID,
// projected to the pure indicators.Observation validation rules consume
// (design.md "Prior []indicators.Observation // current vintage before
// this run") -- unlike currentObservation, which resolves exactly one
// (series, period), this is every current row a rule needs to check
// continuity/plausibility/revision against. The stored period text is
// already the canonical label (Period.String()'s own format, e.g.
// "2026-Q2"), so indicators.NormalizePeriodLabel -- which already
// recognises that exact shape as Eurostat's native one -- parses it back
// with no separate "database period" parser needed.
func ListCurrentObservations(ctx context.Context, db DBTX, seriesID string) ([]indicators.Observation, error) {
	rows, err := db.Query(ctx, `SELECT period, value FROM observation WHERE series_id=$1 AND is_current`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing current observations for %s: %w", seriesID, err)
	}
	defer rows.Close()

	var out []indicators.Observation
	for rows.Next() {
		var periodText string
		var value *float64
		if err := rows.Scan(&periodText, &value); err != nil {
			return nil, fmt.Errorf("postgres: scanning a current observation row for %s: %w", seriesID, err)
		}
		period, err := indicators.NormalizePeriodLabel(periodText)
		if err != nil {
			return nil, fmt.Errorf("postgres: parsing stored period %q for %s: %w", periodText, seriesID, err)
		}
		out = append(out, indicators.Observation{Period: period, Value: value})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating current observations for %s: %w", seriesID, err)
	}
	return out, nil
}

func sameValue(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ObservationWriter is the driven adapter that owns the append-only
// version chain and the is_current promotion invariant.
type ObservationWriter struct {
	db TxBeginner
}

// NewObservationWriter builds an ObservationWriter against db. db may be
// the production *pgxpool.Pool or a test's pgx.Tx (see TxBeginner).
func NewObservationWriter(db TxBeginner) *ObservationWriter {
	return &ObservationWriter{db: db}
}

// WriteRevision appends a new version for (in.SeriesID, in.Period) when
// the incoming value or status differs from the current one, flipping
// is_current in the same transaction as the insert (spec "Observations
// are immutable per version", "The database enforces exactly one
// current row"). Resubmitting an identical value/status creates no new
// row (spec "Unchanged value does not create a new version") — a
// vintage is a record of the source changing its mind, not of a re-run.
func (w *ObservationWriter) WriteRevision(ctx context.Context, in ObservationInput) (Observation, bool, error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return Observation{}, false, fmt.Errorf("postgres: beginning observation write: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, found, err := currentObservation(ctx, tx, in.SeriesID, in.Period)
	if err != nil {
		return Observation{}, false, err
	}
	if found && sameValue(current.Value, in.Value) && current.Status == in.Status {
		if err := tx.Commit(ctx); err != nil {
			return Observation{}, false, fmt.Errorf("postgres: committing no-op revision: %w", err)
		}
		return current, false, nil
	}

	nextVersion := 1
	if found {
		nextVersion = current.Version + 1
		if _, err := tx.Exec(ctx, `UPDATE observation SET is_current=false, superseded_at=now()
			WHERE series_id=$1 AND period=$2 AND version=$3`, in.SeriesID, in.Period, current.Version); err != nil {
			return Observation{}, false, fmt.Errorf("postgres: demoting prior current observation for %s/%s: %w", in.SeriesID, in.Period, err)
		}
	}

	row := tx.QueryRow(ctx, `INSERT INTO observation (series_id, period, version, value, status, extracted_at, ingestion_run_id, is_current)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true)
		RETURNING `+observationColumnsUnqualified,
		in.SeriesID, in.Period, nextVersion, in.Value, string(in.Status), in.ExtractedAt, in.IngestionRunID)
	obs, err := scanObservation(row)
	if err != nil {
		return Observation{}, false, fmt.Errorf("postgres: inserting new observation version for %s/%s: %w", in.SeriesID, in.Period, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Observation{}, false, fmt.Errorf("postgres: committing observation revision for %s/%s: %w", in.SeriesID, in.Period, err)
	}
	return obs, true, nil
}

// ErrNoPriorVersion is returned by RollbackRun when the row being rolled
// back has no earlier version to restore.
var ErrNoPriorVersion = errors.New("postgres: no prior version to roll back to")

// RollbackRun re-points is_current back to the prior version for every
// (series_id, period) whose current row was produced by ingestionRunID,
// records reason on the demoted row, and never deletes an observation or
// touches raw_file/ingestion_run (spec "Bad-run rollback never
// deletes", principle P7).
func (w *ObservationWriter) RollbackRun(ctx context.Context, ingestionRunID int64, reason string) ([]Observation, error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: beginning rollback of run %d: %w", ingestionRunID, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT series_id, period, version FROM observation
		WHERE ingestion_run_id=$1 AND is_current`, ingestionRunID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing current rows for run %d: %w", ingestionRunID, err)
	}
	type target struct {
		seriesID, period string
		version          int
	}
	var targets []target
	for rows.Next() {
		var tg target
		if err := rows.Scan(&tg.seriesID, &tg.period, &tg.version); err != nil {
			rows.Close()
			return nil, fmt.Errorf("postgres: scanning rollback target for run %d: %w", ingestionRunID, err)
		}
		targets = append(targets, tg)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("postgres: iterating rollback targets for run %d: %w", ingestionRunID, err)
	}
	rows.Close()

	restored := make([]Observation, 0, len(targets))
	for _, tg := range targets {
		priorVersion := tg.version - 1
		if priorVersion < 1 {
			return nil, fmt.Errorf("%w: %s/%s version %d", ErrNoPriorVersion, tg.seriesID, tg.period, tg.version)
		}
		if _, err := tx.Exec(ctx, `UPDATE observation SET is_current=false, superseded_at=now(), rollback_reason=$1
			WHERE series_id=$2 AND period=$3 AND version=$4`, reason, tg.seriesID, tg.period, tg.version); err != nil {
			return nil, fmt.Errorf("postgres: recording rollback on %s/%s v%d: %w", tg.seriesID, tg.period, tg.version, err)
		}
		row := tx.QueryRow(ctx, `UPDATE observation SET is_current=true, superseded_at=NULL
			WHERE series_id=$1 AND period=$2 AND version=$3
			RETURNING `+observationColumnsUnqualified, tg.seriesID, tg.period, priorVersion)
		obs, err := scanObservation(row)
		if err != nil {
			return nil, fmt.Errorf("postgres: restoring prior version %s/%s v%d: %w", tg.seriesID, tg.period, priorVersion, err)
		}
		restored = append(restored, obs)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("postgres: committing rollback of run %d: %w", ingestionRunID, err)
	}
	return restored, nil
}
