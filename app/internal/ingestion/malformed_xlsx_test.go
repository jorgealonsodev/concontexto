package ingestion_test

// Task 8.10/8.11 (the state half): per the orchestrator's explicit design
// decision for slice 8b -- "use BOTH, because the spec asks for two
// different things" -- app/internal/adapters/xlsx/malformed_test.go and
// app/internal/adapters/postgres/gate_spy_test.go together prove the
// malformed-file suite's INTERACTION property (Decode fails; the writer
// is never called) for every one of the nine listed cases, generically.
//
// This file is the ONE testcontainers test the design decision calls
// for: a real Postgres, the real `one_current_row` schema, a real
// malformed XLSX workbook served over HTTP, and the full
// ingestion.IngestSeries pipeline -- proving the STATE property a real
// database uniquely can: "the current observation for (series, period)
// is still V at its original version" after a malformed run, not merely
// "the writer was never called" (a real database cannot distinguish
// never-called from called-then-rolled-back on its own -- that is
// exactly why the spy test exists separately).
//
// The chosen case is partial-success-within-one-workbook -- "the
// sharpest case" (batch instructions): a workbook where two rows parse
// perfectly and a third is malformed. This is the case most worth
// proving all the way through the real stack, because it is also where
// a subtly wrong implementation would be most likely to leak a partial
// write (e.g. a naive per-row publish loop instead of parse-then-publish
// as one unit) -- decode.go's own contract (validate-before-write,
// all-or-nothing) makes that impossible by construction, and this test
// makes that a falsifiable fact against a real database rather than an
// inference from Go source alone.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
)

// malformedXLSXSchema mirrors app/internal/adapters/xlsx's own
// malformedSuiteSchema (period A, components B/C, total D) -- kept as a
// separate, small definition here rather than importing the test-only
// helper across packages (Go test helpers are not exported), so this
// file stays self-contained and its fixture is inspectable in place.
func malformedXLSXSchema() config.XLSXSchemaConfig {
	return config.XLSXSchemaConfig{
		SheetName:         "Hoja1",
		HeaderRow:         1,
		ColumnAnchors:     map[string]string{"period": "A", "total": "D"},
		HeaderFingerprint: "ignored-by-decode-itself",
		TotalColumn:       "D",
		ComponentColumns:  []string{"B", "C"},
		Tolerance:         0.01,
	}
}

// buildMalformedPartialSuccessWorkbook returns raw bytes for a workbook
// where two data rows parse cleanly and a third carries text in the
// total column -- the same construction as
// app/internal/adapters/xlsx's TestDecode_PartialSuccessWithinOneWorkbookWritesNothing,
// duplicated here (not imported -- test helpers do not cross package
// boundaries in Go) so this end-to-end test does not depend on the unit
// test package at all.
func buildMalformedPartialSuccessWorkbook(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Hoja1")

	set := func(cell, value string) {
		if err := f.SetCellStr("Hoja1", cell, value); err != nil {
			t.Fatalf("SetCellStr(%s): %v", cell, err)
		}
	}
	setNum := func(cell string, value float64) {
		if err := f.SetCellFloat("Hoja1", cell, value, -1, 64); err != nil {
			t.Fatalf("SetCellFloat(%s): %v", cell, err)
		}
	}

	set("A1", "Periodo")
	set("B1", "ComponenteUno")
	set("C1", "ComponenteDos")
	set("D1", "Total")

	set("A2", "Enero 2001")
	setNum("B2", 10)
	setNum("C2", 20)
	setNum("D2", 30) // valid: 10+20=30

	set("A3", "Febrero 2001")
	setNum("B3", 11)
	setNum("C3", 21)
	setNum("D3", 32) // valid: 11+21=32

	set("A4", "Marzo 2001")
	setNum("B4", 12)
	setNum("C4", 22)
	set("D4", "N/D") // malformed: text in the total column

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("writing malformed partial-success workbook: %v", err)
	}
	return buf.Bytes()
}

// seedMalformedTestIngestionRun mirrors
// observation_writer_test.go's own seedIngestionRun (package
// postgres_test), duplicated here in ingestion_test against this file's
// own dataset/series ids -- Go test harnesses are not shared across
// packages.
func seedMalformedTestIngestionRun(t *testing.T, ctx context.Context, tx pgx.Tx, sourceID, datasetID, seriesID, hash string, startedAt time.Time) int64 {
	t.Helper()
	mustExecT(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ($1, $2, 'https://seg-social.example/data', $3, '/app_data/'||$1, 100)`, hash, sourceID, startedAt)

	row := tx.QueryRow(ctx, `INSERT INTO ingestion_run (dataset_id, series_id, started_at, finished_at, raw_file_hash, outcome)
		VALUES ($1, $2, $3, $3, $4, 'succeeded') RETURNING id`, datasetID, seriesID, startedAt, hash)
	var id int64
	if err := row.Scan(&id); err != nil {
		t.Fatalf("seedMalformedTestIngestionRun: %v", err)
	}
	return id
}

func ptrMalformed(v float64) *float64 { return &v }

// TestIngestSeries_XLSXPartialSuccessWithinOneWorkbookWritesNothingAndPreservesThePublishedDatum
// is this batch's mandated end-to-end proof (see the package doc comment
// above): a real Postgres, the real writer, and a real malformed
// workbook served over HTTP -- the previously published datum for
// (series, period) MUST still be exactly V at its original version after
// the malformed run, and no new observation row may exist for either of
// the two rows that individually parsed cleanly.
func TestIngestSeries_XLSXPartialSuccessWithinOneWorkbookWritesNothingAndPreservesThePublishedDatum(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	const sourceID = "seg-social-test"
	const slug = "afiliacion-ss-malformed-test"
	const datasetID = "seg-social-malformed-test"

	mustExecT(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ($1, 'Seguridad Social (test)', 'https://seg-social.example', 'lic', 'attr', 'xlsx-download', 'digest1')`, sourceID)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ($1, $2, $1, 'digest1')`, datasetID, sourceID)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ($1, $2, $1, 'personas', 'M', 'ES', 2, false, 'digest1')`, slug, datasetID)

	// Seed the previously published current observation for the period
	// the malformed workbook's FIRST (individually valid) row would also
	// cover -- proving it is untouched is the sharpest version of "the
	// published datum is unchanged": the malformed run's own workbook
	// contains a row that, in isolation, would have produced the exact
	// same period.
	seedRun := seedMalformedTestIngestionRun(t, ctx, tx, sourceID, datasetID, slug, "hash-seed-run", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	const publishedValue = 12345.67
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: slug, Period: "2001-01", Value: ptrMalformed(publishedValue),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: seedRun,
	}); err != nil {
		t.Fatalf("seeding the already-published observation: %v", err)
	}

	raw := buildMalformedPartialSuccessWorkbook(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
	}))
	defer server.Close()

	client := xlsx.NewClient(malformedXLSXSchema(), server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	cfg := ingestion.SeriesIngestConfig{
		SourceID: sourceID, DatasetID: datasetID, SeriesID: slug,
		COD: server.URL, // an xlsx-url ref IS the request URL itself
		Series: indicators.Series{
			Slug: slug, Unit: "personas", Frequency: indicators.FrequencyMonthly, Decimals: 2,
			Source: sourceID, Licence: "test licence",
		},
	}

	_, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now)
	if err == nil {
		t.Fatal("expected the malformed (partial-success) workbook to fail ingestion")
	}

	// The previously published datum is untouched: same value, same
	// version, still current.
	var value float64
	var version int
	var isCurrent bool
	row := tx.QueryRow(ctx, `SELECT value, version, is_current FROM observation WHERE series_id=$1 AND period='2001-01' AND is_current`, slug)
	if err := row.Scan(&value, &version, &isCurrent); err != nil {
		t.Fatalf("reading the current observation after the malformed run: %v", err)
	}
	if value != publishedValue || version != 1 || !isCurrent {
		t.Fatalf("expected the original published observation (value=%v version=1 current=true) to be untouched, got value=%v version=%v current=%v",
			publishedValue, value, version, isCurrent)
	}

	// Nothing from ANY row of the malformed workbook was written -- not
	// even the two individually valid rows (Enero 2001 already existed
	// with a different run; Febrero 2001 never existed at all).
	var totalRows, februaryRows int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id=$1`, slug).Scan(&totalRows); err != nil {
		t.Fatalf("counting observation rows for %s: %v", slug, err)
	}
	if totalRows != 1 {
		t.Fatalf("expected exactly the one pre-existing observation row (zero writes from the malformed run), got %d", totalRows)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id=$1 AND period='2001-02'`, slug).Scan(&februaryRows); err != nil {
		t.Fatalf("counting Febrero 2001 observation rows for %s: %v", slug, err)
	}
	if februaryRows != 0 {
		t.Fatalf("expected zero rows for the individually-valid Febrero 2001 row (all-or-nothing), got %d", februaryRows)
	}
}
