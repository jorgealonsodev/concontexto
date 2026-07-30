package postgres_test

// Remediation B (verify-report CRITICAL-4), schema half: migration
// 0005_series_discontinued plus the reconcile and read paths that carry
// config.SeriesConfig.Discontinued from YAML to the export artifact.
//
// indicator-page spec, "The three page states of PRD §6.1.3", scenario
// "A discontinued series shows a permanent banner". The banner's two
// facts -- WHEN the source stopped publishing and WHERE the reader
// continues -- are editorial configuration with no observable in-band
// signal from any source in this project, so they follow the same
// config -> reconcile -> database -> export path the break and event
// registries already take. publishing.Export reads exclusively through
// postgres ports, so anything it must render has to reach a column
// first.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// discontinuedTestConfig builds a two-series config: the subject, which
// the caller may mark discontinued, and one live successor candidate.
func discontinuedTestConfig(d *config.DiscontinuedConfig) *config.Config {
	validFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &config.Config{
		Sources: map[string]config.SourceConfig{
			"src-disc": {ID: "src-disc", Name: "Test Source", URL: "https://example.test", AccessType: "api-json",
				Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
		},
		Series: []config.SeriesConfig{
			{Slug: "series-disc-subject", Name: "Subject", Source: "src-disc", Dataset: "ds-disc",
				Unit: "index", Frequency: "Q", Decimals: 1, Geo: "ES", Discontinued: d,
				SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "DISC001", ValidFrom: validFrom}}},
			{Slug: "series-disc-successor", Name: "Successor", Source: "src-disc", Dataset: "ds-disc",
				Unit: "index", Frequency: "Q", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "DISC002", ValidFrom: validFrom}}},
		},
	}
}

// A series row written before migration 0005 (or by a deployment whose
// config marks nothing discontinued) must survive it untouched, with
// both new columns NULL -- the "additive nullable, no bespoke backfill"
// convention migrations 0003 and 0004 already established.
func TestMigrationUp_SeriesDiscontinuedColumnsAreAdditiveNullableAndPreserveExistingRows(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx) // seeds `tasa-de-paro-epa` with no discontinuation

	var name string
	var since *time.Time
	var successor *string
	row := tx.QueryRow(ctx, `SELECT name, discontinued_since, discontinued_successor_slug
		FROM series WHERE id = 'tasa-de-paro-epa'`)
	if err := row.Scan(&name, &since, &successor); err != nil {
		t.Fatalf("reading a series row after migration 0005: %v", err)
	}
	if name != "Tasa de paro" {
		t.Errorf("expected the pre-existing series identity to survive unchanged, got name=%q", name)
	}
	if since != nil {
		t.Errorf("expected discontinued_since to be NULL on a live series, got %v", *since)
	}
	if successor != nil {
		t.Errorf("expected discontinued_successor_slug to be NULL on a live series, got %q", *successor)
	}
}

// Every migration is reversible (spec data-model-vintages, "Every
// migration is reversible"). Down must drop both columns for real and
// lose no series.
func TestMigrationDown_SeriesDiscontinuedColumnsDropWithoutLosingAnySeries(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `UPDATE series SET discontinued_since = DATE '2026-03-31' WHERE id = 'tasa-de-paro-epa'`)

	// Down rolls back exactly one step (runner.go's own contract); 0005
	// is head, so a single call reverts precisely this migration.
	if err := runner.Down(ctx); err != nil {
		t.Fatalf("Down (0005): %v", err)
	}

	for _, column := range []string{"discontinued_since", "discontinued_successor_slug"} {
		var exists bool
		row := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='public' AND table_name='series' AND column_name=$1)`, column)
		if err := row.Scan(&exists); err != nil {
			t.Fatalf("checking series.%s existence: %v", column, err)
		}
		if exists {
			t.Errorf("expected column series.%s to be dropped by Down, it still exists", column)
		}
	}

	var name, unit string
	if err := tx.QueryRow(ctx, `SELECT name, unit FROM series WHERE id='tasa-de-paro-epa'`).Scan(&name, &unit); err != nil {
		t.Fatalf("expected the series row to survive Down, got: %v", err)
	}
	if name != "Tasa de paro" || unit != "%" {
		t.Errorf("expected name/unit to survive Down, got name=%q unit=%q", name, unit)
	}
}

func TestReconcileSeries_PersistsTheDiscontinuationAndClearsItWhenTheSeriesReturns(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	cfg := discontinuedTestConfig(&config.DiscontinuedConfig{Since: "2026-03-31", Successor: "series-disc-successor"})
	if err := postgres.ReconcileDimensions(ctx, tx, cfg, now); err != nil {
		t.Fatalf("ReconcileDimensions: %v", err)
	}

	var since *time.Time
	var successor *string
	row := tx.QueryRow(ctx, `SELECT discontinued_since, discontinued_successor_slug
		FROM series WHERE id='series-disc-subject'`)
	if err := row.Scan(&since, &successor); err != nil {
		t.Fatalf("reading the discontinued series: %v", err)
	}
	if since == nil || since.UTC().Format("2006-01-02") != "2026-03-31" {
		t.Fatalf("expected discontinued_since 2026-03-31, got %v", since)
	}
	if successor == nil || *successor != "series-disc-successor" {
		t.Fatalf("expected discontinued_successor_slug series-disc-successor, got %v", successor)
	}

	// A source that resumes publication, or an editorial correction to a
	// discontinuation entered in error, must be reversible from config
	// alone. An upsert that only ever WROTE the two columns would leave
	// a permanent banner on a live series with no way to remove it short
	// of hand-editing the database.
	if err := postgres.ReconcileDimensions(ctx, tx, discontinuedTestConfig(nil), now); err != nil {
		t.Fatalf("ReconcileDimensions (un-discontinued): %v", err)
	}
	row = tx.QueryRow(ctx, `SELECT discontinued_since, discontinued_successor_slug
		FROM series WHERE id='series-disc-subject'`)
	if err := row.Scan(&since, &successor); err != nil {
		t.Fatalf("re-reading the series after it was un-discontinued: %v", err)
	}
	if since != nil {
		t.Errorf("expected discontinued_since to be cleared to NULL, got %v", *since)
	}
	if successor != nil {
		t.Errorf("expected discontinued_successor_slug to be cleared to NULL, got %q", *successor)
	}
}

// A discontinued series with no configured successor persists the date
// and a NULL successor -- never an empty string, which would be
// indistinguishable from "a successor exists and its slug is blank"
// (the same nullableString discipline reconcileSource applies to
// licence_url).
func TestReconcileSeries_PersistsADiscontinuationWithNoSuccessorAsNull(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	cfg := discontinuedTestConfig(&config.DiscontinuedConfig{Since: "2026-03-31"})
	if err := postgres.ReconcileDimensions(ctx, tx, cfg, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ReconcileDimensions: %v", err)
	}

	var since *time.Time
	var successor *string
	row := tx.QueryRow(ctx, `SELECT discontinued_since, discontinued_successor_slug
		FROM series WHERE id='series-disc-subject'`)
	if err := row.Scan(&since, &successor); err != nil {
		t.Fatalf("reading the discontinued series: %v", err)
	}
	if since == nil {
		t.Fatal("expected discontinued_since to be persisted")
	}
	if successor != nil {
		t.Errorf("expected a NULL successor when none is configured, got %q", *successor)
	}
}

// series.config_digest is what makes a config edit distinguishable from
// a no-op re-reconcile (seriesIdentityDigest's own doc comment). If the
// digest ignored the discontinuation, a series being retired by its
// source would leave the row's digest byte-identical to its live state
// -- an edit that looks, to every consumer of that column, like nothing
// happened.
func TestReconcileSeries_TheIdentityDigestCoversTheDiscontinuation(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	if err := postgres.ReconcileDimensions(ctx, tx, discontinuedTestConfig(nil), now); err != nil {
		t.Fatalf("ReconcileDimensions (live): %v", err)
	}
	var liveDigest string
	if err := tx.QueryRow(ctx, `SELECT config_digest FROM series WHERE id='series-disc-subject'`).Scan(&liveDigest); err != nil {
		t.Fatalf("reading the live digest: %v", err)
	}

	cfg := discontinuedTestConfig(&config.DiscontinuedConfig{Since: "2026-03-31", Successor: "series-disc-successor"})
	if err := postgres.ReconcileDimensions(ctx, tx, cfg, now); err != nil {
		t.Fatalf("ReconcileDimensions (discontinued): %v", err)
	}
	var discontinuedDigest string
	if err := tx.QueryRow(ctx, `SELECT config_digest FROM series WHERE id='series-disc-subject'`).Scan(&discontinuedDigest); err != nil {
		t.Fatalf("reading the discontinued digest: %v", err)
	}
	if discontinuedDigest == liveDigest {
		t.Fatalf("expected the identity digest to change when a series is discontinued, both are %q", liveDigest)
	}

	// The successor is part of the row too: swapping only the successor
	// must also move the digest.
	cfg = discontinuedTestConfig(&config.DiscontinuedConfig{Since: "2026-03-31"})
	if err := postgres.ReconcileDimensions(ctx, tx, cfg, now); err != nil {
		t.Fatalf("ReconcileDimensions (successor removed): %v", err)
	}
	var noSuccessorDigest string
	if err := tx.QueryRow(ctx, `SELECT config_digest FROM series WHERE id='series-disc-subject'`).Scan(&noSuccessorDigest); err != nil {
		t.Fatalf("reading the successor-less digest: %v", err)
	}
	if noSuccessorDigest == discontinuedDigest {
		t.Fatalf("expected the identity digest to change when the successor is removed, both are %q", discontinuedDigest)
	}
}

// The read half: ListPublishedSeries must carry both facts, because
// publishing.Export sees the database only through it. A discontinued
// series is NOT retired -- retired_at excludes a series from the
// artifact entirely, whereas a discontinued one must still be exported
// with its full history and a permanent banner (spec: "The chart MUST
// NOT be hidden in any state").
func TestListPublishedSeries_CarriesTheDiscontinuation(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)
	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'TESTCOD001', '2020-01-01', 'digest1')`)
	mustExec(t, ctx, tx, `UPDATE series SET discontinued_since = DATE '2026-03-31',
		discontinued_successor_slug = 'ocupados-epa' WHERE id = 'tasa-de-paro-epa'`)

	got, err := postgres.ListPublishedSeries(ctx, tx)
	if err != nil {
		t.Fatalf("ListPublishedSeries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected a discontinued series to still be listed, got %d", len(got))
	}
	if got[0].DiscontinuedSince == nil || got[0].DiscontinuedSince.UTC().Format("2006-01-02") != "2026-03-31" {
		t.Errorf("expected DiscontinuedSince 2026-03-31, got %v", got[0].DiscontinuedSince)
	}
	if got[0].DiscontinuedSuccessorSlug == nil || *got[0].DiscontinuedSuccessorSlug != "ocupados-epa" {
		t.Errorf("expected DiscontinuedSuccessorSlug ocupados-epa, got %v", got[0].DiscontinuedSuccessorSlug)
	}
}

func TestListPublishedSeries_ALiveSeriesCarriesNoDiscontinuation(t *testing.T) {
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
	if got[0].DiscontinuedSince != nil {
		t.Errorf("expected a live series to carry no discontinuation date, got %v", *got[0].DiscontinuedSince)
	}
	if got[0].DiscontinuedSuccessorSlug != nil {
		t.Errorf("expected a live series to carry no successor, got %q", *got[0].DiscontinuedSuccessorSlug)
	}
}
