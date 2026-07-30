package ingestion_test

// Task 5b.5 (RED) / 5b.6 (GREEN): the end-to-end proof for milestone 0.2
// (spec source-ingestion-ine, "All six series load full history and
// validate"): each of the six pinned series, given its real trimmed
// fixture, MUST load its complete observation history, pass every
// applicable validation rule, and produce observations carrying source,
// origin COD, extraction timestamp and raw-file hash.
//
// Dimension rows (source/dataset/series/series_source_mapping) are
// seeded directly here, standing in for ReconcileEditorialConfig
// (design.md lists it as a SEPARATE ingestion/ function, not built in
// this batch) -- IngestSeries itself assumes they already exist.

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// sixSeries mirrors config/series/*.yaml's six milestone-0.2 entries
// (spec source-ingestion-ine's own table). Each entry's fixture file is
// named by SLUG (testdata/datos_serie/{slug}.json); the series COD is
// read from the fixture's own top-level "COD" field at test time (real
// DATA inside a testdata/ file) rather than hard-coded as a Go string
// literal, matching the origin-identifier guard's existing convention.
type sixSeriesCase struct {
	slug      string
	datasetID string
	unit      string
	frequency indicators.Frequency
	decimals  int
}

var sixSeries = []sixSeriesCase{
	{slug: "tasa-de-paro-epa", datasetID: "ine-epa", unit: "% población activa", frequency: indicators.FrequencyQuarterly, decimals: 2},
	{slug: "ocupados-epa", datasetID: "ine-epa", unit: "miles de personas", frequency: indicators.FrequencyQuarterly, decimals: 1},
	{slug: "ipc-general", datasetID: "ine-ipc", unit: "índice", frequency: indicators.FrequencyMonthly, decimals: 3},
	{slug: "ipc-subyacente", datasetID: "ine-ipc", unit: "índice", frequency: indicators.FrequencyMonthly, decimals: 3},
	{slug: "pib-cvi", datasetID: "ine-cntr", unit: "índice de volumen encadenado", frequency: indicators.FrequencyQuarterly, decimals: 4},
	{slug: "poblacion-residente", datasetID: "ine-ecp", unit: "personas", frequency: indicators.FrequencyQuarterly, decimals: 0},
}

// loadFixture reads testdata/datos_serie/{slug}.json and returns its
// bytes plus the COD its own "COD" field carries -- no index/order
// coupling between the six slugs and six fixtures, and no COD ever
// appears as a Go string literal.
func loadFixture(t *testing.T, slug string) (cod string, body []byte) {
	t.Helper()
	path := filepath.Join("testdata", "datos_serie", slug+".json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	var wire struct {
		COD string `json:"COD"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("parsing COD out of fixture %s: %v", path, err)
	}
	if wire.COD == "" {
		t.Fatalf("fixture %s carries no COD field", path)
	}
	return wire.COD, body
}

func mustExecT(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// seedDimensions inserts the source/dataset/series/series_source_mapping
// rows IngestSeries assumes already exist (ReconcileEditorialConfig's
// job, not built this batch -- see the package doc comment above).
func seedDimensions(t *testing.T, ctx context.Context, tx pgx.Tx, sc sixSeriesCase, cod string) {
	t.Helper()
	mustExecT(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://ine.es', 'Reutilización con atribución', 'Fuente: INE', 'api-json', 'digest1')
		ON CONFLICT (id) DO NOTHING`)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ($1, 'ine', $1, 'digest1') ON CONFLICT (id) DO NOTHING`, sc.datasetID)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ($1, $2, $1, $3, $4, 'ES', $5, false, 'digest1')`,
		sc.slug, sc.datasetID, sc.unit, string(sc.frequency), sc.decimals)
	mustExecT(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ($1, 'ine-series-cod', $2, '2026-07-28', 'digest1')`, sc.slug, cod)
}

// shippedConfig loads the /config tree exactly as production does --
// configdata.FS -> fs.Sub -> config.Load, the same three calls
// `validate-config` and every `ingest` invocation make. Loaded once for
// the whole package: it is read-only, and every caller below wants the
// same bytes.
var shippedConfig = sync.OnceValues(func() (*config.Config, error) {
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		return nil, err
	}
	return config.Load(sub)
})

// realValidationConfig returns the `validation:` block
// config/series/{slug}.yaml actually ships, or the zero value for a slug
// the shipped config does not define.
//
// verify-report CRITICAL-37. Every IngestSeries call in this package used
// to pass `config.ValidationConfig{}` -- literally no thresholds --
// INCLUDING the end-to-end export test that
// .github/workflows/ingest-export-build.yml runs. So the one CI job
// proving the Go->Astro hand-off ran the pipeline with the guard that
// blocks a series in production switched off, and no fixture in this
// repository could have made it red: with no thresholds there is no
// finding, and with the last-3-period fixtures there would have been no
// breach even if there had been thresholds. A rule that cannot be shown
// failing is not a check.
//
// Reading the block from the shipped YAML rather than restating it as Go
// literals is the load-bearing half. A restated threshold is a second
// source of truth: raising `max_delta_abs` in config/series/*.yaml would
// leave every restating test green while the production guard went blind,
// which is the same defect one level down.
//
// THE ZERO-VALUE BRANCH IS FOR SYNTHETIC SLUGS ONLY (`test-break-wiring`,
// `test-e2e-export` and friends), which have no shipped configuration and
// legitimately have no thresholds. It is not a quiet fallback for the six:
// TestIneIngestConfig_CarriesEveryShippedThresholdForTheSixFrozenSlugs
// pins that none of them ever takes it.
func realValidationConfig(t *testing.T, slug string) config.ValidationConfig {
	t.Helper()
	cfg, err := shippedConfig()
	if err != nil {
		t.Fatalf("loading the shipped /config tree: %v", err)
	}
	for _, s := range cfg.Series {
		if s.Slug == slug {
			return s.Validation
		}
	}
	return config.ValidationConfig{}
}

// ineIngestConfig builds the SeriesIngestConfig ReconcileEditorialConfig
// would eventually resolve for sc -- including, since CRITICAL-37, the
// real shipped validation thresholds (see realValidationConfig).
func ineIngestConfig(t *testing.T, sc sixSeriesCase, cod string) ingestion.SeriesIngestConfig {
	t.Helper()
	return ingestion.SeriesIngestConfig{
		SourceID:  "ine",
		DatasetID: sc.datasetID,
		SeriesID:  sc.slug,
		COD:       cod,
		Series: indicators.Series{
			Slug: sc.slug, Unit: sc.unit, Frequency: sc.frequency, Decimals: sc.decimals,
			Source: "ine", Licence: "Reutilización con atribución (condiciones INE)",
		},
		Validation: realValidationConfig(t, sc.slug),
	}
}

// TestIneIngestConfig_CarriesEveryShippedThresholdForTheSixFrozenSlugs is
// the structural half of CRITICAL-37, and it is deliberately blunt: for
// each of the six frozen slugs, the config every IngestSeries test in this
// package hands the pipeline must equal the `validation:` block the YAML
// ships, field for field.
//
// It exists because the behavioural proof
// (e2e_blocked_export_test.go) can only demonstrate ONE series' threshold
// biting on ONE quarter. Five of the six could quietly revert to
// `config.ValidationConfig{}` and that test would still pass. This one
// notices.
func TestIneIngestConfig_CarriesEveryShippedThresholdForTheSixFrozenSlugs(t *testing.T) {
	cfg, err := shippedConfig()
	if err != nil {
		t.Fatalf("loading the shipped /config tree: %v", err)
	}
	shipped := map[string]config.ValidationConfig{}
	for _, s := range cfg.Series {
		shipped[s.Slug] = s.Validation
	}

	for _, sc := range sixSeries {
		want, ok := shipped[sc.slug]
		if !ok {
			t.Errorf("frozen slug %q has no config/series/%s.yaml entry in the shipped tree", sc.slug, sc.slug)
			continue
		}
		// A shipped series with NO plausibility bound would make the
		// comparison below vacuously true, so the fixture of this test --
		// the shipped config itself -- is checked first.
		if want.Plausibility.MaxDeltaAbs == nil && want.Plausibility.Min == nil && want.Plausibility.Max == nil {
			t.Errorf("config/series/%s.yaml declares no plausibility bound at all; this test would pass vacuously for it", sc.slug)
		}

		got := ineIngestConfig(t, sc, "irrelevant-for-this-assertion").Validation
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ineIngestConfig(%s).Validation must be the block config/series/%s.yaml ships.\n got: %+v\nwant: %+v",
				sc.slug, sc.slug, got, want)
		}
	}
}

// TestIngestSeries_AllSixSeriesLoadTheirTrimmedFixtureHistoryAndValidate
// -- renamed (remediation batch, verify-report WARNING W5): the previous
// name said "FullHistory" but every assertion below is against the
// checked-in 3-period TRIMMED fixture (loadFixture's own doc comment;
// spec source-ingestion-ine itself mandates trimmed INE fixtures), not a
// full historical load. The genuine full-history sweep (306 real rows)
// is proven separately for XLSX, source-ingestion-xlsx's own
// TestDecode_FullHistoryEveryRowSatisfiesArithmeticInvariant. This test
// still proves what its assertions actually claim: all six series load,
// publish and satisfy provenance across their 3-period fixture.
func TestIngestSeries_AllSixSeriesLoadTheirTrimmedFixtureHistoryAndValidate(t *testing.T) {
	for _, sc := range sixSeries {
		sc := sc
		t.Run(sc.slug, func(t *testing.T) {
			ctx := context.Background()
			tx := newTx(t)
			if err := postgres.NewRunner(tx).Up(ctx); err != nil {
				t.Fatalf("Up: %v", err)
			}

			cod, fixture := loadFixture(t, sc.slug)
			seedDimensions(t, ctx, tx, sc, cod)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(fixture)
			}))
			defer server.Close()

			client := ine.NewClient(server.URL, server.Client())
			store := filestore.NewStore(t.TempDir())
			now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

			result, err := ingestion.IngestSeries(ctx, tx, store, client, ineIngestConfig(t, sc, cod), now)
			if err != nil {
				t.Fatalf("IngestSeries(%s): %v", sc.slug, err)
			}
			if result.Outcome != validation.GatePublish {
				t.Fatalf("expected %s to publish, got outcome=%v findings=%+v", sc.slug, result.Outcome, result.Findings)
			}
			if len(result.Published) != 3 {
				t.Fatalf("expected %s to load its full 3-period fixture history, got %d observations", sc.slug, len(result.Published))
			}

			// Every observation carries source, origin COD, extraction
			// timestamp and raw-file hash (spec "each produces
			// observations with source, origin COD, extraction timestamp
			// and raw-file hash").
			for _, obs := range result.Published {
				prov, err := postgres.ResolveProvenance(ctx, tx, sc.slug, obs.Period)
				if err != nil {
					t.Fatalf("ResolveProvenance(%s, %s): %v", sc.slug, obs.Period, err)
				}
				if prov.SourceID != "ine" {
					t.Errorf("expected SourceID ine, got %q", prov.SourceID)
				}
				if prov.OriginRef != cod {
					t.Errorf("expected OriginRef %s, got %q", cod, prov.OriginRef)
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

// TestIngestSeries_TransportFailureRecordsDownloadAttemptAndWritesNothing
// proves the pipeline's other safety story half: when the FETCH itself
// never completes (every retry exhausted against a persistent 503), the
// failure is recorded as a download_attempt with no resulting hash and
// IngestSeries returns the classified error -- and because there were no
// raw bytes to archive at all, NOTHING else is written: no raw_file, no
// ingestion_run, no observation.
func TestIngestSeries_TransportFailureRecordsDownloadAttemptAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sc := sixSeries[0]
	seedDimensions(t, ctx, tx, sc, "test-transport-failure")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client(),
		ine.WithMaxAttempts(2), ine.WithSleep(func(time.Duration) {}))
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	_, err := ingestion.IngestSeries(ctx, tx, store, client, ineIngestConfig(t, sc, "test-transport-failure"), now)
	if err == nil {
		t.Fatal("expected an exhausted-retry transport failure to fail IngestSeries")
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.RetryableTransport {
		t.Errorf("expected class %s, got %s", sourceerr.RetryableTransport, classified.Class)
	}

	var attemptCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM download_attempt WHERE source_id='ine'`).Scan(&attemptCount); err != nil {
		t.Fatalf("counting download_attempt rows: %v", err)
	}
	if attemptCount != 1 {
		t.Fatalf("expected exactly 1 download_attempt row, got %d", attemptCount)
	}
	var outcome string
	var resultingHash *string
	if err := tx.QueryRow(ctx, `SELECT outcome, resulting_hash FROM download_attempt WHERE source_id='ine'`).Scan(&outcome, &resultingHash); err != nil {
		t.Fatalf("reading the download_attempt row: %v", err)
	}
	if outcome != string(postgres.OutcomeRetryableTransport) {
		t.Errorf("expected outcome %q, got %q", postgres.OutcomeRetryableTransport, outcome)
	}
	if resultingHash != nil {
		t.Errorf("expected a nil resulting_hash on a failed attempt, got %v", *resultingHash)
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
	if runCount != 0 || obsCount != 0 || rawFileCount != 0 {
		t.Errorf("expected zero ingestion_run/observation/raw_file rows on a transport failure, got runs=%d observations=%d raw_files=%d", runCount, obsCount, rawFileCount)
	}
}

// TestIngestSeries_RefusalEnvelopeArchivesButBlocksPublication proves the
// pipeline's ORDERING guarantee for exactly the case that motivates it
// (Finding A/Engram #4690): the volume-restriction envelope is HTTP 200,
// so it is indistinguishable from success until the body is decoded --
// meaning the raw bytes are ALREADY archived (raw_file, ingestion_run)
// by the time decode discovers the refusal. The evidence survives even
// though nothing is published: this is exactly "archive the raw payload
// BEFORE parsing ... the evidence survives even when parsing ... fails."
func TestIngestSeries_RefusalEnvelopeArchivesButBlocksPublication(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sc := sixSeries[0]
	seedDimensions(t, ctx, tx, sc, "test-refused-series")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status" : "No puede mostrarse por restricciones de volumen"}`))
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	result, err := ingestion.IngestSeries(ctx, tx, store, client, ineIngestConfig(t, sc, "test-refused-series"), now)
	if err == nil {
		t.Fatal("expected the volume-restriction refusal to fail IngestSeries")
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
		t.Errorf("expected exactly 1 raw_file row (the refusal bytes ARE archived -- this is the whole point), got %d", rawFileCount)
	}
	if obsCount != 0 {
		t.Errorf("expected zero observation rows, got %d", obsCount)
	}

	var recordedOutcome string
	if err := tx.QueryRow(ctx, `SELECT outcome FROM ingestion_run WHERE id=$1`, result.RunID).Scan(&recordedOutcome); err != nil {
		t.Fatalf("reading the run's recorded outcome: %v", err)
	}
	if recordedOutcome != string(postgres.RunOutcomeValidationFailed) {
		t.Errorf("expected the run's recorded outcome to be %q, got %q", postgres.RunOutcomeValidationFailed, recordedOutcome)
	}
}

// TestIngestSeries_ResolvedBreaksFromPostgresExemptRule3AtTheBreakPeriod is
// the remediation batch's proof for verify-report CRITICAL C1:
// IngestSeries MUST resolve this series' active breaks from postgres
// (postgres.ResolveActiveBreaksForSeries) and pass them into
// validation.SeriesContext.Breaks, not leave the field permanently nil.
// A recorded break at the SAME period as an oversized period-over-period
// jump MUST exempt rule 3 (spec data-validation, "A large jump at a
// recorded break passes") -- the asymmetry this proves against the same
// jump with no break is already covered at the unit level by
// TestRule3Plausibility_SameJumpWithNoRecordedBreakFails; this test is the
// missing end-to-end half nothing previously exercised: a break recorded
// in POSTGRES (not hand-built in a SeriesContext literal) reaching the
// rule at all.
func TestIngestSeries_ResolvedBreaksFromPostgresExemptRule3AtTheBreakPeriod(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sc := sixSeriesCase{slug: "test-break-wiring", datasetID: "test-break-dataset", unit: "índice", frequency: indicators.FrequencyMonthly, decimals: 1}
	cod := "TESTBRK001"
	seedDimensions(t, ctx, tx, sc, cod)

	breakDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if _, err := postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{{
		BreakKey: "test-break-wiring-break", ScopeKind: "series", ScopeRef: sc.slug,
		Date: breakDate, Kind: "methodology", NoteMD: "test break for C1's wiring proof", ConfigDigest: "digest1",
	}}); err != nil {
		t.Fatalf("ReconcileBreaks: %v", err)
	}

	// M04 -> M05 jumps from 10 to 100 (delta 90), far above the 50
	// threshold below -- blocked UNLESS the break just seeded at M05 is
	// actually resolved and passed through.
	fixture := []byte(`{"COD":"` + cod + `", "Nombre":"test", "T3_Unidad":"indice", "T3_Escala":" ", "Data":[` +
		`{"Fecha":"2026-04-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M04", "Anyo":2026, "Valor":10}` +
		`,{"Fecha":"2026-05-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M05", "Anyo":2026, "Valor":100}` +
		`]}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	maxDelta := 50.0
	cfg := ingestion.SeriesIngestConfig{
		SourceID: "ine", DatasetID: sc.datasetID, SeriesID: sc.slug, COD: cod,
		Series:     indicators.Series{Slug: sc.slug, Unit: sc.unit, Frequency: sc.frequency, Decimals: sc.decimals, Source: "ine", Licence: "test"},
		Validation: config.ValidationConfig{Plausibility: config.PlausibilityConfig{MaxDeltaAbs: &maxDelta}},
	}

	result, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now)
	if err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected the break recorded in postgres at M05 to exempt the jump and publish, got outcome=%v findings=%+v", result.Outcome, result.Findings)
	}
}

// TestIngestSeries_PublishesTheRawFileHashListingWhenPathsAreConfigured is
// the remediation batch's proof for verify-report CRITICAL C3:
// postgres.PublishRawFileHashListing had no production call site, so
// /public/transparencia/raw-files.sha256 never existed. IngestSeries MUST
// refresh both copies as part of the pipeline (spec raw-file-archive, "The
// listing MUST be refreshed as part of the ingestion pipeline") whenever
// SeriesIngestConfig names where they live. Leaving both paths at their
// zero value (every OTHER test in this file) MUST keep skipping
// publication -- proven implicitly by every other IngestSeries test in
// this file never creating a listing file anywhere.
func TestIngestSeries_PublishesTheRawFileHashListingWhenPathsAreConfigured(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sc := sixSeries[0]
	cod, fixture := loadFixture(t, sc.slug)
	seedDimensions(t, ctx, tx, sc, cod)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	root := t.TempDir()
	archivePath := filepath.Join(root, "app_data", "raw_files.sha256")
	publicPath := filepath.Join(root, "public", "transparencia", "raw-files.sha256")

	cfg := ineIngestConfig(t, sc, cod)
	cfg.HashListingArchivePath = archivePath
	cfg.HashListingPublicPath = publicPath

	if _, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now); err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}

	archived, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("reading the app_data hash listing: %v", err)
	}
	published, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatalf("reading the published /public hash listing: %v", err)
	}
	if string(archived) != string(published) {
		t.Fatal("expected the app_data and public copies to be byte-identical")
	}
	if !strings.Contains(string(published), "ine") {
		t.Errorf("expected the published listing to name source ine, got:\n%s", string(published))
	}
}
