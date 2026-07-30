package publishing_test

// Task 4.2/4.6 (GREEN, wiring): Export now populates SeriesDoc.Breaks and
// SeriesDoc.Events from two new Deps ports -- ResolveActiveBreaksForSeries
// (already existed as postgres.ResolveActiveBreaksForSeries; only the
// Export-level wiring is new this slice) and ListActiveEvents (new this
// slice, events_read.go). Slice 3 left both fields structurally present
// but always empty (artifact.go's own doc comment on BreakRef/EventRef);
// this closes that gap.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func TestExport_PopulatesBreaksAndEventsFromTheirOwnReadPorts(t *testing.T) {
	ps := fakePublishedSeries()
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: "aaaa", RequestURL: "https://ine.es/data",
		}},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	breakDate := time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC)
	sourceURL := "https://ine.es/nota-metodologica"
	deps.ResolveActiveBreaksForSeries = func(_ context.Context, seriesID string) ([]postgres.SeriesBreak, error) {
		if seriesID != ps.Slug {
			t.Fatalf("expected ResolveActiveBreaksForSeries to be called with %q, got %q", ps.Slug, seriesID)
		}
		return []postgres.SeriesBreak{{
			BreakKey: "epa-2020-metodologia", Date: breakDate, Kind: "methodology",
			NoteMD: "Cambio metodológico", SourceURL: &sourceURL,
		}}, nil
	}
	eventStart := time.Date(2008, 9, 15, 0, 0, 0, 0, time.UTC)
	deps.ListActiveEvents = func(_ context.Context, seriesID string) ([]postgres.Event, error) {
		if seriesID != ps.Slug {
			t.Fatalf("expected ListActiveEvents to be called with %q, got %q", ps.Slug, seriesID)
		}
		return []postgres.Event{{ID: "crisis-2008", Group: "exogenous", Name: "Crisis financiera", DateStart: eventStart}}, nil
	}

	artifact, err := publishing.Export(context.Background(), deps, time.Now(), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	doc := artifact.Series[0]

	if len(doc.Breaks) != 1 {
		t.Fatalf("expected exactly one break, got %d: %+v", len(doc.Breaks), doc.Breaks)
	}
	b := doc.Breaks[0]
	if b.Key != "epa-2020-metodologia" || b.Date != "2020-04-01" || b.Kind != "methodology" || b.NoteMD != "Cambio metodológico" {
		t.Fatalf("unexpected break: %+v", b)
	}
	if b.SourceURL == nil || *b.SourceURL != sourceURL {
		t.Fatalf("expected source url %q, got %+v", sourceURL, b.SourceURL)
	}

	if len(doc.Events) != 1 {
		t.Fatalf("expected exactly one event, got %d: %+v", len(doc.Events), doc.Events)
	}
	e := doc.Events[0]
	if e.ID != "crisis-2008" || e.Group != "exogenous" || e.Name != "Crisis financiera" || e.DateStart != "2008-09-15" {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestExport_ASeriesWithNoActiveBreaksOrEventsGetsEmptySlicesNotNil(t *testing.T) {
	ps := fakePublishedSeries()
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
	doc := artifact.Series[0]
	if doc.Breaks == nil || len(doc.Breaks) != 0 {
		t.Fatalf("expected an empty, non-nil Breaks slice, got %+v", doc.Breaks)
	}
	if doc.Events == nil || len(doc.Events) != 0 {
		t.Fatalf("expected an empty, non-nil Events slice, got %+v", doc.Events)
	}
}
