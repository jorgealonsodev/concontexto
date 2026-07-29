package postgres_test

// Task 2.10 (RED) / 2.11 (GREEN): provenance flows through the run
// (observation -> ingestion_run.raw_file_hash -> raw_file), and a
// vintage-as-of-date D resolves to the max version among runs started
// at or before D (spec data-model-vintages, "Run-level vintage
// separated from per-period version").

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestResolveProvenance_YieldsNonNullFieldsFromObservationToRawFile(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'TESTCOD001', '2020-01-01', 'digest1')`)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision: %v", err)
	}

	prov, err := postgres.ResolveProvenance(ctx, tx, "tasa-de-paro-epa", "2026-Q1")
	if err != nil {
		t.Fatalf("ResolveProvenance: %v", err)
	}
	if prov.SourceID == "" {
		t.Error("expected a non-empty SourceID")
	}
	if prov.OriginRef != "TESTCOD001" {
		t.Errorf("expected origin ref TESTCOD001, got %q", prov.OriginRef)
	}
	if prov.ExtractedAt.IsZero() {
		t.Error("expected a non-zero ExtractedAt")
	}
	if prov.IngestionRunID != run1 {
		t.Errorf("expected ingestion run id %d, got %d", run1, prov.IngestionRunID)
	}
	if prov.RawFileHash != "hash-run1" {
		t.Errorf("expected raw file hash hash-run1, got %q", prov.RawFileHash)
	}
}

func TestVintageAsOf_ResolvesMaxVersionAmongRunsAtOrBeforeDate(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	after := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	d := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // strictly between before and after

	run1 := seedIngestionRun(t, ctx, tx, "hash-before", before)
	writer := postgres.NewObservationWriter(tx)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusDefinitive, ExtractedAt: before, IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision (before D): %v", err)
	}

	run2 := seedIngestionRun(t, ctx, tx, "hash-after", after)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.9),
		Status: postgres.StatusDefinitive, ExtractedAt: after, IngestionRunID: run2,
	}); err != nil {
		t.Fatalf("WriteRevision (after D): %v", err)
	}

	obs, err := postgres.VintageAsOf(ctx, tx, "tasa-de-paro-epa", "2026-Q1", d)
	if err != nil {
		t.Fatalf("VintageAsOf: %v", err)
	}
	if obs.Version != 1 {
		t.Errorf("expected the as-of-D vintage to resolve to version 1 (the run before D), got version %d", obs.Version)
	}
	if obs.Value == nil || *obs.Value != 11.5 {
		t.Errorf("expected the as-of-D value to be 11.5, got %v", obs.Value)
	}
}
