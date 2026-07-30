package main

// Remediation batch (verify-report CRITICAL-16): the export gate at the end
// of runIngest, proven per OUTCOME KIND against a real Postgres, a real
// source server, a real IngestSeries and a real publishing.Publish.
//
// WHAT WAS MISSING, AND WHY IT MATTERED. `publishing-export`'s failed-run
// scenario ("THEN no new artifact is exported") had no covering test
// anywhere in the Go suite, and the code that implemented it made
// `indicator-page`'s PRD §6.1.3 validation banner undeliverable: a reader
// only ever sees that banner through a newly exported artifact, and the
// gate withheld exactly that on the only event that raises the banner. The
// spec has since been NARROWED (see publishing-export/spec.md's amended
// "A failed ingestion exports the failure state, never the suspect datum"
// scenario and design.md's D-2 resolution): the suspect datum must never be
// published, but the FAILURE STATE must be. Neither half of that is
// provable from unit tests of the pieces -- the publish gate, the page-state
// read and the artifact writer were each already covered, and the defect
// lived entirely in the seam between them (WARNING-17's own observation).
//
// So this file joins them. TestRunIngest_TheExportGate... drives a real
// validation failure through the real production export trigger and reads
// the resulting page state back off DISK, because bytes on disk are what
// the Astro build consumes and therefore the only artifact a reader can
// ever be shown.
//
// ONE CONTAINER, THREE SUBTESTS. Each subtest owns its own series, source
// server, output directory and dispatcher, so they share nothing but the
// database engine -- the ordinary per-test container in this package costs
// several seconds to start and there are already five of them.

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// ineFixture builds a DATOS_SERIE payload for cod carrying one entry per
// (period label, month, value) triple -- the same minimal shape every
// other ingest test in this package serves, kept local so a change to
// this file's scenarios never perturbs theirs.
func ineFixture(cod string, entries ...ineFixtureEntry) []byte {
	var b strings.Builder
	b.WriteString(`{"COD":"` + cod + `", "Nombre":"test", "T3_Unidad":"index", "T3_Escala":" ", "Data":[`)
	for i, e := range entries {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"Fecha":"2026-` + e.month + `-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M` + e.month + `", "Anyo":2026, "Valor":` + e.value + `}`)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

type ineFixtureEntry struct {
	month string // "06", "07" -- also the T3_Periodo ordinal
	value string // JSON number literal, so a test can serve an implausible one verbatim
}

// exportGatePostgres starts one migrated Postgres container and returns a
// pool against it. Mirrors this package's established per-test container
// setup exactly; extracted only so three subtests can share one engine.
func exportGatePostgres(t *testing.T, dbName string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase(dbName),
		tcpostgres.WithUsername(dbName),
		tcpostgres.WithPassword(dbName),
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
	t.Cleanup(pool.Close)
	return pool
}

// readExportedSeriesDoc reads one exported series document off DISK. The
// on-disk bytes are deliberately the subject of every assertion in this
// file: publishing.Publish's in-memory return value could be correct while
// the write is not, and the Astro build only ever sees the file.
func readExportedSeriesDoc(t *testing.T, outDir, slug string) publishing.SeriesDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(outDir, "series", slug+".json"))
	if err != nil {
		t.Fatalf("reading the exported series doc for %s: %v", slug, err)
	}
	var doc publishing.SeriesDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshalling the exported series doc for %s: %v", slug, err)
	}
	return doc
}

func readExportedManifest(t *testing.T, outDir string) publishing.Manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("reading the exported manifest: %v", err)
	}
	var m publishing.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshalling the exported manifest: %v", err)
	}
	return m
}

func TestRunIngest_TheExportGateReflectsEachOutcomeKind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	pool := exportGatePostgres(t, "concontexto_export_gate_test")
	ctx := context.Background()

	// A VALIDATION FAILURE MUST REACH A READER. This is the case
	// CRITICAL-16 exists for, and the one no test covered.
	//
	// Run 1 publishes 2026-06 = 100 and exports normally. Run 2 serves the
	// same 2026-06 plus a 2026-07 = 5000 that breaches the series'
	// configured plausibility maximum, so the publish gate blocks the WHOLE
	// run (validation.Gate is all-or-nothing by design -- gate.go) and
	// records outcome='validation-failed' with zero observation writes.
	//
	// Everything the amended spec scenario asserts is then read back off
	// disk: the suspect 5000 is absent, the previously published 100 is
	// still the latest point, and pageState carries kind
	// "validation-failure" with lastCorrectUpdate naming run 1's date --
	// not run 2's, and not today's.
	t.Run("a validation-failed run exports the failure state without the suspect datum", func(t *testing.T) {
		const (
			slug = "test-gate-series-vf"
			cod  = "TESTGATEVF01"
		)
		payload := ineFixture(cod, ineFixtureEntry{month: "06", value: "100"})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(payload)
		}))
		defer server.Close()

		maxPlausible := 1000.0
		cfg := &config.Config{
			Sources: map[string]config.SourceConfig{
				"test-gate-src-vf": {ID: "test-gate-src-vf", Name: "Test", URL: "https://example.test", AccessType: "api-json",
					API:     &config.APIConfig{BaseURL: server.URL},
					Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
			},
			Series: []config.SeriesConfig{
				{Slug: slug, Name: "Test", Source: "test-gate-src-vf", Dataset: "test-gate-dataset-vf",
					Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
					Validation: config.ValidationConfig{Plausibility: config.PlausibilityConfig{Max: &maxPlausible}},
					SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: cod, ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}},
			},
		}

		root := t.TempDir()
		// APP_DATA_ROOT redirects retentionHistoryDir() away from the
		// container-only /app_data default, so retention archiving is
		// exercised rather than failing best-effort into stderr.
		t.Setenv("APP_DATA_ROOT", filepath.Join(root, "app_data"))
		store := filestore.NewStore(filepath.Join(root, "raw"))
		archiveHashPath := filepath.Join(root, "app_data", "raw_files.sha256")
		publicHashPath := filepath.Join(root, "public", "transparencia", "raw-files.sha256")
		outDir := filepath.Join(root, "data-derived")

		dispatches := 0
		dispatch := publishing.Dispatcher(func(context.Context, time.Time, string) error {
			dispatches++
			return nil
		})

		// Run 1: the last correct update. Its `now` is the date the banner
		// must later name, which is why the two runs are given distinct,
		// explicit days rather than both reading the clock.
		firstRunAt := time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)
		var stdout, stderr bytes.Buffer
		if code := runIngest(ctx, pool, cfg, store, archiveHashPath, publicHashPath, firstRunAt, slug, "", outDir, dispatch, &stdout, &stderr); code != 0 {
			t.Fatalf("first (valid) run: exit %d, stderr=%q", code, stderr.String())
		}
		// Checked before reading the artifact off disk so a publish that
		// FAILED reports its own reason (Publish's error reaches stderr)
		// rather than surfacing three assertions later as a bare
		// "no such file".
		if !strings.Contains(stdout.String(), "ingest: publish: exported and dispatched") {
			t.Fatalf("expected the first, valid run to export; got stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
		if got := readExportedSeriesDoc(t, outDir, slug).PageState.Kind; got != publishing.PageStateFresh {
			t.Fatalf("expected the first, valid run to export pageState %q, got %q", publishing.PageStateFresh, got)
		}
		if dispatches != 1 {
			t.Fatalf("expected the first run to dispatch exactly one rebuild, got %d", dispatches)
		}

		// Run 2: the source publishes a datum our validation rejects.
		payload = ineFixture(cod,
			ineFixtureEntry{month: "06", value: "100"},
			ineFixtureEntry{month: "07", value: "5000"},
		)
		secondRunAt := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
		stdout.Reset()
		stderr.Reset()
		if code := runIngest(ctx, pool, cfg, store, archiveHashPath, publicHashPath, secondRunAt, slug, "", outDir, dispatch, &stdout, &stderr); code != 0 {
			// A validation failure is a recorded outcome, not a runIngest
			// error: nothing crashed, the pipeline did its job and refused
			// the datum. Only a fetch/decode/infrastructure failure exits
			// non-zero.
			t.Fatalf("second (validation-failing) run: exit %d, stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "outcome=block") {
			t.Fatalf("expected the second run to report a blocked outcome, got stdout=%q", stdout.String())
		}

		// The gate ran the export. Without this the whole scenario is
		// unobservable to a reader, whatever the database holds.
		if !strings.Contains(stdout.String(), "ingest: publish: exported and dispatched") {
			t.Fatalf("expected a validation failure to STILL export and dispatch, so the §6.1.3 banner reaches a reader; got stdout=%q stderr=%q",
				stdout.String(), stderr.String())
		}
		if dispatches != 2 {
			t.Fatalf("expected the validation-failed run to dispatch its own rebuild (2 total), got %d", dispatches)
		}

		// A NEW artifact, not the stale one run 1 left behind.
		if generatedAt := readExportedManifest(t, outDir).GeneratedAt; !generatedAt.Equal(secondRunAt) {
			t.Errorf("expected a freshly generated manifest stamped %s, got %s -- the artifact a reader is served is still run 1's",
				secondRunAt.Format(time.RFC3339), generatedAt.Format(time.RFC3339))
		}

		doc := readExportedSeriesDoc(t, outDir, slug)

		// "no suspect datum enters the artifact" -- the strongest half of
		// the amended requirement, and the half the publish gate already
		// guaranteed structurally (a blocked run writes no observation, so
		// the export cannot read one back). Asserted anyway, because the
		// export now runs on this path for the first time and "the export
		// runs after a failure" must never come to mean "the failure's
		// datum gets published".
		for _, p := range doc.Points {
			if p.Period == "2026-07" {
				t.Errorf("the suspect period 2026-07 reached the artifact: %+v", p)
			}
			if p.Value == 5000 {
				t.Errorf("the suspect value 5000 reached the artifact at %s", p.Period)
			}
		}

		// "the last valid value remains, and it is the latest shown".
		if len(doc.Points) != 1 {
			t.Fatalf("expected exactly the one previously published point to survive, got %+v", doc.Points)
		}
		if doc.Points[0].Period != "2026-06" || doc.Points[0].Value != 100 {
			t.Errorf("expected the previously published 2026-06=100 to be the latest point, got %+v", doc.Points[0])
		}

		// "the failure state is exported" -- kind AND date. The date is the
		// banner's own copy ("Última actualización correcta: {fecha}"), so a
		// wrong one is a lie told to a reader, not a cosmetic slip.
		if doc.PageState.Kind != publishing.PageStateValidationFailure {
			t.Fatalf("expected pageState.kind %q on disk, got %q -- PRD §6.1.3's banner cannot render",
				publishing.PageStateValidationFailure, doc.PageState.Kind)
		}
		if doc.PageState.LastCorrectUpdate == nil {
			t.Fatal("expected lastCorrectUpdate to name the last succeeded run's date, got null")
		}
		if *doc.PageState.LastCorrectUpdate != "2026-07-28" {
			t.Errorf("expected lastCorrectUpdate 2026-07-28 (run 1, the last SUCCEEDED run), got %q -- the banner would name the wrong date",
				*doc.PageState.LastCorrectUpdate)
		}
	})

	// A FETCH FAILURE MUST NOT EXPORT. The three failing outcome kinds are
	// not interchangeable: a fetch failure produced no payload, so
	// IngestSeries records a download_attempt with no resulting hash, never
	// creates an ingestion_run row at all, and returns the zero Result. The
	// database therefore holds no new fact about this series -- neither a
	// datum nor a validation verdict -- and re-exporting would rewrite the
	// artifact with a new generated_at and dispatch a rebuild that changes
	// nothing a reader can see. That is the "no new artifact" half of the
	// original requirement that SURVIVES the narrowing.
	t.Run("a fetch failure with no payload exports nothing", func(t *testing.T) {
		const (
			slug = "test-gate-series-fetchfail"
			cod  = "TESTGATEFF01"
		)
		// 404, not 500: a refused request classifies as non-retryable and
		// fails on the first attempt (ine.Client.fetchWithRetry), so this
		// subtest never waits on the real backoff curve.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		cfg := &config.Config{
			Sources: map[string]config.SourceConfig{
				"test-gate-src-ff": {ID: "test-gate-src-ff", Name: "Test", URL: "https://example.test", AccessType: "api-json",
					API:     &config.APIConfig{BaseURL: server.URL},
					Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
			},
			Series: []config.SeriesConfig{
				{Slug: slug, Name: "Test", Source: "test-gate-src-ff", Dataset: "test-gate-dataset-ff",
					Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
					SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: cod, ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}},
			},
		}

		root := t.TempDir()
		t.Setenv("APP_DATA_ROOT", filepath.Join(root, "app_data"))
		store := filestore.NewStore(filepath.Join(root, "raw"))
		outDir := filepath.Join(root, "data-derived")

		dispatched := false
		dispatch := publishing.Dispatcher(func(context.Context, time.Time, string) error {
			dispatched = true
			return nil
		})

		var stdout, stderr bytes.Buffer
		code := runIngest(ctx, pool, cfg, store,
			filepath.Join(root, "app_data", "raw_files.sha256"),
			filepath.Join(root, "public", "transparencia", "raw-files.sha256"),
			time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC), slug, "", outDir, dispatch, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected a fetch failure to exit non-zero, got 0 (stdout=%q)", stdout.String())
		}
		if dispatched {
			t.Error("expected no rebuild dispatch for a run that fetched nothing")
		}
		if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); !os.IsNotExist(err) {
			t.Errorf("expected no artifact written for a run that fetched nothing, got err=%v", err)
		}
	})

	// A CONFIGURATION FAILURE THAT NEVER REACHES A SOURCE MUST NOT EXPORT
	// EITHER. The series resolves to no active source_ref, so runIngest
	// fails it before any client is built -- no download attempt, no run
	// row, no verdict. Same reason as the fetch failure, one layer earlier;
	// asserted separately because it takes a different code path out of the
	// per-series loop and would be the easiest kind to collapse
	// accidentally into "any failure exports".
	t.Run("a series with no resolvable source ref exports nothing", func(t *testing.T) {
		const slug = "test-gate-series-noref"
		// RETIRED, not future-dated: postgres.ActiveSourceRef resolves "active"
		// as ValidTo == nil and deliberately ignores ValidFrom, so a
		// future-dated ref with an open end is still active and would send this
		// subtest down the network path instead -- proving nothing this file
		// does not already prove, five seconds of real DNS backoff at a time.
		// A closed ValidTo is the only shape that genuinely resolves to no
		// active ref, and the stderr assertion below pins the path taken.
		retired := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
		cfg := &config.Config{
			Sources: map[string]config.SourceConfig{
				"test-gate-src-noref": {ID: "test-gate-src-noref", Name: "Test", URL: "https://example.test", AccessType: "api-json",
					API:     &config.APIConfig{BaseURL: "https://example.invalid"},
					Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
			},
			Series: []config.SeriesConfig{
				{Slug: slug, Name: "Test", Source: "test-gate-src-noref", Dataset: "test-gate-dataset-noref",
					Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
					SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "TESTGATENR01",
						ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ValidTo: &retired}}},
			},
		}

		root := t.TempDir()
		t.Setenv("APP_DATA_ROOT", filepath.Join(root, "app_data"))
		outDir := filepath.Join(root, "data-derived")

		dispatched := false
		dispatch := publishing.Dispatcher(func(context.Context, time.Time, string) error {
			dispatched = true
			return nil
		})

		var stdout, stderr bytes.Buffer
		code := runIngest(ctx, pool, cfg, filestore.NewStore(filepath.Join(root, "raw")), "", "",
			time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC), slug, "", outDir, dispatch, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected an unresolvable source ref to exit non-zero, got 0 (stdout=%q)", stdout.String())
		}
		// Pins the PATH, not merely the exit code: without this the subtest
		// would still pass if the series had quietly reached the network and
		// failed there, which is a different outcome kind already covered
		// above.
		if !strings.Contains(stderr.String(), "no active source_ref") {
			t.Fatalf("expected the failure to be the unresolvable source ref itself, got stderr=%q", stderr.String())
		}
		if dispatched {
			t.Error("expected no rebuild dispatch for a series that never reached its source")
		}
		if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); !os.IsNotExist(err) {
			t.Errorf("expected no artifact written for a series that never reached its source, got err=%v", err)
		}
	})
}
