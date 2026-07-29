package postgres_test

// Task 5a.7 (RED, DB half) / 5a.8 (GREEN): resolves the one fact
// freshness.Resolve needs — a source's last successful
// download_attempt — and propagates that source-level verdict to every
// series under it (spec raw-file-archive, "Freshness state is tracked
// per source"; "that state MUST propagate to every series belonging to
// that source's datasets").

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
)

func seedDataset(t *testing.T, ctx context.Context, tx pgx.Tx, sourceID, datasetID string) {
	t.Helper()
	seedSource(t, ctx, tx, sourceID)
	mustExec(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ($1, $2, $2, 'digest1')`, datasetID, sourceID)
}

func recordAttempt(t *testing.T, ctx context.Context, tx pgx.Tx, sourceID string, attemptedAt time.Time, outcome postgres.DownloadOutcome) {
	t.Helper()
	if _, err := postgres.RecordDownloadAttempt(ctx, tx, postgres.DownloadAttempt{
		SourceID: sourceID, URL: "https://example.test/data", AttemptedAt: attemptedAt, Outcome: outcome,
	}); err != nil {
		t.Fatalf("RecordDownloadAttempt: %v", err)
	}
}

func TestSourceFreshness_TwentyFourHoursWithoutSuccessMarksTheSourceStale(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "source-a")

	asOf := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	recordAttempt(t, ctx, tx, "source-a", asOf.Add(-25*time.Hour), postgres.OutcomeNewFile)

	state, err := postgres.SourceFreshness(ctx, tx, "source-a", asOf)
	if err != nil {
		t.Fatalf("SourceFreshness: %v", err)
	}
	if state != freshness.StateFailed {
		t.Fatalf("expected StateFailed, got %v", state)
	}
}

func TestSourceFreshness_WithinTheWindowStaysFresh(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "source-a")

	asOf := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	recordAttempt(t, ctx, tx, "source-a", asOf.Add(-3*time.Hour), postgres.OutcomeUnchanged)

	state, err := postgres.SourceFreshness(ctx, tx, "source-a", asOf)
	if err != nil {
		t.Fatalf("SourceFreshness: %v", err)
	}
	if state != freshness.StateFresh {
		t.Fatalf("expected StateFresh, got %v", state)
	}
}

func TestSourceFreshness_OneFailingSourceDoesNotAffectAnother(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "source-a")
	seedSource(t, ctx, tx, "source-b")

	asOf := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	recordAttempt(t, ctx, tx, "source-a", asOf.Add(-25*time.Hour), postgres.OutcomeRetryableTransport) // stale/failing
	recordAttempt(t, ctx, tx, "source-b", asOf.Add(-1*time.Hour), postgres.OutcomeNewFile)             // healthy

	stateA, err := postgres.SourceFreshness(ctx, tx, "source-a", asOf)
	if err != nil {
		t.Fatalf("SourceFreshness A: %v", err)
	}
	stateB, err := postgres.SourceFreshness(ctx, tx, "source-b", asOf)
	if err != nil {
		t.Fatalf("SourceFreshness B: %v", err)
	}
	if stateA != freshness.StateFailed {
		t.Fatalf("expected source A failed, got %v", stateA)
	}
	if stateB != freshness.StateFresh {
		t.Fatalf("expected source B fresh despite A being stale, got %v", stateB)
	}
}

func TestSeriesFreshness_PropagatesTheSourceStateToEveryUnderlyingSeries(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedDataset(t, ctx, tx, "source-a", "dataset-a")
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('series-1', 'dataset-a', 'Series 1', '%', 'Q', 'ES', 2, false, 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('series-2', 'dataset-a', 'Series 2', '%', 'Q', 'ES', 2, false, 'digest1')`)

	asOf := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	recordAttempt(t, ctx, tx, "source-a", asOf.Add(-25*time.Hour), postgres.OutcomeRetryableTransport)

	for _, seriesID := range []string{"series-1", "series-2"} {
		state, err := postgres.SeriesFreshness(ctx, tx, seriesID, asOf)
		if err != nil {
			t.Fatalf("SeriesFreshness(%s): %v", seriesID, err)
		}
		if state != freshness.StateFailed {
			t.Fatalf("expected series %s to resolve to amber (StateFailed) via its stale source, got %v", seriesID, state)
		}
	}
}

func TestSeriesFreshness_StaysFreshWhenTheUnderlyingSourceIsHealthy(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedDataset(t, ctx, tx, "source-a", "dataset-a")
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('series-1', 'dataset-a', 'Series 1', '%', 'Q', 'ES', 2, false, 'digest1')`)

	asOf := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	recordAttempt(t, ctx, tx, "source-a", asOf.Add(-2*time.Hour), postgres.OutcomeNewFile)

	state, err := postgres.SeriesFreshness(ctx, tx, "series-1", asOf)
	if err != nil {
		t.Fatalf("SeriesFreshness: %v", err)
	}
	if state != freshness.StateFresh {
		t.Fatalf("expected series-1 fresh, got %v", state)
	}
}
