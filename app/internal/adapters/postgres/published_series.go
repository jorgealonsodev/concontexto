package postgres

// Task 3.10 (RED/GREEN): the two new read ports app/internal/publishing
// needs that no existing function supplies (design.md D-2's own read
// list plus D1's resolution, task 3.1). ListPublishedSeries answers "who
// is this series and where does its data come from" once per series;
// ListPublishedObservations answers "what is currently published, and
// exactly which raw bytes back each value" once per series, joined in
// one query so publishing.Export needs no per-observation round trip
// (spec publishing-export, "Provenance survives the export ... every
// one of those fields is reachable from the artifact without a further
// database query").

import (
	"context"
	"fmt"
	"time"
)

// PublishedSeries is one series' identity and metadata, joined with its
// currently active source_source_mapping (design D-2's read list).
//
// DatasetID stands in for the spec's "statistical operation" field
// (publishing-export, "series metadata (source, statistical operation,
// origin series identifier ...)"): Fase 0/1's config carries no
// dedicated descriptive field for a series' statistical operation
// (config.SeriesConfig/DatasetConfig have no such field, and
// reconcileDataset -- dimensions.go -- sets dataset.name literally equal
// to the dataset id). Using the dataset id here is an honest existing
// fact, not a fabricated description; see design.md's Open Questions.
//
// SourceURL is the source's general website (source.url); SourceLicenceURL
// is the distinct licence-terms URL (source.licence_url, migration 0004 --
// closes a slice-3 disclosed gap: before that migration, this schema had
// no column to persist config.LicenceConfig.URL, so the artifact's
// licenceUrl fell back to SourceURL as an imprecise stand-in. May be empty
// for a row reconciled before migration 0004 shipped, until its next
// ingest cycle re-reconciles it -- no bespoke backfill, the same
// self-heals-on-next-ingest convention D-3's backfill note established).
type PublishedSeries struct {
	Slug       string
	Name       string
	Unit       string
	Frequency  string
	Decimals   int
	Geo        string
	Harmonized bool

	DatasetID string

	SourceID          string
	SourceName        string
	SourceAttribution string
	SourceLicenceName string
	SourceURL         string
	SourceLicenceURL  string

	OriginKind string
	OriginRef  string

	// DiscontinuedSince and DiscontinuedSuccessorSlug carry the editorial
	// discontinuation (migration 0005, reconciled from
	// config.SeriesConfig.Discontinued) that drives one of PRD §6.1.3's
	// three page states. Both nil is the ordinary live case -- every
	// series configured today.
	//
	// Discontinued is NOT retired. retired_at removes a series from this
	// function's result entirely; a discontinued series stays in it,
	// carrying its full history, because the spec is explicit that "the
	// chart MUST NOT be hidden in any state" -- the source stopped
	// publishing new periods, it did not un-publish the old ones.
	//
	// DiscontinuedSuccessorSlug is nil when no successor is configured
	// (spec: "a successor link is shown when a successor is configured"),
	// never an empty string: "no successor exists" and "a successor
	// exists whose slug is blank" are different claims and only one of
	// them is ever true.
	DiscontinuedSince         *time.Time
	DiscontinuedSuccessorSlug *string
}

// ListPublishedSeries returns every non-retired series with a currently
// active source_source_mapping, joined with its dataset and source. It
// does NOT filter on whether the series has ever actually published an
// observation -- publishing.Export does that by checking
// ListPublishedObservations' own result, keeping "who is configured" and
// "who has data" as two separate, independently testable facts.
func ListPublishedSeries(ctx context.Context, db DBTX) ([]PublishedSeries, error) {
	rows, err := db.Query(ctx, `
		SELECT s.id, s.name, s.unit, s.frequency, s.decimals, s.geo, s.is_harmonized,
		       s.discontinued_since, s.discontinued_successor_slug,
		       d.id,
		       src.id, src.name, src.attribution_text, src.license, src.url, src.licence_url,
		       m.ref_kind, m.ref
		FROM series s
		JOIN dataset d ON d.id = s.dataset_id
		JOIN source src ON src.id = d.source_id
		JOIN series_source_mapping m ON m.series_id = s.id AND m.valid_to IS NULL AND m.retired_at IS NULL
		WHERE s.retired_at IS NULL
		ORDER BY s.id`)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing published series: %w", err)
	}
	defer rows.Close()

	var out []PublishedSeries
	for rows.Next() {
		var ps PublishedSeries
		var licenceURL *string
		if err := rows.Scan(&ps.Slug, &ps.Name, &ps.Unit, &ps.Frequency, &ps.Decimals, &ps.Geo, &ps.Harmonized,
			&ps.DiscontinuedSince, &ps.DiscontinuedSuccessorSlug,
			&ps.DatasetID,
			&ps.SourceID, &ps.SourceName, &ps.SourceAttribution, &ps.SourceLicenceName, &ps.SourceURL, &licenceURL,
			&ps.OriginKind, &ps.OriginRef); err != nil {
			return nil, fmt.Errorf("postgres: scanning a published series row: %w", err)
		}
		if licenceURL != nil {
			ps.SourceLicenceURL = *licenceURL
		}
		out = append(out, ps)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating published series rows: %w", err)
	}
	return out, nil
}

// PublishedObservation is one CURRENT (series_id, period) row together
// with its full provenance chain -- exactly the fields the spec's
// "Provenance survives the export" scenario requires reachable without
// a further database query: extraction timestamp, ingestion run and
// raw-file SHA-256 (source and origin identifier are the SERIES-level
// facts ListPublishedSeries already supplies; RequestURL here is the
// exact URL that specific run's raw_file was downloaded from, more
// precise than reconstructing one from the origin reference).
type PublishedObservation struct {
	Period         string
	Value          *float64
	Status         ObservationStatus
	SourceStatus   *string
	Version        int
	IngestionRunID int64
	ExtractedAt    time.Time
	RawFileSHA256  string
	RequestURL     string
}

// ListPublishedObservations returns every current observation for
// seriesID (published or withdrawn -- publishing.buildSeriesDoc is the
// caller that splits Points from Withdrawn), joined with the
// ingestion_run and raw_file rows that back it. Ordered by period so
// callers needing a stable, already-sorted sequence do not need to sort
// again.
func ListPublishedObservations(ctx context.Context, db DBTX, seriesID string) ([]PublishedObservation, error) {
	rows, err := db.Query(ctx, `
		SELECT o.period, o.value, o.status, o.source_status, o.version, o.ingestion_run_id, o.extracted_at, rf.hash, rf.url
		FROM observation o
		JOIN ingestion_run ir ON ir.id = o.ingestion_run_id
		JOIN raw_file rf ON rf.hash = ir.raw_file_hash
		WHERE o.series_id = $1 AND o.is_current
		ORDER BY o.period`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing published observations for %s: %w", seriesID, err)
	}
	defer rows.Close()

	var out []PublishedObservation
	for rows.Next() {
		var po PublishedObservation
		var status string
		if err := rows.Scan(&po.Period, &po.Value, &status, &po.SourceStatus, &po.Version, &po.IngestionRunID,
			&po.ExtractedAt, &po.RawFileSHA256, &po.RequestURL); err != nil {
			return nil, fmt.Errorf("postgres: scanning a published observation row for %s: %w", seriesID, err)
		}
		po.Status = ObservationStatus(status)
		out = append(out, po)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterating published observation rows for %s: %w", seriesID, err)
	}
	return out, nil
}
