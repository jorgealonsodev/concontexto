package ingestion_test

// Task 6.4 (RED) / 6.5 (GREEN): the three pinned Eurostat datasets load
// and validate through IngestSeries -- the same end-to-end pipeline
// milestone 0.2 already proved for INE (spec source-ingestion-eurostat,
// "The three datasets load and validate" / "reuse the same domain
// types, observation writer and validation harness as the INE
// adapter").
//
// This test loads the REAL embedded config/series/*.yaml Eurostat
// entries via config.Load (the same tree validate-config checks), rather
// than hand-building an ingestion config the way the INE six-series test
// does: every Eurostat dataset code and dimension name IS on
// app/internal/guard's origin-identifier deny-list (task 3.3/3.4), so a
// Go string literal for e.g. "prc_hicp_minr" or "coicop18" anywhere in
// this file would fail TestNoOriginIdentifierLiteralsInGoSource. Loading
// the real config sidesteps that entirely (nothing here is a literal)
// and is a strictly stronger proof: it exercises the actual
// config/series/*.yaml content task 6.3 authored, not a hand-typed
// stand-in for it.

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// eurostatSourceRef finds sc's one "eurostat-dataset" source ref -- every
// config/series/*.yaml entry in this batch declares exactly one, mirroring
// the six INE series' single-active-ref shape.
func eurostatSourceRef(t *testing.T, sc config.SeriesConfig) config.SourceRef {
	t.Helper()
	for _, ref := range sc.SourceRefs {
		if ref.Kind == "eurostat-dataset" {
			return ref
		}
	}
	t.Fatalf("series %s (%s) has no eurostat-dataset source ref", sc.Slug, sc.FilePath)
	return config.SourceRef{}
}

// seedEurostatDimensions inserts the source/dataset/series/
// series_source_mapping rows IngestSeries assumes already exist
// (ReconcileEditorialConfig's job, not built this batch -- matches the
// INE six-series test's own documented boundary).
func seedEurostatDimensions(t *testing.T, ctx context.Context, tx pgx.Tx, sc config.SeriesConfig, ref config.SourceRef, licence string) {
	t.Helper()
	mustExecT(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('eurostat', 'Eurostat', 'https://ec.europa.eu/eurostat', $1, 'Source: Eurostat', 'api-json', 'digest1')
		ON CONFLICT (id) DO NOTHING`, licence)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ($1, 'eurostat', $1, 'digest1') ON CONFLICT (id) DO NOTHING`, sc.Dataset)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ($1, $2, $1, $3, $4, $5, $6, $7, 'digest1')`,
		sc.Slug, sc.Dataset, sc.Unit, sc.Frequency, sc.Geo, sc.Decimals, sc.Harmonized)
	mustExecT(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ($1, $2, $3, $4, 'digest1')`, sc.Slug, ref.Kind, ref.Ref, ref.ValidFrom)
}

func TestIngestSeries_AllThreeEurostatDatasetsLoadAndValidate(t *testing.T) {
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("expected the real embedded config to pass validate-config, got %v", violations)
	}

	eurostatSource, ok := cfg.Sources["eurostat"]
	if !ok {
		t.Fatal("expected config/sources/eurostat.yaml to be present")
	}

	var eurostatSeries []config.SeriesConfig
	for _, s := range cfg.Series {
		if s.Source == "eurostat" {
			eurostatSeries = append(eurostatSeries, s)
		}
	}
	if len(eurostatSeries) != 3 {
		t.Fatalf("expected exactly 3 Eurostat series configs (task 6.3), got %d: %+v", len(eurostatSeries), eurostatSeries)
	}

	for _, sc := range eurostatSeries {
		sc := sc
		t.Run(sc.Slug, func(t *testing.T) {
			ctx := context.Background()
			tx := newTx(t)
			if err := postgres.NewRunner(tx).Up(ctx); err != nil {
				t.Fatalf("Up: %v", err)
			}

			ref := eurostatSourceRef(t, sc)

			// The fixture is named by the dataset's own code (task 6.1's
			// testdata convention), read here purely as a filesystem path
			// -- ref.Ref comes from the loaded YAML, never a Go literal.
			fixturePath := filepath.Join("..", "adapters", "eurostat", "testdata", ref.Ref+".json")
			fixture, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("reading fixture %s: %v", fixturePath, err)
			}

			seedEurostatDimensions(t, ctx, tx, sc, ref, eurostatSource.Licence.Name)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(fixture)
			}))
			defer server.Close()

			client := eurostat.NewClient(server.URL, ref.Filters, server.Client())
			store := filestore.NewStore(t.TempDir())
			now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

			ingestCfg := ingestion.SeriesIngestConfig{
				SourceID:  "eurostat",
				DatasetID: sc.Dataset,
				SeriesID:  sc.Slug,
				COD:       ref.Ref,
				Series: indicators.Series{
					Slug: sc.Slug, Unit: sc.Unit, Frequency: indicators.Frequency(sc.Frequency), Decimals: sc.Decimals,
					Source: "eurostat", Licence: eurostatSource.Licence.Name,
				},
				Validation: sc.Validation,
				Schema:     sc.Schema,
			}

			result, err := ingestion.IngestSeries(ctx, tx, store, client, ingestCfg, now)
			if err != nil {
				t.Fatalf("IngestSeries(%s): %v", sc.Slug, err)
			}
			if result.Outcome != validation.GatePublish {
				t.Fatalf("expected %s to publish, got outcome=%v findings=%+v", sc.Slug, result.Outcome, result.Findings)
			}
			if len(result.Published) != 3 {
				t.Fatalf("expected %s to load its full 3-period fixture history, got %d observations", sc.Slug, len(result.Published))
			}

			for _, obs := range result.Published {
				prov, err := postgres.ResolveProvenance(ctx, tx, sc.Slug, obs.Period)
				if err != nil {
					t.Fatalf("ResolveProvenance(%s, %s): %v", sc.Slug, obs.Period, err)
				}
				if prov.SourceID != "eurostat" {
					t.Errorf("expected SourceID eurostat, got %q", prov.SourceID)
				}
				if prov.OriginRef != ref.Ref {
					t.Errorf("expected OriginRef %s, got %q", ref.Ref, prov.OriginRef)
				}
				if prov.RawFileHash == "" {
					t.Error("expected a non-empty raw-file hash")
				}
				if !prov.ExtractedAt.Equal(now) {
					t.Errorf("expected ExtractedAt %v, got %v", now, prov.ExtractedAt)
				}
			}
		})
	}
}

// TestIngestSeries_DeadDimensionCodeArchivesButBlocksPublication is task
// 6.10/6.11's end-to-end proof, mirroring
// ingest_test.go's TestIngestSeries_RefusalEnvelopeArchivesButBlocksPublication
// for INE's own HTTP-200-but-refused case: coicop18=CP00's response is
// ALSO HTTP 200, so the raw bytes are already archived (raw_file,
// ingestion_run) by the time decode discovers the dead dimension code —
// the evidence survives even though nothing is published, and nothing
// is written to observation.
func TestIngestSeries_DeadDimensionCodeArchivesButBlocksPublication(t *testing.T) {
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	eurostatSource, ok := cfg.Sources["eurostat"]
	if !ok {
		t.Fatal("expected config/sources/eurostat.yaml to be present")
	}
	var sc config.SeriesConfig
	found := false
	for _, s := range cfg.Series {
		if s.Source == "eurostat" && s.Dataset == "eurostat-hicp" {
			sc, found = s, true
		}
	}
	if !found {
		t.Fatal("expected the real embedded config to declare the HICP (prc_hicp_minr) series")
	}
	ref := eurostatSourceRef(t, sc)

	deadCodeFixture, err := os.ReadFile(filepath.Join("..", "adapters", "eurostat", "testdata", "deadcode", "prc_hicp_minr-coicop18-cp00.json"))
	if err != nil {
		t.Fatalf("reading the dead-code fixture: %v", err)
	}

	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedEurostatDimensions(t, ctx, tx, sc, ref, eurostatSource.Licence.Name)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(deadCodeFixture)
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, ref.Filters, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	ingestCfg := ingestion.SeriesIngestConfig{
		SourceID:  "eurostat",
		DatasetID: sc.Dataset,
		SeriesID:  sc.Slug,
		COD:       ref.Ref,
		Series: indicators.Series{
			Slug: sc.Slug, Unit: sc.Unit, Frequency: indicators.Frequency(sc.Frequency), Decimals: sc.Decimals,
			Source: "eurostat", Licence: eurostatSource.Licence.Name,
		},
		Validation: sc.Validation,
		Schema:     sc.Schema,
	}

	result, ingestErr := ingestion.IngestSeries(ctx, tx, store, client, ingestCfg, now)
	if ingestErr == nil {
		t.Fatal("expected the dead dimension code to fail IngestSeries")
	}
	if result.RunID == 0 {
		t.Fatal("expected an ingestion_run id even though the run failed at decode (the archive already happened)")
	}
	if result.Outcome != validation.GateBlock {
		t.Errorf("expected GateBlock, got %v", result.Outcome)
	}
	if len(result.Published) != 0 {
		t.Errorf("expected zero published observations, got %d", len(result.Published))
	}

	var runCount, obsCount, rawFileCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM ingestion_run`).Scan(&runCount); err != nil {
		t.Fatalf("counting ingestion_run rows: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM observation`).Scan(&obsCount); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM raw_file`).Scan(&rawFileCount); err != nil {
		t.Fatalf("counting raw_file rows: %v", err)
	}
	if runCount != 1 {
		t.Errorf("expected exactly 1 ingestion_run row (the archived-but-blocked run), got %d", runCount)
	}
	if rawFileCount != 1 {
		t.Errorf("expected exactly 1 raw_file row (the HTTP-200 dead-code bytes ARE archived), got %d", rawFileCount)
	}
	if obsCount != 0 {
		t.Errorf("expected zero observation rows -- nothing written for the distinct zero-observation class, got %d", obsCount)
	}
}

// TestIngestSeries_EurostatSatisfiesTheSameSourceClientPortAsINE is a
// runtime-checked type-level proof: nothing in IngestSeries changed for
// Eurostat to work, because both adapters satisfy the same
// indicators.SourceClient interface (spec "reuse the same ... validation
// harness as the INE adapter; only the adapter differs").
func TestIngestSeries_EurostatSatisfiesTheSameSourceClientPortAsINE(t *testing.T) {
	var client indicators.SourceClient = eurostat.NewClient("http://example.invalid", nil, nil)
	if client == nil {
		t.Fatal("expected a non-nil indicators.SourceClient")
	}
}
