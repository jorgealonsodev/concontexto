package ingestion_test

// Task 2a.7/2a.8 (RED/GREEN): IngestSeries must stop hardcoding
// StatusDefinitive and instead persist the status ine.Client.Decode
// already classified from T3_TipoDato, fail-closed on an unrecognised or
// missing token (spec source-ingestion-ine, "The source's data-type
// token is carried through to the domain" / "An unknown data-type token
// fails closed"). The classification itself lives in ine.Client.Decode
// (see adapters/ine/tipodato_test.go); this file proves the
// END-TO-END wiring through the real pipeline: what gets published, and
// what gets written when a token is rejected.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func serveFixture(fixture []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
}

// TestIngestSeries_PersistsTheRealDefinitivoAndProvisionalStatusPerObservation
// proves the ordinary path, not just the error path: "Definitivo" ->
// D/"Definitivo" and "Provisional" -> P/"Provisional" round-trip through
// the whole pipeline into postgres.Observation, replacing the hardcoded
// StatusDefinitive every published observation used to carry regardless
// of what INE actually reported.
func TestIngestSeries_PersistsTheRealDefinitivoAndProvisionalStatusPerObservation(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sc := sixSeriesCase{slug: "test-status-mapping", datasetID: "test-status-dataset", unit: "índice", frequency: indicators.FrequencyMonthly, decimals: 1}
	cod := "TESTSTATUS001"
	seedDimensions(t, ctx, tx, sc, cod)

	fixture := []byte(`{"COD":"` + cod + `", "Nombre":"test", "T3_Unidad":"indice", "T3_Escala":" ", "Data":[` +
		`{"Fecha":"2026-04-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M04", "Anyo":2026, "Valor":10}` +
		`,{"Fecha":"2026-05-01T00:00:00.000+02:00", "T3_TipoDato":"Provisional", "T3_Periodo":"M05", "Anyo":2026, "Valor":11}` +
		`]}`)

	server := serveFixture(fixture)
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

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
		t.Fatalf("expected GatePublish, got outcome=%v findings=%+v", result.Outcome, result.Findings)
	}
	if len(result.Published) != 2 {
		t.Fatalf("expected 2 published observations, got %d", len(result.Published))
	}

	byPeriod := map[string]postgres.Observation{}
	for _, obs := range result.Published {
		byPeriod[obs.Period] = obs
	}

	definitivo := byPeriod["2026-04"]
	if definitivo.Status != postgres.StatusDefinitive {
		t.Errorf("expected 2026-04 status D, got %s", definitivo.Status)
	}
	if definitivo.SourceStatus == nil || *definitivo.SourceStatus != "Definitivo" {
		t.Errorf("expected 2026-04 source_status %q, got %v", "Definitivo", definitivo.SourceStatus)
	}

	provisional := byPeriod["2026-05"]
	if provisional.Status != postgres.StatusProvisional {
		t.Errorf("expected 2026-05 status P, got %s", provisional.Status)
	}
	if provisional.SourceStatus == nil || *provisional.SourceStatus != "Provisional" {
		t.Errorf("expected 2026-05 source_status %q, got %v", "Provisional", provisional.SourceStatus)
	}
}

// TestIngestSeries_UnrecognisedTipoDatoFailsClosedAndWritesNoObservation
// proves the spec's own scenario end-to-end: "GIVEN a stubbed response
// carrying an unrecognised T3_TipoDato value WHEN ingestion runs THEN it
// fails with sourceerr.SchemaDrift naming the series and the
// unrecognised token AND nothing is written." The raw file and
// ingestion_run DO persist (same ordering guarantee every other
// decode-failure test in this package already proves) -- only the
// observation table stays empty.
func TestIngestSeries_UnrecognisedTipoDatoFailsClosedAndWritesNoObservation(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sc := sixSeriesCase{slug: "test-status-unrecognised", datasetID: "test-status-dataset", unit: "índice", frequency: indicators.FrequencyMonthly, decimals: 1}
	cod := "TESTSTATUS002"
	seedDimensions(t, ctx, tx, sc, cod)

	fixture := []byte(`{"COD":"` + cod + `", "Nombre":"test", "T3_Unidad":"indice", "T3_Escala":" ", "Data":[` +
		`{"Fecha":"2026-04-01T00:00:00.000+02:00", "T3_TipoDato":"Estimado", "T3_Periodo":"M04", "Anyo":2026, "Valor":10}` +
		`]}`)

	server := serveFixture(fixture)
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	cfg := ingestion.SeriesIngestConfig{
		SourceID: "ine", DatasetID: sc.datasetID, SeriesID: sc.slug, COD: cod,
		Series:     indicators.Series{Slug: sc.slug, Unit: sc.unit, Frequency: sc.frequency, Decimals: sc.decimals, Source: "ine", Licence: "test"},
		Validation: config.ValidationConfig{},
	}

	_, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now)
	if err == nil {
		t.Fatal("expected an unrecognised T3_TipoDato token to fail IngestSeries")
	}
	got := err.Error()
	if !strings.Contains(got, cod) {
		t.Errorf("expected the error to name the series %q, got: %v", cod, got)
	}
	if !strings.Contains(got, "Estimado") {
		t.Errorf("expected the error to name the unrecognised token %q, got: %v", "Estimado", got)
	}

	var obsCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM observation`).Scan(&obsCount); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if obsCount != 0 {
		t.Errorf("expected zero observation rows on a rejected status token, got %d", obsCount)
	}

	var runCount, rawFileCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM ingestion_run`).Scan(&runCount); err != nil {
		t.Fatalf("counting ingestion_run rows: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM raw_file`).Scan(&rawFileCount); err != nil {
		t.Fatalf("counting raw_file rows: %v", err)
	}
	if runCount != 1 || rawFileCount != 1 {
		t.Errorf("expected the run/raw_file to still be archived (same ordering guarantee as any other decode failure), got runs=%d raw_files=%d", runCount, rawFileCount)
	}
}

// TestIngestSeries_MissingTipoDatoFailsClosedRatherThanDefaultingToDefinitive
// is the spec's other unknown-token scenario: a tip=A response whose
// observation OMITS T3_TipoDato entirely must fail exactly like an
// unrecognised value, never silently default to definitive.
func TestIngestSeries_MissingTipoDatoFailsClosedRatherThanDefaultingToDefinitive(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sc := sixSeriesCase{slug: "test-status-missing", datasetID: "test-status-dataset", unit: "índice", frequency: indicators.FrequencyMonthly, decimals: 1}
	cod := "TESTSTATUS003"
	seedDimensions(t, ctx, tx, sc, cod)

	fixture := []byte(`{"COD":"` + cod + `", "Nombre":"test", "T3_Unidad":"indice", "T3_Escala":" ", "Data":[` +
		`{"Fecha":"2026-04-01T00:00:00.000+02:00", "T3_Periodo":"M04", "Anyo":2026, "Valor":10}` +
		`]}`)

	server := serveFixture(fixture)
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	cfg := ingestion.SeriesIngestConfig{
		SourceID: "ine", DatasetID: sc.datasetID, SeriesID: sc.slug, COD: cod,
		Series:     indicators.Series{Slug: sc.slug, Unit: sc.unit, Frequency: sc.frequency, Decimals: sc.decimals, Source: "ine", Licence: "test"},
		Validation: config.ValidationConfig{},
	}

	_, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now)
	if err == nil {
		t.Fatal("expected a missing T3_TipoDato token to fail IngestSeries rather than defaulting to definitive")
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SchemaDrift {
		t.Errorf("expected class %s, got %s", sourceerr.SchemaDrift, classified.Class)
	}

	var obsCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM observation`).Scan(&obsCount); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if obsCount != 0 {
		t.Errorf("expected zero observation rows on a missing status token, got %d", obsCount)
	}
}
