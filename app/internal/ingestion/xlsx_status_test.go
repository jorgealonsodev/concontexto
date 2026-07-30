package ingestion_test

// Slice 2c (corrective slice, apply-progress): closes the gap slice 2b
// disclosed but deliberately did not fix (out of that slice's file
// scope) -- adapters/xlsx does not classify indicators.Observation.Status
// at all, so a real, successfully-decoded XLSX ingest cycle would BLOCK
// on ingest.go's firstUnclassifiedStatus guard rather than publish. No
// prior XLSX/IngestSeries test reaches that guard: the one that exists
// (malformed_xlsx_test.go's own TestIngestSeries_XLSXPartialSuccess...)
// fails at decode, before firstUnclassifiedStatus is ever evaluated --
// which is exactly why the gap survived `go test ./...` staying green.
//
// This file proves the opposite path: a workbook with NO malformed rows,
// decoded successfully end to end through the real Postgres writer, MUST
// publish -- proving the guard is satisfied by adapters/xlsx's new
// classification rather than merely unreached.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
)

// xlsxStatusTestSchema mirrors malformed_xlsx_test.go's own
// malformedXLSXSchema (period A, components B/C, total D) -- kept as its
// own small definition for the same reason that file gives: Go test
// helpers do not cross package boundaries, and this file must stay
// self-contained.
func xlsxStatusTestSchema() config.XLSXSchemaConfig {
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

// buildCleanWorkbook returns raw bytes for a workbook where every data
// row parses cleanly -- unlike malformed_xlsx_test.go's fixture, no row
// here is malformed, so decode succeeds and the pipeline reaches
// ingest.go's candidate-building loop and firstUnclassifiedStatus guard.
func buildCleanWorkbook(t *testing.T) []byte {
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

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("writing clean workbook: %v", err)
	}
	return buf.Bytes()
}

// TestIngestSeries_XLSXSuccessfulDecodePublishesDefinitiveWithNullSourceStatus
// is this slice's mandated proof: a real Postgres, the real writer, and a
// workbook with zero malformed rows -- the run MUST publish (not block
// on firstUnclassifiedStatus), and every published observation MUST
// carry status='D' (Definitive) with source_status NULL, because the
// Social Security workbook publishes no status concept of its own.
func TestIngestSeries_XLSXSuccessfulDecodePublishesDefinitiveWithNullSourceStatus(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	const sourceID = "seg-social-test"
	const slug = "afiliacion-ss-status-test"
	const datasetID = "seg-social-status-test"

	mustExecT(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ($1, 'Seguridad Social (test)', 'https://seg-social.example', 'lic', 'attr', 'xlsx-download', 'digest1')`, sourceID)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ($1, $2, $1, 'digest1')`, datasetID, sourceID)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ($1, $2, $1, 'personas', 'M', 'ES', 2, false, 'digest1')`, slug, datasetID)

	raw := buildCleanWorkbook(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
	}))
	defer server.Close()

	client := xlsx.NewClient(xlsxStatusTestSchema(), server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	cfg := ingestion.SeriesIngestConfig{
		SourceID: sourceID, DatasetID: datasetID, SeriesID: slug,
		COD: server.URL, // an xlsx-url ref IS the request URL itself
		Series: indicators.Series{
			Slug: slug, Unit: "personas", Frequency: indicators.FrequencyMonthly, Decimals: 2,
			Source: sourceID, Licence: "test licence",
		},
	}

	if _, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now); err != nil {
		t.Fatalf("expected a clean workbook to publish successfully, got: %v", err)
	}

	rows, err := tx.Query(ctx, `SELECT period, status, source_status FROM observation WHERE series_id=$1 AND is_current ORDER BY period`, slug)
	if err != nil {
		t.Fatalf("querying published observations: %v", err)
	}
	defer rows.Close()

	type row struct {
		period       string
		status       string
		sourceStatus *string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.period, &r.status, &r.sourceStatus); err != nil {
			t.Fatalf("scanning published observation: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating published observations: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected exactly 2 published observations (the guard did not block), got %d", len(got))
	}
	for _, r := range got {
		if r.status != string(postgres.StatusDefinitive) {
			t.Errorf("period %s: expected status Definitive (%q), got %q", r.period, postgres.StatusDefinitive, r.status)
		}
		if r.sourceStatus != nil {
			t.Errorf("period %s: expected source_status NULL (the workbook publishes no status token), got %q", r.period, *r.sourceStatus)
		}
	}
}
