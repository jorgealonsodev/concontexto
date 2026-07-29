package postgres

// Task 2.11 (GREEN): provenance routes through the run —
// observation.ingestion_run_id -> ingestion_run.raw_file_hash ->
// raw_file(hash) — so every published observation resolves to the exact
// bytes it came from (spec data-model-vintages, "Run-level vintage
// separated from per-period version"; principle P2 made queryable).

import (
	"context"
	"fmt"
	"time"
)

// Provenance is the resolved provenance chain for one observation: which
// source it came from, under what origin reference, when it was
// extracted, which run produced it, and which exact raw file bytes back
// it — every field non-null per spec.
type Provenance struct {
	SourceID       string
	OriginRef      string
	ExtractedAt    time.Time
	IngestionRunID int64
	RawFileHash    string
}

// ResolveProvenance resolves the CURRENT observation's full provenance
// chain: observation -> ingestion_run -> raw_file -> source, joined
// with the series_source_mapping active at extraction time for the
// origin reference (spec "Provenance is traceable from an observation
// to a raw file").
func ResolveProvenance(ctx context.Context, db DBTX, seriesID, period string) (Provenance, error) {
	row := db.QueryRow(ctx, `
		SELECT s.id, m.ref, o.extracted_at, o.ingestion_run_id, r.hash
		FROM observation o
		JOIN ingestion_run ir ON ir.id = o.ingestion_run_id
		JOIN raw_file r ON r.hash = ir.raw_file_hash
		JOIN source s ON s.id = r.source_id
		JOIN series_source_mapping m ON m.series_id = o.series_id
			AND m.valid_from <= o.extracted_at::date
			AND (m.valid_to IS NULL OR m.valid_to > o.extracted_at::date)
		WHERE o.series_id = $1 AND o.period = $2 AND o.is_current`, seriesID, period)

	var p Provenance
	if err := row.Scan(&p.SourceID, &p.OriginRef, &p.ExtractedAt, &p.IngestionRunID, &p.RawFileHash); err != nil {
		return Provenance{}, fmt.Errorf("postgres: resolving provenance for %s/%s: %w", seriesID, period, err)
	}
	return p, nil
}

// VintageAsOf resolves the observation that would have been served on
// date asOf: the maximum version among runs started at or before asOf
// (spec "Vintage as-of a date resolves to the right version"; design.md
// "Frozen view at date D" — never executed at request time, pre-render
// only; this function is the pre-render-time query it describes).
func VintageAsOf(ctx context.Context, db DBTX, seriesID, period string, asOf time.Time) (Observation, error) {
	row := db.QueryRow(ctx, `
		SELECT `+observationColumnsQualified+`
		FROM observation o
		JOIN ingestion_run ir ON ir.id = o.ingestion_run_id
		WHERE o.series_id = $1 AND o.period = $2 AND ir.started_at <= $3
		ORDER BY o.version DESC
		LIMIT 1`, seriesID, period, asOf)

	obs, err := scanObservation(row)
	if err != nil {
		return Observation{}, fmt.Errorf("postgres: resolving vintage as of %s for %s/%s: %w", asOf, seriesID, period, err)
	}
	return obs, nil
}
