package ingestion_test

// Task 4.13: "An automated test MUST ingest from recorded fixtures,
// export the artifact and build the site from it, proving the two
// ecosystems actually meet" (spec publishing-export, "One end-to-end
// ingest-then-build test exists"). This file owns the Go-side half: a
// real ingestion.IngestSeries run against served fixtures, through a
// real Postgres transaction, followed by a real publishing.Export --
// exercising the exact chain the CI job
// (.github/workflows/ingest-export-build.yml) runs.
//
// THE HAND-OFF TO THE ASTRO HALF. The artifact-consuming loader
// (web/src/lib/export/loader.ts) exists since slice 9a and is wired into
// /indicador/[slug].astro's getStaticPaths, so the Astro build can now
// read the very bytes this test writes. It only does so if both halves
// are pointed at the SAME directory, which is what E2E_EXPORT_DIR is
// for: when it is set, the export lands there instead of in a t.TempDir()
// the test discards on exit, and the CI job's next step runs
// `astro build` with EXPORT_DIR set to that same path. Without that
// hand-off the two steps look like a chain and are in fact two unrelated
// runs in sequence -- precisely the "built, tested, never connected"
// shape task 4.13 exists to kill.
//
// A separate `concontexto export` process cannot substitute for the
// in-process export here: every row this test writes lives inside a
// transaction that is rolled back when the test ends, so no other
// connection can see it. The export MUST happen inside this process,
// against this transaction -- hence an environment-driven output
// directory rather than a second command invocation.
//
// The six frozen slugs are ingested alongside the synthetic probe series
// precisely so the Astro build has real routes to emit: getStaticPaths
// filters INDICATOR_CONTENT down to the slugs the artifact actually
// carries, so an artifact holding only a throwaway slug would build a
// site with zero indicator pages and prove nothing about rendering.
//
// go test -count=1 -run TestEndToEndIngestExportBuild -v ./app/internal/ingestion/...
// is the exact invocation the CI job runs.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func TestEndToEndIngestExportBuild(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	// STEP 3 OF THE DEPLOY SEQUENCE, IN THE SAME ORDER THE STACK RUNS IT:
	// migrations, then the editorial reconcile, then ingestion. The order is
	// load-bearing twice over -- validation rule 3 consults series_break
	// while each ingest below runs, and publishing.Export reads the same
	// rows back into the artifact's `breaks`/`events` arrays afterwards --
	// which is exactly why docker-compose.yml gates `app` on the `reconcile`
	// service having exited 0 rather than letting the scheduler race it.
	reconcileShippedEditorialConfig(t, ctx, tx)

	sc := sixSeriesCase{slug: "test-e2e-export", datasetID: "test-e2e-dataset", unit: "índice", frequency: indicators.FrequencyQuarterly, decimals: 2}
	cod := "TESTE2E001"
	seedDimensions(t, ctx, tx, sc, cod)

	fixture := []byte(`{"COD":"` + cod + `", "Nombre":"test e2e", "T3_Unidad":"indice", "T3_Escala":" ", "Data":[` +
		`{"Fecha":"2026-01-01T00:00:00.000+01:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"T1", "Anyo":2026, "Valor":42.5}` +
		`]}`)
	server := serveFixture(fixture)
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	cfg := ingestion.SeriesIngestConfig{
		SourceID: "ine", DatasetID: sc.datasetID, SeriesID: sc.slug, COD: cod,
		Series:     indicators.Series{Slug: sc.slug, Unit: sc.unit, Frequency: sc.frequency, Decimals: sc.decimals, Source: "ine", Licence: "test"},
		Validation: config.ValidationConfig{},
	}

	result, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now)
	if err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected the ingest to publish, got outcome=%v findings=%+v", result.Outcome, result.Findings)
	}

	// The six real, frozen slugs, from the same recorded INE payloads the
	// rest of this package ingests -- so the exported artifact carries the
	// routes /indicador/[slug].astro actually publishes, and the CI build
	// step downstream renders real pages from freshly ingested data rather
	// than emitting an empty site.
	ingestSixFrozenSlugs(t, ctx, tx, store, now)

	outDir := exportOutputDir(t)
	artifact, err := publishing.Export(ctx, exportDeps(tx), now, outDir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	var found *publishing.SeriesDoc
	for i := range artifact.Series {
		if artifact.Series[i].Slug == sc.slug {
			found = &artifact.Series[i]
		}
	}
	if found == nil {
		t.Fatalf("expected the just-ingested series %q in the exported artifact, got %+v", sc.slug, artifact.Series)
	}
	if len(found.Points) != 1 || found.Points[0].Period != "2026-Q1" || found.Points[0].Value != 42.5 {
		t.Fatalf("expected the exported artifact to carry the ingested 2026-Q1=42.5 point, got %+v", found.Points)
	}

	// Prove the on-disk artifact (what the astro-build CI step actually
	// reads) matches too, not only the in-memory return value.
	seriesBytes, err := os.ReadFile(filepath.Join(outDir, "series", sc.slug+".json"))
	if err != nil {
		t.Fatalf("reading the exported series doc from disk: %v", err)
	}
	var onDisk publishing.SeriesDoc
	if err := json.Unmarshal(seriesBytes, &onDisk); err != nil {
		t.Fatalf("unmarshalling the on-disk series doc: %v", err)
	}
	if len(onDisk.Points) != 1 || onDisk.Points[0].Value != 42.5 {
		t.Fatalf("expected the on-disk artifact to match the in-memory one, got %+v", onDisk.Points)
	}

	// Every frozen slug the Astro routes publish must be on disk too. If
	// this ever regresses, the CI build step downstream would silently
	// emit a site with fewer indicator pages instead of failing, so the
	// gap is asserted HERE rather than inferred from a green build.
	exported := map[string]bool{}
	for _, doc := range artifact.Series {
		exported[doc.Slug] = true
	}
	for _, frozen := range sixSeries {
		if !exported[frozen.slug] {
			t.Errorf("expected the frozen slug %q in the exported artifact; the Astro build would emit no page for it", frozen.slug)
		}
		if _, err := os.Stat(filepath.Join(outDir, "series", frozen.slug+".json")); err != nil {
			t.Errorf("expected %s.json on disk for the Astro build to read: %v", frozen.slug, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); err != nil {
		t.Errorf("expected manifest.json on disk -- the loader reads it first: %v", err)
	}

	// THE EDITORIAL HALF OF THE HAND-OFF. Everything above proves the
	// OBSERVATIONS reach the Astro build. The artifact carries two more
	// arrays the site renders from -- `breaks` (BreakBand.astro and the
	// chart's shaded band geometry) and `events` (annotations) -- and this
	// test used to export both EMPTY, because nothing here ever called
	// ingestion.ReconcileEditorialConfig. The CI job whose stated purpose is
	// "the test that kills the 'built, tested, never connected' shape" was
	// therefore building a site in which the break band could not render,
	// and asserting nothing about it. The deployed stack was in the same
	// state for the same reason (docker-compose.yml's `reconcile` service
	// closes that half; app/cmd/concontexto/deploy_reconcile_composition_test.go
	// pins it).
	//
	// epa-metodologia-2021 is the assertion target because it is scoped to
	// DATASET ine-epa, so it must resolve for every series in that dataset
	// via ResolveActiveBreaksForSeries' scope expansion -- proving the
	// reconcile, the scope expansion and the export all line up, not merely
	// that a row exists.
	assertExportedBreak(t, artifact, "tasa-de-paro-epa", "epa-metodologia-2021")
	assertExportedBreak(t, artifact, "ocupados-epa", "epa-metodologia-2021")
}

// reconcileShippedEditorialConfig projects the SHIPPED config/*.yaml
// registries onto tx, exactly as the deployed stack's one-shot `reconcile`
// service does (docker-compose.yml -> `concontexto ingest --reconcile` ->
// ingestion.ReconcileEditorialConfig). The shipped config is used rather
// than a hand-built one on purpose: an invented break would prove that the
// export can carry SOME break, not that the break this project actually
// publishes reaches the page.
func reconcileShippedEditorialConfig(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	cfg, err := shippedConfig()
	if err != nil {
		t.Fatalf("loading the shipped config: %v", err)
	}
	if _, err := ingestion.ReconcileEditorialConfig(ctx, tx, *cfg); err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}
}

// assertExportedBreak fails unless slug's exported doc carries breakKey.
func assertExportedBreak(t *testing.T, artifact publishing.Artifact, slug, breakKey string) {
	t.Helper()
	for i := range artifact.Series {
		if artifact.Series[i].Slug != slug {
			continue
		}
		for _, b := range artifact.Series[i].Breaks {
			if b.Key == breakKey {
				return
			}
		}
		t.Errorf("the exported %s doc carries breaks %+v, which does not include %q: "+
			"the Astro build renders BreakBand from this array, so an empty or incomplete one means "+
			"no band on the page no matter how correct the component is",
			slug, artifact.Series[i].Breaks, breakKey)
		return
	}
	t.Errorf("no exported doc for %q, so its breaks cannot be asserted", slug)
}

// exportDeps binds every publishing port against tx, exactly as
// buildExportDeps (app/cmd/concontexto/export_cmd.go) binds them in
// production.
//
// Shared by both end-to-end export tests rather than restated in each,
// because the point of an end-to-end test here is that the exporter is the
// production one: two hand-written Deps literals drift, and a thinner one
// silently changes what the artifact says. SeriesValidationOutcome is the
// concrete example -- an artifact exported without that port reports every
// series as "fresh" regardless of its real validation outcome, so the
// bytes the Astro build reads would not be the bytes production writes.
func exportDeps(tx pgx.Tx) publishing.Deps {
	return publishing.Deps{
		ListPublishedSeries: func(ctx context.Context) ([]postgres.PublishedSeries, error) {
			return postgres.ListPublishedSeries(ctx, tx)
		},
		ListObservations: func(ctx context.Context, seriesID string) ([]postgres.PublishedObservation, error) {
			return postgres.ListPublishedObservations(ctx, tx, seriesID)
		},
		SeriesFreshness: func(ctx context.Context, seriesID string, asOf time.Time) (freshness.State, error) {
			return postgres.SeriesFreshness(ctx, tx, seriesID, asOf)
		},
		ResolveActiveBreaksForSeries: func(ctx context.Context, seriesID string) ([]postgres.SeriesBreak, error) {
			return postgres.ResolveActiveBreaksForSeries(ctx, tx, seriesID)
		},
		ListActiveEvents: func(ctx context.Context, seriesID string) ([]postgres.Event, error) {
			return postgres.ListActiveEvents(ctx, tx, seriesID)
		},
		SeriesValidationOutcome: func(ctx context.Context, seriesID string) (postgres.ValidationOutcome, error) {
			return postgres.SeriesValidationOutcome(ctx, tx, seriesID)
		},
	}
}

// exportOutputDir resolves where publishing.Export writes.
//
// Unset (every ordinary `go test ./...` run) means a t.TempDir() that Go
// removes when the test ends -- this test leaves nothing behind and needs
// no writable project path. Set (the CI job, and the local reproduction of
// it) means the artifact SURVIVES the test process, so the `astro build`
// step that follows can point EXPORT_DIR at these exact bytes. That is the
// whole hand-off; see this file's header.
func exportOutputDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("E2E_EXPORT_DIR")
	if dir == "" {
		return t.TempDir()
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("E2E_EXPORT_DIR=%q is not resolvable to an absolute path: %v", dir, err)
	}
	// publishing.Export writes into outDir/series and outDir/csv, but does
	// not create outDir itself.
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatalf("creating E2E_EXPORT_DIR %q: %v", abs, err)
	}
	return abs
}

// ingestSixFrozenSlugs runs a real IngestSeries for each of the six pinned
// series against its own recorded INE payload, on the transaction the
// caller is about to export from. Reuses the package's existing fixture,
// dimension-seeding and config helpers rather than restating them, so the
// six series enter the database by exactly the path the rest of the suite
// already proves.
func ingestSixFrozenSlugs(t *testing.T, ctx context.Context, tx pgx.Tx, store *filestore.Store, now time.Time) {
	t.Helper()
	for _, sc := range sixSeries {
		cod, fixture := loadFixture(t, sc.slug)
		seedDimensions(t, ctx, tx, sc, cod)

		server := serveFixture(fixture)
		client := ine.NewClient(server.URL, server.Client())
		result, err := ingestion.IngestSeries(ctx, tx, store, client, ineIngestConfig(t, sc, cod), now)
		server.Close()
		if err != nil {
			t.Fatalf("IngestSeries(%s): %v", sc.slug, err)
		}
		if result.Outcome != validation.GatePublish {
			t.Fatalf("expected %s to publish, got outcome=%v findings=%+v", sc.slug, result.Outcome, result.Findings)
		}
	}
}
