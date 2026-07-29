package main

// Remediation batch (verify-report CRITICAL C2): the `ingest` subcommand
// handled only `--reconcile`; every other invocation kept a placeholder
// ("not yet implemented"). ingestion.IngestSeries had ten call sites, all
// _test.go -- the binary could not ingest a single configured series.
// runIngest is the testable core (mirrors runIngestReconcile's own
// pattern): it takes an already-open db, an already-loaded *config.Config
// and an already-constructed *filestore.Store instead of resolving
// DATABASE_URL / the embedded tree / /app_data itself, so this file can
// prove the whole pipeline (dimension reconcile -> client selection ->
// IngestSeries -> hash listing) against a real httptest server and a real
// Postgres, without touching the real filesystem or environment.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
)

func TestCmdIngest_SeriesAndSourceFlagsRequireDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	for _, args := range [][]string{{"--series=whatever"}, {"--source=whatever"}} {
		var stdout, stderr bytes.Buffer
		if code := cmdIngest(args, &stdout, &stderr); code != 1 {
			t.Fatalf("cmdIngest(%v): expected exit 1 when DATABASE_URL is unset, got %d", args, code)
		}
		if !strings.Contains(stderr.String(), "DATABASE_URL") {
			t.Errorf("cmdIngest(%v): expected stderr to mention DATABASE_URL, got %q", args, stderr.String())
		}
	}
}

func TestCmdIngest_NoTargetFlagPrintsUsageNamingSeriesAndSource(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdIngest(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 with no target flag, got %d (stdout=%q)", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "--series") || !strings.Contains(stderr.String(), "--source") {
		t.Errorf("expected usage naming --series and --source, got %q", stderr.String())
	}
}

func TestRunIngest_UnknownSeriesSlugFails(t *testing.T) {
	cfg := &config.Config{}
	var stdout, stderr bytes.Buffer
	code := runIngest(context.Background(), nil, cfg, nil, "", "", time.Now(), "does-not-exist", "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for an unknown series slug, got %d", code)
	}
	if !strings.Contains(stderr.String(), "does-not-exist") {
		t.Errorf("expected stderr to name the unknown slug, got %q", stderr.String())
	}
}

func TestRunIngest_UnknownSourceIDFails(t *testing.T) {
	cfg := &config.Config{}
	var stdout, stderr bytes.Buffer
	code := runIngest(context.Background(), nil, cfg, nil, "", "", time.Now(), "", "does-not-exist", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for an unknown source id, got %d", code)
	}
	if !strings.Contains(stderr.String(), "does-not-exist") {
		t.Errorf("expected stderr to name the unknown source, got %q", stderr.String())
	}
}

func TestRunIngest_SeriesFlagIngestsOneSeriesEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_run_ingest_series_test"),
		tcpostgres.WithUsername("concontexto_run_ingest_series_test"),
		tcpostgres.WithPassword("concontexto_run_ingest_series_test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	var migrateOut, migrateErr bytes.Buffer
	t.Setenv("DATABASE_URL", dsn)
	if code := cmdMigrate([]string{"up"}, &migrateOut, &migrateErr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, migrateErr.String())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	fixture := []byte(`{"COD":"TESTCMDING01", "Nombre":"test", "T3_Unidad":"index", "T3_Escala":" ", "Data":[` +
		`{"Fecha":"2026-06-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M06", "Anyo":2026, "Valor":100}]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	cfg := &config.Config{
		Sources: map[string]config.SourceConfig{
			"test-src": {ID: "test-src", Name: "Test", URL: "https://example.test", AccessType: "api-json",
				API:     &config.APIConfig{BaseURL: server.URL},
				Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
		},
		Series: []config.SeriesConfig{
			{Slug: "test-cmd-series", Name: "Test", Source: "test-src", Dataset: "test-cmd-dataset",
				Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "TESTCMDING01", ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}},
		},
	}

	root := t.TempDir()
	store := filestore.NewStore(filepath.Join(root, "raw"))
	archiveHashPath := filepath.Join(root, "app_data", "raw_files.sha256")
	publicHashPath := filepath.Join(root, "public", "transparencia", "raw-files.sha256")
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	var stdout, stderr bytes.Buffer
	code := runIngest(ctx, pool, cfg, store, archiveHashPath, publicHashPath, now, "test-cmd-series", "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runIngest: exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "outcome=publish") {
		t.Errorf("expected a publish outcome, got %q", stdout.String())
	}

	var obsCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id='test-cmd-series'`).Scan(&obsCount); err != nil {
		t.Fatalf("counting observation: %v", err)
	}
	if obsCount != 1 {
		t.Errorf("expected exactly 1 observation row, got %d", obsCount)
	}

	if _, err := os.Stat(publicHashPath); err != nil {
		t.Errorf("expected the public hash listing to exist after a real ingest run: %v", err)
	}
}

func TestRunIngest_SourceFlagIngestsEveryConfiguredSeriesOfThatSource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_run_ingest_source_test"),
		tcpostgres.WithUsername("concontexto_run_ingest_source_test"),
		tcpostgres.WithPassword("concontexto_run_ingest_source_test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	var migrateOut, migrateErr bytes.Buffer
	t.Setenv("DATABASE_URL", dsn)
	if code := cmdMigrate([]string{"up"}, &migrateOut, &migrateErr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, migrateErr.String())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	fixtures := map[string][]byte{
		"TESTCMDSRCA": []byte(`{"COD":"TESTCMDSRCA", "Nombre":"a", "T3_Unidad":"index", "T3_Escala":" ", "Data":[` +
			`{"Fecha":"2026-06-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M06", "Anyo":2026, "Valor":10}]}`),
		"TESTCMDSRCB": []byte(`{"COD":"TESTCMDSRCB", "Nombre":"b", "T3_Unidad":"index", "T3_Escala":" ", "Data":[` +
			`{"Fecha":"2026-06-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M06", "Anyo":2026, "Valor":20}]}`),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for cod, body := range fixtures {
			if strings.Contains(r.URL.Path, cod) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(body)
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := &config.Config{
		Sources: map[string]config.SourceConfig{
			"test-src-multi": {ID: "test-src-multi", Name: "Test", URL: "https://example.test", AccessType: "api-json",
				API:     &config.APIConfig{BaseURL: server.URL},
				Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
		},
		Series: []config.SeriesConfig{
			{Slug: "test-cmd-series-a", Name: "A", Source: "test-src-multi", Dataset: "test-cmd-dataset-multi",
				Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "TESTCMDSRCA", ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}},
			{Slug: "test-cmd-series-b", Name: "B", Source: "test-src-multi", Dataset: "test-cmd-dataset-multi",
				Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "TESTCMDSRCB", ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}},
		},
	}

	root := t.TempDir()
	store := filestore.NewStore(filepath.Join(root, "raw"))
	archiveHashPath := filepath.Join(root, "app_data", "raw_files.sha256")
	publicHashPath := filepath.Join(root, "public", "transparencia", "raw-files.sha256")
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	var stdout, stderr bytes.Buffer
	code := runIngest(ctx, pool, cfg, store, archiveHashPath, publicHashPath, now, "", "test-src-multi", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runIngest: exit %d, stderr=%q", code, stderr.String())
	}
	for _, slug := range []string{"test-cmd-series-a", "test-cmd-series-b"} {
		if !strings.Contains(stdout.String(), slug) {
			t.Errorf("expected stdout to report series %s, got %q", slug, stdout.String())
		}
	}

	var seriesCount int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT series_id) FROM observation WHERE series_id IN ('test-cmd-series-a','test-cmd-series-b')`).Scan(&seriesCount); err != nil {
		t.Fatalf("counting distinct series with observations: %v", err)
	}
	if seriesCount != 2 {
		t.Errorf("expected both series of the source to be ingested, got observations for %d series", seriesCount)
	}
}
