package postgres

// Task 3.12 (RED) / 3.13 (GREEN): attribution resolution (spec
// source-attribution-licensing, "Attribution chains from the original
// source"). Reuses the exact same join ResolveProvenance already proves
// (observation -> ingestion_run -> raw_file -> source, joined with the
// series_source_mapping active at extraction time) so the chain "must
// resolve without manual lookup" is a single query, not a composition of
// several round trips that could disagree with each other.

import (
	"context"
	"fmt"
	"time"
)

// Attribution is what a published value's attribution resolves to: the
// originating source's configured attribution text, the origin series
// identifier it was extracted under, and when it was extracted (spec
// "it yields the configured attribution text of its originating source
// AND the origin series identifier and extraction timestamp accompany
// it").
type Attribution struct {
	AttributionText string
	OriginRef       string
	ExtractedAt     time.Time
}

// ResolveAttribution resolves the CURRENT observation's attribution for
// (seriesID, period). It errors if there is no current observation for
// that period — attribution cannot be derived for a value that was
// never published (or was withdrawn to a tombstone with no matching
// mapping, an edge case Phase 4's publish gate governs).
func ResolveAttribution(ctx context.Context, db DBTX, seriesID, period string) (Attribution, error) {
	row := db.QueryRow(ctx, `
		SELECT s.attribution_text, m.ref, o.extracted_at
		FROM observation o
		JOIN ingestion_run ir ON ir.id = o.ingestion_run_id
		JOIN raw_file r ON r.hash = ir.raw_file_hash
		JOIN source s ON s.id = r.source_id
		JOIN series_source_mapping m ON m.series_id = o.series_id
			AND m.valid_from <= o.extracted_at::date
			AND (m.valid_to IS NULL OR m.valid_to > o.extracted_at::date)
		WHERE o.series_id = $1 AND o.period = $2 AND o.is_current`, seriesID, period)

	var a Attribution
	if err := row.Scan(&a.AttributionText, &a.OriginRef, &a.ExtractedAt); err != nil {
		return Attribution{}, fmt.Errorf("postgres: resolving attribution for %s/%s: %w", seriesID, period, err)
	}
	return a, nil
}
