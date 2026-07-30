package publishing_test

// Slice 4: closes the slice-3 disclosed gap (design.md's "Disclosed
// gaps" note) that publishing.SourceRef.LicenceURL was always populated
// from postgres.PublishedSeries.SourceURL (the source's general
// website), never a distinct licence-terms URL -- migration
// 0004_source_licence_url adds the column this test proves Export now
// prefers.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func TestExport_PrefersDistinctLicenceURLOverGeneralSourceURL(t *testing.T) {
	ps := fakePublishedSeries()
	ps.SourceURL = "https://ine.es"
	ps.SourceLicenceURL = "https://ine.es/condiciones-de-uso"
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: "aaaa", RequestURL: "https://ine.es/data",
		}},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Now(), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	got := artifact.Series[0].Source.LicenceURL
	if got != "https://ine.es/condiciones-de-uso" {
		t.Fatalf("expected the distinct licence URL to win over the general source URL, got %q", got)
	}
}

func TestExport_FallsBackToGeneralSourceURLWhenLicenceURLNotYetReconciled(t *testing.T) {
	ps := fakePublishedSeries()
	ps.SourceURL = "https://ine.es"
	ps.SourceLicenceURL = "" // not yet re-reconciled since migration 0004 shipped
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: "aaaa", RequestURL: "https://ine.es/data",
		}},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Now(), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	got := artifact.Series[0].Source.LicenceURL
	if got != "https://ine.es" {
		t.Fatalf("expected the transitional fallback to the general source URL, got %q", got)
	}
}
