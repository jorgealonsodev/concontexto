package postgres

// Task 2.13 (GREEN): series_source_mapping's append-only churn — an
// identifier change closes the old mapping (valid_to) and inserts a
// replacement, never overwriting the origin reference (spec
// data-model-vintages, "Source identifiers are validity-ranged
// mappings", verified risk R1: both table Ids and series CODs churn).

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SourceMapping is one series_source_mapping row.
type SourceMapping struct {
	ID           int64
	SeriesID     string
	RefKind      string
	Ref          string
	ValidFrom    time.Time
	ValidTo      *time.Time
	ConfigDigest string
	RetiredAt    *time.Time
}

// SourceMappingInput is the domain-facing input for a new mapping row.
type SourceMappingInput struct {
	SeriesID     string
	RefKind      string
	Ref          string
	ValidFrom    time.Time
	ConfigDigest string
}

const sourceMappingColumns = `id, series_id, ref_kind, ref, valid_from, valid_to, config_digest, retired_at`

func scanSourceMapping(row pgx.Row) (SourceMapping, error) {
	var m SourceMapping
	if err := row.Scan(&m.ID, &m.SeriesID, &m.RefKind, &m.Ref, &m.ValidFrom, &m.ValidTo, &m.ConfigDigest, &m.RetiredAt); err != nil {
		return SourceMapping{}, err
	}
	return m, nil
}

// SourceMappingRepo owns series_source_mapping writes.
type SourceMappingRepo struct {
	db TxBeginner
}

// NewSourceMappingRepo builds a SourceMappingRepo against db.
func NewSourceMappingRepo(db TxBeginner) *SourceMappingRepo {
	return &SourceMappingRepo{db: db}
}

// ReplaceActiveMapping closes the current active mapping for seriesID
// (setting valid_to = closedValidTo) and inserts next as the new active
// mapping, in one transaction (spec "An identifier change adds a
// mapping instead of replacing one").
func (r *SourceMappingRepo) ReplaceActiveMapping(ctx context.Context, seriesID string, closedValidTo time.Time, next SourceMappingInput) (SourceMapping, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return SourceMapping{}, fmt.Errorf("postgres: beginning mapping replacement for %s: %w", seriesID, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE series_source_mapping SET valid_to=$1
		WHERE series_id=$2 AND valid_to IS NULL AND retired_at IS NULL`, closedValidTo, seriesID); err != nil {
		return SourceMapping{}, fmt.Errorf("postgres: closing active mapping for %s: %w", seriesID, err)
	}

	row := tx.QueryRow(ctx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+sourceMappingColumns,
		next.SeriesID, next.RefKind, next.Ref, next.ValidFrom, next.ConfigDigest)
	m, err := scanSourceMapping(row)
	if err != nil {
		return SourceMapping{}, fmt.Errorf("postgres: inserting replacement mapping for %s: %w", seriesID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return SourceMapping{}, fmt.Errorf("postgres: committing mapping replacement for %s: %w", seriesID, err)
	}
	return m, nil
}

// ListMappings returns every mapping row for seriesID (both closed and
// active), ordered by valid_from — the query that proves a closed
// mapping and its replacement both remain queryable.
func ListMappings(ctx context.Context, db DBTX, seriesID string) ([]SourceMapping, error) {
	rows, err := db.Query(ctx, `SELECT `+sourceMappingColumns+`
		FROM series_source_mapping WHERE series_id=$1 ORDER BY valid_from`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing mappings for %s: %w", seriesID, err)
	}
	defer rows.Close()

	var out []SourceMapping
	for rows.Next() {
		var m SourceMapping
		if err := rows.Scan(&m.ID, &m.SeriesID, &m.RefKind, &m.Ref, &m.ValidFrom, &m.ValidTo, &m.ConfigDigest, &m.RetiredAt); err != nil {
			return nil, fmt.Errorf("postgres: scanning mapping row for %s: %w", seriesID, err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating mappings for %s: %w", seriesID, err)
	}
	return out, nil
}
