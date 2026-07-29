package postgres_test

// Task 3.12 (RED) / 3.13 (GREEN): attribution for a published value must
// be derivable from its provenance chain — series to source to that
// source's configured attribution text (spec
// source-attribution-licensing, "Attribution chains from the original
// source"). The origin series identifier and extraction timestamp
// accompany it, so the same join ResolveProvenance already proves
// (observation -> ingestion_run -> raw_file -> source, joined with the
// active series_source_mapping) also resolves attribution — it only
// needs the source's already-stored attribution_text alongside it.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestResolveAttribution_YieldsSourceTextOriginRefAndExtractedAt(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx) // source.attribution_text = 'attr' (see observation_writer_test.go)
	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'TESTCOD003', '2020-01-01', 'digest1')`)

	run1 := seedIngestionRun(t, ctx, tx, "hash-attr-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	extractedAt := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusDefinitive, ExtractedAt: extractedAt,
		IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision: %v", err)
	}

	attr, err := postgres.ResolveAttribution(ctx, tx, "tasa-de-paro-epa", "2026-Q1")
	if err != nil {
		t.Fatalf("ResolveAttribution: %v", err)
	}
	if attr.AttributionText != "attr" {
		t.Errorf("AttributionText = %q, want the source's configured attribution text %q", attr.AttributionText, "attr")
	}
	if attr.OriginRef != "TESTCOD003" {
		t.Errorf("OriginRef = %q, want TESTCOD003", attr.OriginRef)
	}
	if !attr.ExtractedAt.Equal(extractedAt) {
		t.Errorf("ExtractedAt = %v, want %v", attr.ExtractedAt, extractedAt)
	}
}

func TestResolveAttribution_NoCurrentObservationReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	if _, err := postgres.ResolveAttribution(ctx, tx, "tasa-de-paro-epa", "2099-Q1"); err == nil {
		t.Fatal("expected an error resolving attribution for a period with no observation, got nil")
	}
}
