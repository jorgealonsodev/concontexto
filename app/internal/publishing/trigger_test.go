package publishing_test

// Task 4.5 (RED) / 4.6 (GREEN): publishing.Publish wraps Export with
// dispatch + instant recording (design D-2: "Publish(ctx, deps, asOf)
// (Export + dispatch + latency stamp)"). Task 4.7's own dispatch-failure
// contract ("alerts, never retries, never fails the ingest") is proven
// here at the Publish level: a failing Dispatcher never surfaces as a
// Publish error.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func onePublishedSeriesDeps(t *testing.T) publishing.Deps {
	ps := fakePublishedSeries()
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: "aaaa", RequestURL: "https://ine.es/data",
		}},
	}
	return fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)
}

func TestPublish_ExportsAndDispatchesRecordingBothInstants(t *testing.T) {
	deps := onePublishedSeriesDeps(t)
	var dispatchedGeneratedAt time.Time
	var dispatchedDigest string
	dispatch := func(_ context.Context, generatedAt time.Time, digest string) error {
		dispatchedGeneratedAt = generatedAt
		dispatchedDigest = digest
		return nil
	}

	asOf := time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC)
	result, err := publishing.Publish(context.Background(), deps, dispatch, asOf, t.TempDir(), "", 0)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(result.Artifact.Series) != 1 {
		t.Fatalf("expected the artifact to carry the one published series, got %d", len(result.Artifact.Series))
	}
	if !result.IngestionSuccessAt.Equal(asOf) {
		t.Errorf("expected IngestionSuccessAt to equal asOf, got %v", result.IngestionSuccessAt)
	}
	if result.DispatchedAt == nil {
		t.Fatal("expected DispatchedAt to be recorded on a successful dispatch")
	}
	if dispatchedGeneratedAt.IsZero() || dispatchedDigest == "" {
		t.Errorf("expected the dispatcher to receive a non-zero generatedAt and a non-empty digest, got %v/%q", dispatchedGeneratedAt, dispatchedDigest)
	}
}

func TestPublish_ANilDispatcherSkipsDispatchWithoutError(t *testing.T) {
	deps := onePublishedSeriesDeps(t)
	result, err := publishing.Publish(context.Background(), deps, nil, time.Now(), t.TempDir(), "", 0)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.DispatchedAt != nil {
		t.Errorf("expected no dispatch instant with a nil dispatcher, got %v", result.DispatchedAt)
	}
}

func TestPublish_AnEmptyHistoryDirSkipsRetentionWithoutError(t *testing.T) {
	deps := onePublishedSeriesDeps(t)
	result, err := publishing.Publish(context.Background(), deps, nil, time.Now(), t.TempDir(), "", 0)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.ArchiveErr != nil {
		t.Errorf("expected no archive error when historyDir is empty (not configured), got %v", result.ArchiveErr)
	}
}

func TestPublish_ArchivesARetainedSnapshotWhenHistoryDirIsConfigured(t *testing.T) {
	deps := onePublishedSeriesDeps(t)
	historyDir := t.TempDir()
	asOf := time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC)

	result, err := publishing.Publish(context.Background(), deps, nil, asOf, t.TempDir(), historyDir, 5)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.ArchiveErr != nil {
		t.Fatalf("expected retention to succeed, got %v", result.ArchiveErr)
	}
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatalf("reading historyDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected Publish to archive exactly one snapshot, got %d", len(entries))
	}
}

func TestPublish_ADispatchFailureAlertsAndNeverFailsPublish(t *testing.T) {
	deps := onePublishedSeriesDeps(t)
	spy := &spySink{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil)

	dispatch := func(context.Context, time.Time, string) error {
		return errors.New("github: dispatch returned status 403")
	}

	result, err := publishing.Publish(context.Background(), deps, dispatch, time.Now(), t.TempDir(), "", 0)
	if err != nil {
		t.Fatalf("Publish must never fail the whole cycle on a dispatch error, got: %v", err)
	}
	if result.DispatchedAt != nil {
		t.Errorf("expected no dispatch instant on a dispatch failure, got %v", result.DispatchedAt)
	}
	if len(spy.alerts) != 1 || spy.alerts[0].Kind != alerting.KindDispatchFailed {
		t.Fatalf("expected exactly one KindDispatchFailed alert, got %+v", spy.alerts)
	}
}

type spySink struct {
	alerts []alerting.Alert
}

func (s *spySink) Alert(_ context.Context, a alerting.Alert) error {
	s.alerts = append(s.alerts, a)
	return nil
}

func TestPublish_AFailedExportNeverDispatches(t *testing.T) {
	deps := onePublishedSeriesDeps(t)
	deps.SeriesFreshness = func(context.Context, string, time.Time) (freshness.State, error) {
		return "", errors.New("boom")
	}
	dispatched := false
	dispatch := func(context.Context, time.Time, string) error {
		dispatched = true
		return nil
	}

	_, err := publishing.Publish(context.Background(), deps, dispatch, time.Now(), t.TempDir(), "", 0)
	if err == nil {
		t.Fatal("expected Publish to surface Export's own error")
	}
	if dispatched {
		t.Error("expected a failed export to never reach dispatch")
	}
}

// CRITICAL-28 (RED), link 2. DispatchedAt == nil used to mean two entirely
// different things -- "no dispatcher was configured, so nothing was
// attempted" and "a dispatcher was configured and the attempt failed" --
// and the caller had no way to tell them apart. runIngest consequently
// printed "exported and dispatched" for all three outcomes, including the
// deployed-stack case where nothing was ever dispatched at all.
// DispatchSkipped is the discriminator: with DispatchedAt it names exactly
// one of the three states, and the caller logs which one.
func TestPublish_DispatchStateIsDistinguishableAcrossAllThreeOutcomes(t *testing.T) {
	t.Run("not configured: skipped, never attempted", func(t *testing.T) {
		result, err := publishing.Publish(context.Background(), onePublishedSeriesDeps(t), nil, time.Now(), t.TempDir(), "", 0)
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		if !result.DispatchSkipped {
			t.Error("expected DispatchSkipped with a nil dispatcher: nothing was attempted")
		}
		if result.DispatchedAt != nil {
			t.Errorf("expected no dispatch instant when dispatch was skipped, got %v", result.DispatchedAt)
		}
	})

	t.Run("configured and dispatching: not skipped, instant recorded", func(t *testing.T) {
		dispatch := func(context.Context, time.Time, string) error { return nil }
		result, err := publishing.Publish(context.Background(), onePublishedSeriesDeps(t), dispatch, time.Now(), t.TempDir(), "", 0)
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		if result.DispatchSkipped {
			t.Error("expected DispatchSkipped to be false when a dispatcher was configured and succeeded")
		}
		if result.DispatchedAt == nil {
			t.Error("expected a dispatch instant on a successful dispatch")
		}
	})

	t.Run("configured and failing: not skipped, no instant", func(t *testing.T) {
		alerting.SetDefaultSink(alerting.NoopSink{})
		defer alerting.SetDefaultSink(nil)
		dispatch := func(context.Context, time.Time, string) error {
			return errors.New("github: dispatch returned status 403")
		}
		result, err := publishing.Publish(context.Background(), onePublishedSeriesDeps(t), dispatch, time.Now(), t.TempDir(), "", 0)
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		if result.DispatchSkipped {
			t.Error("expected DispatchSkipped to be FALSE on a dispatch FAILURE: an attempt was made and it failed, which is not the same as never attempting")
		}
		if result.DispatchedAt != nil {
			t.Errorf("expected no dispatch instant on a dispatch failure, got %v", result.DispatchedAt)
		}
	})
}
