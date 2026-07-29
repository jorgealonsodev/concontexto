package ingestion_test

// Task 9.5 (RED)/9.6 (GREEN) + 9.7 (RED)/9.8 (GREEN), wiring half: proves
// IngestSeries itself -- not just pipelinelog/alerting's own pure
// builders -- emits a structured log line and (on a validation failure)
// an alert, end to end against a real Postgres and a real INE client.
// Reuses ingest_test.go's own fixtures/helpers (newTx, sixSeries,
// seedDimensions, ineIngestConfig, loadFixture) rather than duplicating
// them.

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// capturingHandler is a minimal slog.Handler recording every record it
// receives, so a test can assert on structured fields without parsing
// text/JSON output.
type capturingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}
func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func (h *capturingHandler) attrValues(t *testing.T, recordIndex int) map[string]any {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if recordIndex >= len(h.records) {
		t.Fatalf("expected at least %d captured log record(s), got %d", recordIndex+1, len(h.records))
	}
	out := map[string]any{}
	h.records[recordIndex].Attrs(func(a slog.Attr) bool {
		out[a.Key] = a.Value.Any()
		return true
	})
	return out
}

// TestIngestSeries_CompletedRunLogsRunIDSourceDatasetSeriesOutcomeVerdictsHashDuration
// is spec's "A run is reconstructible from logs alone" scenario for a
// successful (Publish) run.
func TestIngestSeries_CompletedRunLogsRunIDSourceDatasetSeriesOutcomeVerdictsHashDuration(t *testing.T) {
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

	handler := &capturingHandler{}
	prior := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(prior)

	result, err := ingestion.IngestSeries(ctx, tx, store, client, ineIngestConfig(sc, cod), now)
	if err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected GatePublish, got %v", result.Outcome)
	}

	attrs := handler.attrValues(t, 0)
	if got, ok := attrs["run_id"].(int64); !ok || got != result.RunID {
		t.Errorf("expected run_id=%d, got %v", result.RunID, attrs["run_id"])
	}
	if attrs["source"] != "ine" {
		t.Errorf("expected source=ine, got %v", attrs["source"])
	}
	if attrs["dataset"] != sc.datasetID {
		t.Errorf("expected dataset=%q, got %v", sc.datasetID, attrs["dataset"])
	}
	if attrs["series"] != sc.slug {
		t.Errorf("expected series=%q, got %v", sc.slug, attrs["series"])
	}
	if attrs["outcome"] != string(validation.GatePublish) {
		t.Errorf("expected outcome=%q, got %v", validation.GatePublish, attrs["outcome"])
	}
	hash, _ := attrs["raw_file_hash"].(string)
	if hash == "" {
		t.Error("expected a non-empty raw_file_hash")
	}
	if _, ok := attrs["duration"]; !ok {
		t.Error("expected a duration field")
	}
}

// TestIngestSeries_FailedRunLogsWhichRulesFailedAndAlerts is spec's
// "a failed run additionally records which rules failed" (logging) +
// "A validation failure alerts without publishing" (alerting) scenarios,
// combined -- forcing a genuine rule3-plausibility failure (not the
// synthetic decode-failure finding) via a Plausibility.Max no real
// fixture value can satisfy.
func TestIngestSeries_FailedRunLogsWhichRulesFailedAndAlerts(t *testing.T) {
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

	cfg := ineIngestConfig(sc, cod)
	belowEveryRealValue := -1.0
	cfg.Validation = config.ValidationConfig{
		Plausibility: config.PlausibilityConfig{Max: &belowEveryRealValue},
	}

	handler := &capturingHandler{}
	priorLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(priorLogger)

	spy := &spySinkForIngestion{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil)

	result, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now)
	if err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}
	if result.Outcome != validation.GateBlock {
		t.Fatalf("expected GateBlock (every real value exceeds the configured max), got %v findings=%+v", result.Outcome, result.Findings)
	}

	attrs := handler.attrValues(t, 0)
	if attrs["outcome"] != string(validation.GateBlock) {
		t.Errorf("expected outcome=%q, got %v", validation.GateBlock, attrs["outcome"])
	}
	failedRules, _ := attrs["failed_rules"].([]string)
	if len(failedRules) != 1 || failedRules[0] != "rule3-plausibility" {
		t.Errorf("expected failed_rules=[rule3-plausibility], got %v", attrs["failed_rules"])
	}

	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly 1 alert raised, got %d", len(spy.alerts))
	}
	a := spy.alerts[0]
	if a.Kind != alerting.KindValidationFailed {
		t.Errorf("expected KindValidationFailed, got %v", a.Kind)
	}
	if a.Source != "ine" || a.Series != sc.slug {
		t.Errorf("expected the alert to name source=ine series=%q, got source=%q series=%q", sc.slug, a.Source, a.Series)
	}
	if len(a.FailingRules) != 1 || a.FailingRules[0] != "rule3-plausibility" {
		t.Errorf("expected the alert to name the failing rule, got %v", a.FailingRules)
	}

	// "the previously published datum is still served": this run
	// published nothing (Block), so the prior current vintage (none, in
	// this fresh series) stays exactly as absent as before -- the
	// stronger, general property (a PRIOR publish surviving a LATER
	// block) is already proven at the gate level by
	// postgres.ApplyGate's own Block-branch tests (task 4.15/4.16), not
	// re-proven here.
	prior, err := postgres.ListCurrentObservations(ctx, tx, sc.slug)
	if err != nil {
		t.Fatalf("ListCurrentObservations: %v", err)
	}
	if len(prior) != 0 {
		t.Errorf("expected zero current observations after a Block outcome, got %d", len(prior))
	}
}

type spySinkForIngestion struct{ alerts []alerting.Alert }

func (s *spySinkForIngestion) Alert(_ context.Context, a alerting.Alert) error {
	s.alerts = append(s.alerts, a)
	return nil
}
