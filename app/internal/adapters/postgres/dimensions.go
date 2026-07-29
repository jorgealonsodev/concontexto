package postgres

// Remediation batch (verify-report CRITICAL C2's prerequisite): IngestSeries
// assumes source/dataset/series/series_source_mapping already exist
// (ingest.go's own doc comment); every existing test seeds them by hand.
// ReconcileEditorialConfig only ever reconciled Breaks/Events, never
// series identity (reconcile.go's own doc comment) -- so nothing in
// production ever wrote these rows, and `ingest --series|--source`
// against a real, fresh database would fail on the first foreign key. This
// file is that missing reconcile, upsert-only and idempotent like
// ReconcileBreaks/ReconcileEvents, applied to config/sources/*.yaml and
// config/series/*.yaml instead of rupturas.yaml/eventos.yaml.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func simpleDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ReconcileDimensions upserts every source, dataset and series identity
// row cfg declares, plus each series' currently active
// series_source_mapping. It is safe to call before every ingest run: an
// unchanged config changes zero rows (idempotence), and a series whose
// active source_ref differs from what is already recorded closes the old
// mapping and opens a new one (spec data-model-vintages, "An identifier
// change adds a mapping instead of replacing one") rather than being
// silently skipped.
func ReconcileDimensions(ctx context.Context, db TxBeginner, cfg *config.Config, now time.Time) error {
	for _, src := range cfg.Sources {
		if err := reconcileSource(ctx, db, src); err != nil {
			return err
		}
	}
	for _, s := range cfg.Series {
		if s.Source == "" || s.Dataset == "" {
			continue // validate-config already rejects this; defensive skip, not a silent guess
		}
		if err := reconcileDataset(ctx, db, s.Dataset, s.Source); err != nil {
			return err
		}
		if err := reconcileSeriesIdentity(ctx, db, s); err != nil {
			return err
		}
		ref, ok := ActiveSourceRef(s)
		if !ok {
			continue // a series with no active (ValidTo == nil) source_ref has nothing to map yet
		}
		if err := reconcileActiveMapping(ctx, db, s.Slug, ref, now); err != nil {
			return err
		}
	}
	return nil
}

// ActiveSourceRef resolves s's currently active (ValidTo == nil)
// source_ref -- the same "active mapping" convention
// app/internal/probe.Targets already established for the synthetic
// probe, generalised here to all three kinds (ine-series-cod,
// eurostat-dataset, xlsx-url) since dimension reconciliation, unlike the
// probe, must cover every configured series regardless of source kind.
func ActiveSourceRef(s config.SeriesConfig) (config.SourceRef, bool) {
	for _, ref := range s.SourceRefs {
		if ref.ValidTo == nil {
			return ref, true
		}
	}
	return config.SourceRef{}, false
}

func reconcileSource(ctx context.Context, db DBTX, src config.SourceConfig) error {
	_, err := db.Exec(ctx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, url=EXCLUDED.url, license=EXCLUDED.license,
			attribution_text=EXCLUDED.attribution_text, access_type=EXCLUDED.access_type, config_digest=EXCLUDED.config_digest`,
		src.ID, src.Name, src.URL, src.Licence.Name, src.Licence.AttributionText, src.AccessType, sourceDigest(src))
	if err != nil {
		return fmt.Errorf("postgres: reconciling source %q: %w", src.ID, err)
	}
	return nil
}

func reconcileDataset(ctx context.Context, db DBTX, datasetID, sourceID string) error {
	_, err := db.Exec(ctx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ($1, $2, $1, $3)
		ON CONFLICT (id) DO UPDATE SET source_id=EXCLUDED.source_id, config_digest=EXCLUDED.config_digest`,
		datasetID, sourceID, datasetDigest(datasetID, sourceID))
	if err != nil {
		return fmt.Errorf("postgres: reconciling dataset %q: %w", datasetID, err)
	}
	return nil
}

func reconcileSeriesIdentity(ctx context.Context, db DBTX, s config.SeriesConfig) error {
	_, err := db.Exec(ctx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ($1, $2, $1, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET dataset_id=EXCLUDED.dataset_id, unit=EXCLUDED.unit, frequency=EXCLUDED.frequency,
			geo=EXCLUDED.geo, decimals=EXCLUDED.decimals, is_harmonized=EXCLUDED.is_harmonized, config_digest=EXCLUDED.config_digest`,
		s.Slug, s.Dataset, s.Unit, s.Frequency, s.Geo, s.Decimals, s.Harmonized, seriesIdentityDigest(s))
	if err != nil {
		return fmt.Errorf("postgres: reconciling series %q: %w", s.Slug, err)
	}
	return nil
}

// reconcileActiveMapping leaves the currently active mapping untouched
// when it already matches ref (idempotence); otherwise it closes the old
// one and opens a new one (or opens the first one, when none exists yet)
// at ref's own declared ValidFrom -- never at now, so a source_ref
// declared valid from a past date is recorded as having been active
// since that date, not since the moment this reconcile happened to run.
func reconcileActiveMapping(ctx context.Context, db TxBeginner, seriesID string, ref config.SourceRef, now time.Time) error {
	var curKind, curRef string
	err := db.QueryRow(ctx, `SELECT ref_kind, ref FROM series_source_mapping
		WHERE series_id=$1 AND valid_to IS NULL AND retired_at IS NULL`, seriesID).Scan(&curKind, &curRef)
	switch {
	case err == nil:
		if curKind == ref.Kind && curRef == ref.Ref {
			return nil // unchanged: nothing to reconcile
		}
		if _, err := NewSourceMappingRepo(db).ReplaceActiveMapping(ctx, seriesID, ref.ValidFrom, SourceMappingInput{
			SeriesID: seriesID, RefKind: ref.Kind, Ref: ref.Ref, ValidFrom: ref.ValidFrom, ConfigDigest: sourceRefDigest(ref),
		}); err != nil {
			return fmt.Errorf("postgres: replacing the active mapping for %q: %w", seriesID, err)
		}
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := db.Exec(ctx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
			VALUES ($1, $2, $3, $4, $5)`, seriesID, ref.Kind, ref.Ref, ref.ValidFrom, sourceRefDigest(ref)); err != nil {
			return fmt.Errorf("postgres: inserting the first mapping for %q: %w", seriesID, err)
		}
		return nil
	default:
		return fmt.Errorf("postgres: reading the active mapping for %q: %w", seriesID, err)
	}
}

// sourceDigest/datasetDigest/seriesIdentityDigest/sourceRefDigest mirror
// ingestion.breakDigest/eventDigest's convention (a plain, non-
// cryptographic-strength SHA-256 over exactly the fields that reach the
// row) so a config edit is distinguishable from a delete plus insert,
// same as every other reconciled table in this schema.
func sourceDigest(src config.SourceConfig) string {
	return simpleDigest(fmt.Sprintf("id=%s\nname=%s\nurl=%s\nlicence=%s\nattribution=%s\naccess=%s\n",
		src.ID, src.Name, src.URL, src.Licence.Name, src.Licence.AttributionText, src.AccessType))
}

func datasetDigest(datasetID, sourceID string) string {
	return simpleDigest(fmt.Sprintf("id=%s\nsource_id=%s\n", datasetID, sourceID))
}

func seriesIdentityDigest(s config.SeriesConfig) string {
	return simpleDigest(fmt.Sprintf("id=%s\ndataset_id=%s\nunit=%s\nfrequency=%s\ngeo=%s\ndecimals=%d\nharmonized=%t\n",
		s.Slug, s.Dataset, s.Unit, s.Frequency, s.Geo, s.Decimals, s.Harmonized))
}

func sourceRefDigest(ref config.SourceRef) string {
	return simpleDigest(fmt.Sprintf("kind=%s\nref=%s\nvalid_from=%s\n", ref.Kind, ref.Ref, ref.ValidFrom.Format("2006-01-02")))
}
