package postgres

// The acknowledgement registry's DATABASE half (spec data-validation,
// "Acknowledged findings"): validation_acknowledgement is a projection of
// config/reconocimientos.yaml, reconciled by
// ingestion.ReconcileEditorialConfig in the SAME transaction as
// series_break and event.
//
// ReconcileAcknowledgements applies the identical insert / update-in-place
// / soft-retire / zero-writes-on-an-identical-rerun discipline
// editorial.go's package doc comment describes for the other two
// registries, and for the same reasons. It is deliberately not a new
// pattern: an approvals registry is exactly the kind of record that must
// be soft-retired rather than deleted (principle P7), and an operator who
// has learned how rupturas.yaml reconciles already knows how this one
// does.
//
// The READ side (ResolveActiveAcknowledgementsForSeries) is the mirror of
// ResolveActiveBreaksForSeries, minus the scope expansion — and the
// absence of that expansion is the point. A break resolves for a whole
// dataset or source because a methodology change genuinely applies to
// every series under it. An acknowledgement is one human's statement
// about one number, so it resolves for exactly the series it names and
// nothing wider.

import (
	"context"
	"fmt"
	"time"
)

// AcknowledgementInput is one already-digested reconocimientos.yaml entry
// ready to reconcile (ingestion.ReconcileEditorialConfig computes
// ConfigDigest from the source YAML).
type AcknowledgementInput struct {
	AckKey         string
	SeriesID       string
	Period         string
	Rule           string
	Value          float64
	AcknowledgedBy string
	AcknowledgedOn time.Time
	NoteMD         string
	SourceURL      string
	ConfigDigest   string
}

// Acknowledgement is one persisted validation_acknowledgement row.
type Acknowledgement struct {
	AckKey         string
	SeriesID       string
	Period         string
	Rule           string
	Value          float64
	AcknowledgedBy string
	AcknowledgedOn time.Time
	NoteMD         string
	SourceURL      *string
	ConfigDigest   string
	RetiredAt      *time.Time
}

// ReconcileAcknowledgements reconciles desired against
// validation_acknowledgement in its own transaction. Production goes
// through ReconcileEditorial instead, which shares ONE transaction across
// all three editorial tables; this entry point exists for the same reason
// ReconcileBreaks/ReconcileEvents do, so the per-table discipline is
// directly testable on its own.
func ReconcileAcknowledgements(ctx context.Context, db TxBeginner, desired []AcknowledgementInput) (ReconcileCounts, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return ReconcileCounts{}, fmt.Errorf("postgres: beginning acknowledgement reconcile: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	counts, err := reconcileAcknowledgementsTx(ctx, tx, desired)
	if err != nil {
		return ReconcileCounts{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconcileCounts{}, fmt.Errorf("postgres: committing acknowledgement reconcile: %w", err)
	}
	return counts, nil
}

func reconcileAcknowledgementsTx(ctx context.Context, tx DBTX, desired []AcknowledgementInput) (ReconcileCounts, error) {
	live, err := allAcknowledgements(ctx, tx)
	if err != nil {
		return ReconcileCounts{}, err
	}

	var counts ReconcileCounts
	seen := map[string]bool{}
	for _, in := range desired {
		if seen[in.AckKey] {
			return ReconcileCounts{}, fmt.Errorf("postgres: reconciling validation_acknowledgement: duplicate ack_key %q in one reconcile batch", in.AckKey)
		}
		seen[in.AckKey] = true

		current, exists := live[in.AckKey]
		switch {
		case !exists:
			if _, err := tx.Exec(ctx, `INSERT INTO validation_acknowledgement
				(ack_key, series_id, period, rule, acknowledged_value, acknowledged_by, acknowledged_on, note_md, source_url, config_digest)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				in.AckKey, in.SeriesID, in.Period, in.Rule, in.Value, in.AcknowledgedBy, in.AcknowledgedOn,
				in.NoteMD, nullableString(in.SourceURL), in.ConfigDigest); err != nil {
				return ReconcileCounts{}, fmt.Errorf("postgres: inserting validation_acknowledgement %q: %w", in.AckKey, err)
			}
			counts.Inserted++
		case current.ConfigDigest != in.ConfigDigest || current.RetiredAt != nil:
			if _, err := tx.Exec(ctx, `UPDATE validation_acknowledgement
				SET series_id=$2, period=$3, rule=$4, acknowledged_value=$5, acknowledged_by=$6, acknowledged_on=$7,
				    note_md=$8, source_url=$9, config_digest=$10, retired_at=NULL
				WHERE ack_key=$1`,
				in.AckKey, in.SeriesID, in.Period, in.Rule, in.Value, in.AcknowledgedBy, in.AcknowledgedOn,
				in.NoteMD, nullableString(in.SourceURL), in.ConfigDigest); err != nil {
				return ReconcileCounts{}, fmt.Errorf("postgres: updating validation_acknowledgement %q: %w", in.AckKey, err)
			}
			counts.Updated++
		}
	}

	for key, current := range live {
		if seen[key] || current.RetiredAt != nil {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE validation_acknowledgement SET retired_at=now() WHERE ack_key=$1`, key); err != nil {
			return ReconcileCounts{}, fmt.Errorf("postgres: retiring validation_acknowledgement %q: %w", key, err)
		}
		counts.Retired++
	}
	return counts, nil
}

const acknowledgementColumns = `ack_key, series_id, period, rule, acknowledged_value, acknowledged_by, acknowledged_on, note_md, source_url, config_digest, retired_at`

func scanAcknowledgement(row interface{ Scan(...any) error }) (Acknowledgement, error) {
	var a Acknowledgement
	if err := row.Scan(&a.AckKey, &a.SeriesID, &a.Period, &a.Rule, &a.Value, &a.AcknowledgedBy,
		&a.AcknowledgedOn, &a.NoteMD, &a.SourceURL, &a.ConfigDigest, &a.RetiredAt); err != nil {
		return Acknowledgement{}, err
	}
	return a, nil
}

// allAcknowledgements returns EVERY row, retired or not — the full state
// reconcileAcknowledgementsTx needs to diff against.
func allAcknowledgements(ctx context.Context, tx DBTX) (map[string]Acknowledgement, error) {
	rows, err := tx.Query(ctx, `SELECT `+acknowledgementColumns+` FROM validation_acknowledgement`)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing validation_acknowledgement rows: %w", err)
	}
	defer rows.Close()

	out := map[string]Acknowledgement{}
	for rows.Next() {
		a, err := scanAcknowledgement(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scanning validation_acknowledgement row: %w", err)
		}
		out[a.AckKey] = a
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating validation_acknowledgement rows: %w", err)
	}
	return out, nil
}

// ListAcknowledgements returns every row (retired included) — the read
// side used by tests and by any future audit surface, mirroring
// ListSeriesBreaks.
func ListAcknowledgements(ctx context.Context, db DBTX) ([]Acknowledgement, error) {
	m, err := allAcknowledgements(ctx, db)
	if err != nil {
		return nil, err
	}
	out := make([]Acknowledgement, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	return out, nil
}

// ResolveActiveAcknowledgementsForSeries returns every currently-active
// acknowledgement scoped to exactly seriesID.
//
// Note what is NOT here: no scope expansion. ResolveActiveBreaksForSeries
// widens a break across series ⊂ dataset ⊂ source because a methodology
// change really does apply to every series under it. An acknowledgement is
// one person's statement about one number in one series, so widening it
// would be inventing consent nobody gave. The `series_id = $1` equality
// below is that principle in its entirety.
func ResolveActiveAcknowledgementsForSeries(ctx context.Context, db DBTX, seriesID string) ([]Acknowledgement, error) {
	rows, err := db.Query(ctx, `SELECT `+acknowledgementColumns+`
		FROM validation_acknowledgement
		WHERE retired_at IS NULL AND series_id = $1
		ORDER BY period, rule`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("postgres: resolving active acknowledgements for series %q: %w", seriesID, err)
	}
	defer rows.Close()

	var out []Acknowledgement
	for rows.Next() {
		a, err := scanAcknowledgement(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scanning resolved acknowledgement for series %q: %w", seriesID, err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating resolved acknowledgements for series %q: %w", seriesID, err)
	}
	return out, nil
}
