package postgres_test

// Remediation batch (verify-report CRITICAL C2's prerequisite): IngestSeries
// assumes source/dataset/series/series_source_mapping already exist
// (its own doc comment, and every existing test seeds them by hand --
// seedDimensions in app/internal/ingestion/ingest_test.go). No production
// code ever wrote those rows: ReconcileEditorialConfig only reconciles
// Breaks/Events (reconcile.go's own doc comment), never series identity.
// Wiring `ingest --series|--source` to a real, fresh database therefore
// needs this: the same upsert-only, idempotent discipline
// ReconcileBreaks/ReconcileEvents already established for editorial YAML,
// applied to config/sources/*.yaml and config/series/*.yaml.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestReconcileDimensions_UpsertsSourceDatasetSeriesAndActiveMappingIdempotently(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	validFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := &config.Config{
		Sources: map[string]config.SourceConfig{
			"test-source": {ID: "test-source", Name: "Test Source", URL: "https://example.test", AccessType: "api-json",
				Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
		},
		Series: []config.SeriesConfig{
			{Slug: "test-series-dim", Name: "Test Series", Source: "test-source", Dataset: "test-dataset",
				Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "TESTDIM001", ValidFrom: validFrom}}},
		},
	}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := postgres.ReconcileDimensions(ctx, tx, cfg, now); err != nil {
		t.Fatalf("ReconcileDimensions: %v", err)
	}

	var sourceCount, datasetCount, seriesCount, mappingCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM source WHERE id='test-source'`).Scan(&sourceCount); err != nil {
		t.Fatalf("counting source: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM dataset WHERE id='test-dataset'`).Scan(&datasetCount); err != nil {
		t.Fatalf("counting dataset: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM series WHERE id='test-series-dim'`).Scan(&seriesCount); err != nil {
		t.Fatalf("counting series: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM series_source_mapping WHERE series_id='test-series-dim'`).Scan(&mappingCount); err != nil {
		t.Fatalf("counting series_source_mapping: %v", err)
	}
	if sourceCount != 1 || datasetCount != 1 || seriesCount != 1 || mappingCount != 1 {
		t.Fatalf("expected exactly one row in each dimension table, got source=%d dataset=%d series=%d mapping=%d",
			sourceCount, datasetCount, seriesCount, mappingCount)
	}

	var refKind, ref string
	if err := tx.QueryRow(ctx, `SELECT ref_kind, ref FROM series_source_mapping WHERE series_id='test-series-dim' AND valid_to IS NULL`).Scan(&refKind, &ref); err != nil {
		t.Fatalf("reading the active mapping: %v", err)
	}
	if refKind != "ine-series-cod" || ref != "TESTDIM001" {
		t.Errorf("expected the active mapping to be ine-series-cod/TESTDIM001, got %s/%s", refKind, ref)
	}

	// Idempotence: re-running against the identical config must not
	// duplicate any row (matches ReconcileBreaks/ReconcileEvents's own
	// "re-running changes nothing" discipline).
	if err := postgres.ReconcileDimensions(ctx, tx, cfg, now); err != nil {
		t.Fatalf("ReconcileDimensions (2nd run): %v", err)
	}
	var mappingCount2 int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM series_source_mapping WHERE series_id='test-series-dim'`).Scan(&mappingCount2); err != nil {
		t.Fatalf("counting series_source_mapping (2nd run): %v", err)
	}
	if mappingCount2 != 1 {
		t.Errorf("expected the second reconcile to add zero new mapping rows, got %d total", mappingCount2)
	}
}
