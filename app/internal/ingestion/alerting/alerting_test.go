package alerting_test

// Task 9.7 (RED)/9.8 (GREEN): pure, offline proof of ValidationFailed and
// SourceDown's exact alert content (spec pipeline-operations,
// "Operational alerts": "Alerts MUST name the source and the affected
// series").
//
// Disclosure: alerting.go's production code and this test file were
// authored together, not test-first -- see this batch's own apply-
// progress note for the full disclosure, matching pipelinelog_test.go's
// and scheduler_test.go's.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
)

type spySink struct {
	alerts []alerting.Alert
	err    error
}

func (s *spySink) Alert(_ context.Context, a alerting.Alert) error {
	s.alerts = append(s.alerts, a)
	return s.err
}

func TestValidationFailed_NamesSourceSeriesAndFailingRules(t *testing.T) {
	spy := &spySink{}
	err := alerting.ValidationFailed(context.Background(), spy, "eurostat", "paro-armonizado-eurostat", []string{"rule3-plausibility", "rule2-continuity"})
	if err != nil {
		t.Fatalf("ValidationFailed: %v", err)
	}
	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly 1 alert, got %d", len(spy.alerts))
	}
	a := spy.alerts[0]
	if a.Kind != alerting.KindValidationFailed {
		t.Errorf("expected KindValidationFailed, got %v", a.Kind)
	}
	if a.Source != "eurostat" || a.Series != "paro-armonizado-eurostat" {
		t.Errorf("expected the alert to name source+series, got source=%q series=%q", a.Source, a.Series)
	}
	if len(a.FailingRules) != 2 || a.FailingRules[0] != "rule3-plausibility" {
		t.Errorf("expected failing rules preserved in order, got %v", a.FailingRules)
	}
}

func TestSourceDown_NamesTheSource(t *testing.T) {
	spy := &spySink{}
	err := alerting.SourceDown(context.Background(), spy, "ine")
	if err != nil {
		t.Fatalf("SourceDown: %v", err)
	}
	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly 1 alert, got %d", len(spy.alerts))
	}
	a := spy.alerts[0]
	if a.Kind != alerting.KindSourceDown {
		t.Errorf("expected KindSourceDown, got %v", a.Kind)
	}
	if a.Source != "ine" {
		t.Errorf("expected the alert to name the source, got %q", a.Source)
	}
	if a.Series != "" {
		t.Errorf("expected no series on a source-level alert, got %q", a.Series)
	}
}

func TestNilSinkFallsBackToDefaultSink(t *testing.T) {
	spy := &spySink{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil) // resets to NoopSink, never leaves a test spy as the process default

	if err := alerting.ValidationFailed(context.Background(), nil, "ine", "tasa-de-paro-epa", []string{"rule1-schema"}); err != nil {
		t.Fatalf("ValidationFailed with a nil sink: %v", err)
	}
	if len(spy.alerts) != 1 {
		t.Fatalf("expected the nil sink to fall back to the default sink, got %d alerts recorded on the spy", len(spy.alerts))
	}
}

func TestSetDefaultSink_NilResetsToNoopNotPanic(t *testing.T) {
	alerting.SetDefaultSink(nil)
	defer alerting.SetDefaultSink(nil)

	if err := alerting.SourceDown(context.Background(), nil, "eurostat"); err != nil {
		t.Fatalf("expected NoopSink to succeed silently, got: %v", err)
	}
}

func TestNoopSink_DiscardsSilently(t *testing.T) {
	if err := (alerting.NoopSink{}).Alert(context.Background(), alerting.Alert{Kind: alerting.KindSourceDown, Source: "ine"}); err != nil {
		t.Fatalf("expected NoopSink.Alert to never fail, got: %v", err)
	}
}

// Task 4.7/4.11 (RED)/(GREEN): DispatchFailed and PublishLatencyBreach are
// slice 4's two new alert kinds (spec pipeline-operations' MODIFIED
// "Operational alerts" requirement).
func TestDispatchFailed_NamesTheCycleAndTheUnderlyingError(t *testing.T) {
	spy := &spySink{}
	underlying := errors.New("github: dispatch returned status 403")
	generatedAt := time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC)
	if err := alerting.DispatchFailed(context.Background(), spy, generatedAt, underlying); err != nil {
		t.Fatalf("DispatchFailed: %v", err)
	}
	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly 1 alert, got %d", len(spy.alerts))
	}
	a := spy.alerts[0]
	if a.Kind != alerting.KindDispatchFailed {
		t.Errorf("expected KindDispatchFailed, got %v", a.Kind)
	}
	if !strings.Contains(a.Message, "2026-07-29T06:00:00Z") || !strings.Contains(a.Message, "403") {
		t.Errorf("expected the message to name the cycle's generated_at and the underlying error, got %q", a.Message)
	}
}

func TestPublishLatencyBreach_NamesSourceSeriesAndElapsed(t *testing.T) {
	spy := &spySink{}
	if err := alerting.PublishLatencyBreach(context.Background(), spy, "ine", "tasa-de-paro-epa", 45*time.Minute); err != nil {
		t.Fatalf("PublishLatencyBreach: %v", err)
	}
	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly 1 alert, got %d", len(spy.alerts))
	}
	a := spy.alerts[0]
	if a.Kind != alerting.KindPublishLatencyBreach {
		t.Errorf("expected KindPublishLatencyBreach, got %v", a.Kind)
	}
	if a.Source != "ine" || a.Series != "tasa-de-paro-epa" {
		t.Errorf("expected the alert to name source+series, got source=%q series=%q", a.Source, a.Series)
	}
	if !strings.Contains(a.Message, "45m0s") {
		t.Errorf("expected the message to name the elapsed duration, got %q", a.Message)
	}
}
