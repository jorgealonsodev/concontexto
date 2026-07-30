package postgres_test

// Task 3.10 (RED/GREEN): ListPublishedSeries (series metadata joined
// with its active source mapping) and ListPublishedObservations (the
// full current vintage joined with its provenance chain), both against
// a real postgres:17-alpine container (see testmain_test.go).

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestListPublishedSeries_ReturnsMetadataJoinedWithTheActiveSourceMapping(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'TESTCOD001', '2020-01-01', 'digest1')`)

	got, err := postgres.ListPublishedSeries(ctx, tx)
	if err != nil {
		t.Fatalf("ListPublishedSeries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one series, got %d: %+v", len(got), got)
	}
	ps := got[0]
	if ps.Slug != "tasa-de-paro-epa" || ps.Name != "Tasa de paro" || ps.Unit != "%" || ps.Frequency != "Q" {
		t.Fatalf("unexpected series identity: %+v", ps)
	}
	if ps.DatasetID != "ine-epa" {
		t.Fatalf("expected dataset id ine-epa, got %q", ps.DatasetID)
	}
	if ps.SourceID != "ine" || ps.SourceName != "INE" || ps.SourceAttribution != "attr" {
		t.Fatalf("unexpected source metadata: %+v", ps)
	}
	if ps.OriginKind != "ine-series-cod" || ps.OriginRef != "TESTCOD001" {
		t.Fatalf("unexpected origin: %+v", ps)
	}
}

// TestListPublishedSeries_ReturnsDistinctLicenceURLWhenReconciled proves
// SourceLicenceURL (migration 0004) round-trips distinctly from
// SourceURL -- the read half of TestReconcileSource_PersistsDistinctLicenceURL's
// write half (dimensions_test.go).
func TestListPublishedSeries_ReturnsDistinctLicenceURLWhenReconciled(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `UPDATE source SET licence_url = 'https://ine.es/condiciones-de-uso' WHERE id = 'ine'`)
	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'TESTCOD001', '2020-01-01', 'digest1')`)

	got, err := postgres.ListPublishedSeries(ctx, tx)
	if err != nil {
		t.Fatalf("ListPublishedSeries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one series, got %d", len(got))
	}
	if got[0].SourceURL != "https://ine.es" {
		t.Errorf("expected the general source url to stay unchanged, got %q", got[0].SourceURL)
	}
	if got[0].SourceLicenceURL != "https://ine.es/condiciones-de-uso" {
		t.Errorf("expected the distinct licence url, got %q", got[0].SourceLicenceURL)
	}
}

// TestListPublishedSeries_LicenceURLIsEmptyWhenNotYetReconciled proves the
// zero-value case (a row seeded before migration 0004 or never
// re-reconciled) reads back as an empty string, not an error -- the
// caller (publishing.sourceLicenceURL) owns the fallback decision, not
// this read function.
func TestListPublishedSeries_LicenceURLIsEmptyWhenNotYetReconciled(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx) // seedSeries never sets licence_url
	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'TESTCOD001', '2020-01-01', 'digest1')`)

	got, err := postgres.ListPublishedSeries(ctx, tx)
	if err != nil {
		t.Fatalf("ListPublishedSeries: %v", err)
	}
	if got[0].SourceLicenceURL != "" {
		t.Errorf("expected an empty SourceLicenceURL when never reconciled, got %q", got[0].SourceLicenceURL)
	}
}

func TestListPublishedSeries_ExcludesARetiredSeries(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'TESTCOD001', '2020-01-01', 'digest1')`)
	mustExec(t, ctx, tx, `UPDATE series SET retired_at = now() WHERE id = 'tasa-de-paro-epa'`)

	got, err := postgres.ListPublishedSeries(ctx, tx)
	if err != nil {
		t.Fatalf("ListPublishedSeries: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected a retired series to be excluded, got %d: %+v", len(got), got)
	}
}

func TestListPublishedSeries_ExcludesASeriesWithNoActiveMapping(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx) // no series_source_mapping row at all

	got, err := postgres.ListPublishedSeries(ctx, tx)
	if err != nil {
		t.Fatalf("ListPublishedSeries: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected a series with no active mapping to be excluded, got %d: %+v", len(got), got)
	}
}

func TestListPublishedObservations_JoinsEveryCurrentObservationWithItsProvenance(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	sourceStatus := "Definitivo"
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusDefinitive, SourceStatus: &sourceStatus,
		ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision: %v", err)
	}

	got, err := postgres.ListPublishedObservations(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("ListPublishedObservations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one current observation, got %d: %+v", len(got), got)
	}
	obs := got[0]
	if obs.Period != "2026-Q1" || obs.Value == nil || *obs.Value != 11.5 {
		t.Fatalf("unexpected observation: %+v", obs)
	}
	if obs.Status != postgres.StatusDefinitive {
		t.Fatalf("expected status D, got %q", obs.Status)
	}
	if obs.SourceStatus == nil || *obs.SourceStatus != "Definitivo" {
		t.Fatalf("expected source_status Definitivo, got %v", obs.SourceStatus)
	}
	if obs.Version != 1 || obs.IngestionRunID != run1 {
		t.Fatalf("expected version 1 / run %d, got version=%d run=%d", run1, obs.Version, obs.IngestionRunID)
	}
	if obs.RawFileSHA256 != "hash-run1" {
		t.Fatalf("expected raw_file_sha256 hash-run1, got %q", obs.RawFileSHA256)
	}
	if obs.RequestURL != "https://ine.es/data" {
		t.Fatalf("expected request URL https://ine.es/data, got %q", obs.RequestURL)
	}
}

func TestListPublishedObservations_OnlyEverReturnsIsCurrentRows(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run1 := seedIngestionRun(t, ctx, tx, "hash-run1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	writer := postgres.NewObservationWriter(tx)
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.5),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), IngestionRunID: run1,
	}); err != nil {
		t.Fatalf("WriteRevision v1: %v", err)
	}
	run2 := seedIngestionRun(t, ctx, tx, "hash-run2", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if _, _, err := writer.WriteRevision(ctx, postgres.ObservationInput{
		SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(11.9),
		Status: postgres.StatusDefinitive, ExtractedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), IngestionRunID: run2,
	}); err != nil {
		t.Fatalf("WriteRevision v2: %v", err)
	}

	got, err := postgres.ListPublishedObservations(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("ListPublishedObservations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one CURRENT row (not both versions), got %d: %+v", len(got), got)
	}
	if got[0].Version != 2 || got[0].Value == nil || *got[0].Value != 11.9 {
		t.Fatalf("expected the current (version 2, value 11.9) row, got %+v", got[0])
	}
}
