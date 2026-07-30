package ingestion_test

// Task 2b.6 (RED/GREEN): IngestSeries logs and alerts when a decoded
// Eurostat break/definition-differs flag (indicators.SourceResult.
// BreakSignals, envelope.go) has no active series_break covering its
// period yet -- and it never publishes-blocks the run, and never writes
// series_break itself (design D-3: "It never writes series_break --
// rupturas.yaml remains the only writer, editorial follow-up"). This
// file proves both branches end-to-end through the real pipeline: an
// uncovered signal alerts, and a covered one (an already-reconciled
// series_break at the exact same period) raises nothing.
//
// No real recorded Eurostat response verified live in this project
// carries a "status" map (see adapters/eurostat/envelope_test.go's own
// disclosure) -- withInjectedEurostatStatus below injects one into the
// real, live-verified HICP fixture at test time, exactly like that
// file's own withInjectedStatus, duplicated here rather than imported
// (Go test helpers do not cross package boundaries).

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// readEurostatFixture mirrors ingest_eurostat_test.go's own inline
// fixture-path construction, isolated into a helper for this file's two
// tests.
func readEurostatFixture(t *testing.T, ref string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "adapters", "eurostat", "testdata", ref+".json"))
	if err != nil {
		t.Fatalf("reading fixture for %s: %v", ref, err)
	}
	return body
}

// posForTimeLabel independently recomputes raw's own linear JSON-stat
// position for the observation carrying time label label -- the same
// generic Horner formula envelope.go's Decode uses, duplicated here (not
// imported) for the same reason adapters/eurostat/envelope_test.go's own
// posForLabel is not imported either.
func posForTimeLabel(t *testing.T, raw []byte, label string) int {
	t.Helper()
	var wire struct {
		ID        []string `json:"id"`
		Size      []int    `json:"size"`
		Dimension map[string]struct {
			Category struct {
				Index map[string]int `json:"index"`
			} `json:"category"`
		} `json:"dimension"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("posForTimeLabel: parsing fixture: %v", err)
	}
	sizeByDim := make(map[string]int, len(wire.ID))
	for i, dim := range wire.ID {
		if i < len(wire.Size) {
			sizeByDim[dim] = wire.Size[i]
		}
	}
	timeIdx, ok := wire.Dimension["time"].Category.Index[label]
	if !ok {
		t.Fatalf("posForTimeLabel: fixture carries no time label %q", label)
	}
	pos := 0
	for _, dim := range wire.ID {
		idx := timeIdx
		if dim != "time" {
			cat := wire.Dimension[dim].Category.Index
			for _, i := range cat {
				idx = i
			}
		}
		pos = pos*sizeByDim[dim] + idx
	}
	return pos
}

func withInjectedEurostatStatus(t *testing.T, raw []byte, entries map[string]string) []byte {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("withInjectedEurostatStatus: parsing fixture: %v", err)
	}
	statusBytes, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("withInjectedEurostatStatus: marshalling injected status map: %v", err)
	}
	doc["status"] = statusBytes
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("withInjectedEurostatStatus: re-marshalling fixture: %v", err)
	}
	return out
}

// spyAlertSink records every alert raised, standing in for a real
// transport exactly like alerting_test.go's own (unexported, package-
// private) spySink -- duplicated here for the same cross-package reason.
type spyAlertSink struct {
	alerts []alerting.Alert
}

func (s *spyAlertSink) Alert(_ context.Context, a alerting.Alert) error {
	s.alerts = append(s.alerts, a)
	return nil
}

// loadHICPSeries returns the real embedded HICP (prc_hicp_minr) Eurostat
// series config plus its source ref and licence -- the same real config
// ingest_eurostat_test.go's own tests load, isolated to just the one
// series this file needs.
func loadHICPSeries(t *testing.T) (config.SeriesConfig, config.SourceRef, config.SourceConfig) {
	t.Helper()
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
	for _, s := range cfg.Series {
		if s.Source == "eurostat" && s.Dataset == "eurostat-hicp" {
			return s, eurostatSourceRef(t, s), eurostatSource
		}
	}
	t.Fatal("expected the real embedded config to declare the HICP (prc_hicp_minr) series")
	return config.SeriesConfig{}, config.SourceRef{}, config.SourceConfig{}
}

func TestIngestSeries_UncoveredEurostatBreakSignalAlertsWithoutBlockingOrWritingSeriesBreak(t *testing.T) {
	prevSink := alerting.DefaultSink()
	spy := &spyAlertSink{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(prevSink)

	sc, ref, eurostatSource := loadHICPSeries(t)

	rawFixture := readEurostatFixture(t, ref.Ref)
	pos := posForTimeLabel(t, rawFixture, "2026-06")
	raw := withInjectedEurostatStatus(t, rawFixture, map[string]string{strconv.Itoa(pos): "b"})

	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedEurostatDimensions(t, ctx, tx, sc, ref, eurostatSource.Licence.Name)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, ref.Filters, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	ingestCfg := ingestion.SeriesIngestConfig{
		SourceID: "eurostat", DatasetID: sc.Dataset, SeriesID: sc.Slug, COD: ref.Ref,
		Series: indicators.Series{
			Slug: sc.Slug, Unit: sc.Unit, Frequency: indicators.Frequency(sc.Frequency), Decimals: sc.Decimals,
			Source: "eurostat", Licence: eurostatSource.Licence.Name,
		},
		Validation: sc.Validation, Schema: sc.Schema,
	}

	result, err := ingestion.IngestSeries(ctx, tx, store, client, ingestCfg, now)
	if err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected the break-flagged run to still publish (b is metadata, not a block), got outcome=%v findings=%+v", result.Outcome, result.Findings)
	}

	var flaggedStatus string
	for _, obs := range result.Published {
		if obs.Period == "2026-06" {
			flaggedStatus = string(obs.Status)
		}
	}
	if flaggedStatus != string(postgres.StatusDefinitive) {
		t.Errorf("expected the b-flagged observation to publish as Definitive, got %q", flaggedStatus)
	}

	var uncovered []alerting.Alert
	for _, a := range spy.alerts {
		if a.Kind == alerting.KindBreakSignalUncovered {
			uncovered = append(uncovered, a)
		}
	}
	if len(uncovered) != 1 {
		t.Fatalf("expected exactly 1 uncovered-break-signal alert, got %d: %+v", len(uncovered), spy.alerts)
	}
	if uncovered[0].Source != "eurostat" || uncovered[0].Series != sc.Slug {
		t.Errorf("expected the alert to name source=eurostat series=%s, got %+v", sc.Slug, uncovered[0])
	}

	var seriesBreakCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM series_break`).Scan(&seriesBreakCount); err != nil {
		t.Fatalf("counting series_break rows: %v", err)
	}
	if seriesBreakCount != 0 {
		t.Errorf("expected IngestSeries to never write series_break itself, got %d rows", seriesBreakCount)
	}
}

func TestIngestSeries_BreakSignalCoveredByAnExistingSeriesBreakRaisesNoAlert(t *testing.T) {
	prevSink := alerting.DefaultSink()
	spy := &spyAlertSink{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(prevSink)

	sc, ref, eurostatSource := loadHICPSeries(t)

	rawFixture := readEurostatFixture(t, ref.Ref)
	pos := posForTimeLabel(t, rawFixture, "2026-06")
	raw := withInjectedEurostatStatus(t, rawFixture, map[string]string{strconv.Itoa(pos): "b"})

	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedEurostatDimensions(t, ctx, tx, sc, ref, eurostatSource.Licence.Name)

	if _, err := postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{{
		BreakKey: "test-hicp-break-covers-2026-06", ScopeKind: "series", ScopeRef: sc.Slug,
		Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Kind: "methodology",
		NoteMD: "already-reconciled break covering the flagged period", ConfigDigest: "digest1",
	}}); err != nil {
		t.Fatalf("ReconcileBreaks: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, ref.Filters, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	ingestCfg := ingestion.SeriesIngestConfig{
		SourceID: "eurostat", DatasetID: sc.Dataset, SeriesID: sc.Slug, COD: ref.Ref,
		Series: indicators.Series{
			Slug: sc.Slug, Unit: sc.Unit, Frequency: indicators.Frequency(sc.Frequency), Decimals: sc.Decimals,
			Source: "eurostat", Licence: eurostatSource.Licence.Name,
		},
		Validation: sc.Validation, Schema: sc.Schema,
	}

	result, err := ingestion.IngestSeries(ctx, tx, store, client, ingestCfg, now)
	if err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected publish, got outcome=%v findings=%+v", result.Outcome, result.Findings)
	}

	for _, a := range spy.alerts {
		if a.Kind == alerting.KindBreakSignalUncovered {
			t.Errorf("expected no uncovered-break-signal alert once a covering series_break exists, got %+v", a)
		}
	}
}
