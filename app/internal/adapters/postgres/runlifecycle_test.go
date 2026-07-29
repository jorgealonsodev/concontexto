package postgres_test

// Task 5b.6 (RED): the ingestion orchestrator (package ingestion) needs
// two pieces the postgres adapter did not yet expose:
//
//   - CreateIngestionRun: the initial ingestion_run row insert (design.md
//     "[Tx1] download_attempt + raw_file + ingestion_run(pending)" --
//     nothing before this batch ever created that row; ApplyGate's
//     recordRunOutcome only ever UPDATEs an already-existing one).
//   - ListCurrentObservations: the FULL current vintage for a series
//     (design.md "Prior []indicators.Observation // current vintage
//     before this run"), not just one (series, period) the way
//     currentObservation already resolves.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func TestCreateIngestionRun_InsertsAPendingRowThatApplyGateCanLaterUpdate(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ('hash-pending', 'ine', 'https://ine.es/data', now(), '/app_data/hash-pending', 100)`)

	startedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	runID, err := postgres.CreateIngestionRun(ctx, tx, "ine-epa", "tasa-de-paro-epa", startedAt, "hash-pending")
	if err != nil {
		t.Fatalf("CreateIngestionRun: %v", err)
	}
	if runID == 0 {
		t.Fatal("expected a non-zero ingestion_run id")
	}

	var outcome, rawFileHash string
	var seriesID string
	row := tx.QueryRow(ctx, `SELECT outcome, raw_file_hash, series_id FROM ingestion_run WHERE id=$1`, runID)
	if err := row.Scan(&outcome, &rawFileHash, &seriesID); err != nil {
		t.Fatalf("reading the created run: %v", err)
	}
	if outcome != string(postgres.RunOutcomePending) {
		t.Errorf("expected outcome %q, got %q", postgres.RunOutcomePending, outcome)
	}
	if rawFileHash != "hash-pending" {
		t.Errorf("expected raw_file_hash hash-pending, got %q", rawFileHash)
	}
	if seriesID != "tasa-de-paro-epa" {
		t.Errorf("expected series_id tasa-de-paro-epa, got %q", seriesID)
	}

	// ApplyGate's recordRunOutcome must be able to move this exact row to
	// a terminal outcome -- this is the whole reason CreateIngestionRun
	// exists (the pending row IS the input recordRunOutcome expects).
	result, err := postgres.ApplyGate(ctx, tx, runID, nil, nil)
	if err != nil {
		t.Fatalf("ApplyGate on the pending run: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected an all-pass (empty findings) run to publish, got %v", result.Outcome)
	}
}

func TestListCurrentObservations_ReturnsTheFullCurrentVintage(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	for _, p := range []struct {
		period string
		value  float64
	}{
		{"2025-Q4", 9.93},
		{"2026-Q1", 10.83},
	} {
		if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
			SeriesID: "tasa-de-paro-epa", Period: p.period, Value: ptr(p.value),
			Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			IngestionRunID: run1,
		}); err != nil {
			t.Fatalf("seeding %s: %v", p.period, err)
		}
	}

	// Supersede 2025-Q4 with a second version -- ListCurrentObservations
	// must return only the CURRENT (latest) value, never a superseded one.
	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2025-Q4", Value: ptr(9.94),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run2,
	}); err != nil {
		t.Fatalf("revising 2025-Q4: %v", err)
	}

	prior, err := postgres.ListCurrentObservations(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("ListCurrentObservations: %v", err)
	}
	if len(prior) != 2 {
		t.Fatalf("expected exactly 2 current observations, got %d: %+v", len(prior), prior)
	}
	byPeriod := map[string]float64{}
	for _, o := range prior {
		if o.Value == nil {
			t.Fatalf("expected every current observation to carry a value, got nil for %v", o.Period)
		}
		byPeriod[o.Period.String()] = *o.Value
	}
	if byPeriod["2025-Q4"] != 9.94 {
		t.Errorf("expected the CURRENT (revised) value 9.94 for 2025-Q4, got %v", byPeriod["2025-Q4"])
	}
	if byPeriod["2026-Q1"] != 10.83 {
		t.Errorf("expected 10.83 for 2026-Q1, got %v", byPeriod["2026-Q1"])
	}
}
