package main

// Task 3.11 (RED/GREEN): the `export` subcommand. exportOutputDir is a
// pure unit test; cmdExport's DATABASE_URL guard needs no Docker;
// TestRunExport_EndToEnd proves the real wiring -- publishing.Export
// composed against the real postgres package functions, through the
// exact command-layer composition point (buildExportDeps) production
// code uses -- against a genuine Postgres container, closing the loop
// task 4.13's later CI e2e job (ingest -> export -> build) will extend.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func TestExportOutputDir_SelectsFixtureOrStaticRootDataDerived(t *testing.T) {
	t.Setenv("STATIC_ROOT", "/web/dist")
	if got := exportOutputDir(true); got != fixtureExportDir {
		t.Errorf("expected --fixture to select %q, got %q", fixtureExportDir, got)
	}
	if got, want := exportOutputDir(false), filepath.Join("/web/dist", "data-derived"); got != want {
		t.Errorf("expected the live path to be %q, got %q", want, got)
	}
}

func TestCmdExport_FailsWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	var stdout, stderr bytes.Buffer
	code := cmdExport(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected a non-zero exit code with no DATABASE_URL")
	}
	if stderr.Len() == 0 {
		t.Fatal("expected an error message on stderr")
	}
}

// Remediation batch (verify-report WARNING-17): buildExportDeps is the ONE
// production composition point every export path shares (`export`,
// `export --fixture`, and runIngest's in-cycle publish). Nothing used to
// assert that it binds SeriesValidationOutcome, and that port used to be
// optional-with-a-"fresh"-default, so deleting one line here would have
// reverted CRITICAL-4 with the whole suite still green.
//
// Two guards now stand where there were none, deliberately at different
// distances from the defect:
//
//   - publishing.Export REFUSES an unbound SeriesValidationOutcome port
//     (page_state_test.go's own case), which makes the regression
//     impossible rather than merely detected -- production cannot write an
//     artifact at all with that line removed.
//   - THIS test names the defect at its source, with no Docker and no
//     database, so the red is immediate and points at the composition
//     rather than at a failing export three layers away.
//
// Every port is checked, not only the one WARNING-17 names: a nil port
// that Export calls unconditionally panics rather than degrading silently,
// but a panic in a scheduled ingest cycle is still a production outage
// worth catching in a millisecond here. buildExportDeps never touches db,
// so a nil DBTX is the honest argument for a pure composition assertion.
func TestBuildExportDeps_BindsEveryPortIncludingTheValidationOutcome(t *testing.T) {
	deps := buildExportDeps(nil)

	for _, c := range []struct {
		name  string
		bound bool
	}{
		{"ListPublishedSeries", deps.ListPublishedSeries != nil},
		{"ListObservations", deps.ListObservations != nil},
		{"SeriesFreshness", deps.SeriesFreshness != nil},
		{"ResolveActiveBreaksForSeries", deps.ResolveActiveBreaksForSeries != nil},
		{"ListActiveEvents", deps.ListActiveEvents != nil},
		{"SeriesValidationOutcome", deps.SeriesValidationOutcome != nil},
	} {
		if !c.bound {
			t.Errorf("buildExportDeps left %s unbound; the production export path would %s",
				c.name, "either refuse to export or report every series as fresh")
		}
	}
}

func TestRunExport_EndToEndAgainstRealPostgresWritesTheArtifactMatchingIngestedData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_export_cmd_test"),
		tcpostgres.WithUsername("concontexto_export_cmd_test"),
		tcpostgres.WithPassword("concontexto_export_cmd_test"),
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

	if _, err := pool.Exec(ctx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://ine.es', 'lic', 'Fuente: INE', 'api-json', 'd1')`); err != nil {
		t.Fatalf("seeding source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('ine-epa', 'ine', 'ine-epa', 'd1')`); err != nil {
		t.Fatalf("seeding dataset: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-epa', 'Tasa de paro', '%', 'Q', 'ES', 2, false, 'd1')`); err != nil {
		t.Fatalf("seeding series: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'EPA453100', '2020-01-01', 'd1')`); err != nil {
		t.Fatalf("seeding mapping: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ('hash-e2e', 'ine', 'https://ine.es/EPA453100', now(), '/app_data/hash-e2e', 100)`); err != nil {
		t.Fatalf("seeding raw_file: %v", err)
	}
	var runID int64
	if err := pool.QueryRow(ctx, `INSERT INTO ingestion_run (dataset_id, series_id, started_at, finished_at, raw_file_hash, outcome)
		VALUES ('ine-epa', 'tasa-de-paro-epa', now(), now(), 'hash-e2e', 'succeeded') RETURNING id`).Scan(&runID); err != nil {
		t.Fatalf("seeding ingestion_run: %v", err)
	}

	writer := postgres.NewObservationWriter(pool)
	extractedAt := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	value := 11.5
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: &value,
		Status: postgres.StatusDefinitive, ExtractedAt: extractedAt, IngestionRunID: runID,
	}); err != nil {
		t.Fatalf("WriteRevision: %v", err)
	}

	outDir := t.TempDir()
	deps := buildExportDeps(pool)
	var stdout, stderr bytes.Buffer
	if code := runExport(ctx, deps, outDir, time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC), &stdout, &stderr); code != 0 {
		t.Fatalf("runExport: exit %d, stderr=%q", code, stderr.String())
	}

	seriesBytes, err := os.ReadFile(filepath.Join(outDir, "series", "tasa-de-paro-epa.json"))
	if err != nil {
		t.Fatalf("reading the exported series doc: %v", err)
	}
	var doc publishing.SeriesDoc
	if err := json.Unmarshal(seriesBytes, &doc); err != nil {
		t.Fatalf("unmarshalling the exported series doc: %v", err)
	}
	if doc.Slug != "tasa-de-paro-epa" {
		t.Fatalf("expected slug tasa-de-paro-epa, got %q", doc.Slug)
	}
	if len(doc.Points) != 1 || doc.Points[0].Value != 11.5 || doc.Points[0].Period != "2026-Q1" {
		t.Fatalf("expected the ingested 2026-Q1=11.5 point, got %+v", doc.Points)
	}
	rp, ok := doc.Vintages[strconv.FormatInt(runID, 10)]
	if !ok {
		t.Fatalf("expected vintages to resolve run id %d, got %+v", runID, doc.Vintages)
	}
	if rp.RawFileSHA256 != "hash-e2e" {
		t.Fatalf("expected the ingested run's raw-file hash to reach the artifact, got %q", rp.RawFileSHA256)
	}
}

// prunableExportDeps binds publishing.Deps to publish exactly the named
// slugs from memory -- no database, no Docker. The command layer's job
// here is reporting, and reporting is testable without either.
func prunableExportDeps(slugs ...string) publishing.Deps {
	series := make([]postgres.PublishedSeries, 0, len(slugs))
	obs := map[string][]postgres.PublishedObservation{}
	for _, slug := range slugs {
		series = append(series, postgres.PublishedSeries{
			Slug: slug, Name: slug, Unit: "%", Frequency: "Q", Decimals: 2, Geo: "ES",
			DatasetID: "ine-epa", SourceID: "ine", SourceName: "INE", SourceAttribution: "Fuente: INE",
			SourceLicenceName: "lic", SourceURL: "https://ine.es",
			OriginKind: "ine-series-cod", OriginRef: "TESTCOD001",
		})
		value := 10.5
		obs[slug] = []postgres.PublishedObservation{{
			Period: "2026-Q1", Value: &value, Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es/data",
		}}
	}
	return publishing.Deps{
		ListPublishedSeries: func(context.Context) ([]postgres.PublishedSeries, error) { return series, nil },
		ListObservations: func(_ context.Context, seriesID string) ([]postgres.PublishedObservation, error) {
			return obs[seriesID], nil
		},
		SeriesFreshness: func(context.Context, string, time.Time) (freshness.State, error) {
			return freshness.StateFresh, nil
		},
		ResolveActiveBreaksForSeries: func(context.Context, string) ([]postgres.SeriesBreak, error) { return nil, nil },
		ListActiveEvents:             func(context.Context, string) ([]postgres.Event, error) { return nil, nil },
		SeriesValidationOutcome: func(context.Context, string) (postgres.ValidationOutcome, error) {
			return postgres.ValidationOutcome{}, nil
		},
	}
}

// TestRunExport_NamesEveryFileItRemovedFromTheServedDirectory closes the
// observability half of the stale-file defect. publishing.Export now
// deletes the files a dropped-out series left behind (see
// app/internal/publishing/export_prune_test.go for the defect itself), and
// a delete an operator cannot see in the log is its own integrity problem:
// the removed file was reader-facing, and its disappearance is exactly the
// event someone investigating "the page for X is gone" needs to correlate
// against. The paths are named, not counted.
func TestRunExport_NamesEveryFileItRemovedFromTheServedDirectory(t *testing.T) {
	outDir := t.TempDir()
	ctx := context.Background()
	asOf := time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC)

	var stdout, stderr bytes.Buffer
	if code := runExport(ctx, prunableExportDeps("ipc-general", "ocupados-epa"), outDir, asOf, &stdout, &stderr); code != 0 {
		t.Fatalf("seeding export: exit %d, stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	if code := runExport(ctx, prunableExportDeps("ipc-general"), outDir, asOf.Add(time.Hour), &stdout, &stderr); code != 0 {
		t.Fatalf("runExport: exit %d, stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"series/ocupados-epa.json", "csv/ocupados-epa.csv"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected stdout to name %s as removed, got %q", want, stdout.String())
		}
	}
}

// TestRunExport_ReportsThatTheZeroSeriesGuardRefusedToPrune proves the
// guard is not silent. An export declaring no series deletes nothing (see
// pruneUnpublishedFiles' own doc comment for why that refusal is the right
// trade), which leaves the directory holding files the manifest no longer
// declares -- a state an operator must be told about, since it is
// indistinguishable, in a log that only ever prints removals, from a
// healthy cycle with nothing stale to remove.
func TestRunExport_ReportsThatTheZeroSeriesGuardRefusedToPrune(t *testing.T) {
	outDir := t.TempDir()
	ctx := context.Background()
	asOf := time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC)

	var stdout, stderr bytes.Buffer
	if code := runExport(ctx, prunableExportDeps("ipc-general"), outDir, asOf, &stdout, &stderr); code != 0 {
		t.Fatalf("seeding export: exit %d, stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	if code := runExport(ctx, prunableExportDeps(), outDir, asOf.Add(time.Hour), &stdout, &stderr); code != 0 {
		t.Fatalf("runExport: exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "refused to remove anything") {
		t.Fatalf("expected stdout to report the refusal, got %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "series", "ipc-general.json")); err != nil {
		t.Fatalf("expected the previously published series doc to survive an empty export: %v", err)
	}
}
