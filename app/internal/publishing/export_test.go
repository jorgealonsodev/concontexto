package publishing_test

// Task 3.4 (RED) / 3.5 (GREEN): Export() against fake ports -- no
// database, no filesystem beyond t.TempDir() (task 3.8/3.9's atomic-
// write proof lives in this same file). Task 3.6 (RED) / 3.7 (GREEN):
// freshness-in-artifact derives only from SeriesFreshness, no
// build-staleness field.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func fakePublishedSeries() postgres.PublishedSeries {
	return postgres.PublishedSeries{
		Slug: "tasa-de-paro-epa", Name: "Tasa de paro (EPA)", Unit: "%", Frequency: "Q", Decimals: 2, Geo: "ES",
		DatasetID:         "ine-epa",
		SourceID:          "ine",
		SourceName:        "INE",
		SourceAttribution: "Fuente: INE",
		SourceLicenceName: "lic",
		SourceURL:         "https://ine.es",
		OriginKind:        "ine-series-cod",
		OriginRef:         "TESTCOD001",
	}
}

func ptrf(v float64) *float64 { return &v }

func fakeDeps(t *testing.T, series []postgres.PublishedSeries, obs map[string][]postgres.PublishedObservation, state freshness.State) publishing.Deps {
	t.Helper()
	return publishing.Deps{
		ListPublishedSeries: func(context.Context) ([]postgres.PublishedSeries, error) {
			return series, nil
		},
		ListObservations: func(_ context.Context, seriesID string) ([]postgres.PublishedObservation, error) {
			return obs[seriesID], nil
		},
		SeriesFreshness: func(context.Context, string, time.Time) (freshness.State, error) {
			return state, nil
		},
		// Every existing test predates slice 4's breaks/events wiring;
		// defaulting both to "none" here keeps every one of them green
		// without touching their own call sites (task 4.2/4.6's own
		// tests override these two fields directly, see events_test.go).
		ResolveActiveBreaksForSeries: func(context.Context, string) ([]postgres.SeriesBreak, error) {
			return nil, nil
		},
		ListActiveEvents: func(context.Context, string) ([]postgres.Event, error) {
			return nil, nil
		},
		// SeriesValidationOutcome is REQUIRED since the WARNING-17
		// remediation (Export refuses an unbound one), so every fake must
		// bind it. The zero ValidationOutcome is the honest default for a
		// test that says nothing about validation: no failure recorded, no
		// prior success recorded -- which SeriesPageState resolves to
		// "fresh" for a live series, exactly the state these older cases
		// already asserted. page_state_test.go's own cases override this
		// field directly, the same way events_test.go overrides the two
		// above.
		SeriesValidationOutcome: func(context.Context, string) (postgres.ValidationOutcome, error) {
			return postgres.ValidationOutcome{}, nil
		},
	}
}

func TestExport_OnlyCurrentPublishedDataEntersTheArtifact(t *testing.T) {
	ps := fakePublishedSeries()
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {
			{
				Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
				IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es/data",
			},
		},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(artifact.Series) != 1 {
		t.Fatalf("expected exactly one series doc, got %d", len(artifact.Series))
	}
	doc := artifact.Series[0]
	if len(doc.Points) != 1 || doc.Points[0].Period != "2026-Q1" || doc.Points[0].Value != 10.5 {
		t.Fatalf("expected exactly the one current observation as a point, got %+v", doc.Points)
	}
}

// TestExport_AFailedRunsSuspectDatumIsAbsentWhileThePriorValidValueRemains
// proves Export builds strictly from what postgres.ListPublishedObservations
// (is_current only) reports -- a failed run never writes an observation
// row at all (ApplyGate's Block branch), so the prior current row is
// exactly, and only, what a fake simulating that state returns.
func TestExport_AFailedRunsSuspectDatumIsAbsentWhileThePriorValidValueRemains(t *testing.T) {
	ps := fakePublishedSeries()
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {
			{
				Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
				IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es/data",
			},
			// No 2026-Q2 row: that run's candidate was blocked and never
			// reached the writer -- there is nothing to represent it.
		},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	doc := artifact.Series[0]
	for _, p := range doc.Points {
		if p.Period == "2026-Q2" {
			t.Fatalf("expected no 2026-Q2 point (its run failed), got %+v", p)
		}
	}
	if len(doc.Points) != 1 || doc.Points[0].Period != "2026-Q1" {
		t.Fatalf("expected the prior valid 2026-Q1 value to remain, got %+v", doc.Points)
	}
}

func TestExport_ProvenanceResolvesPerObservationWithoutAFurtherQuery(t *testing.T) {
	ps := fakePublishedSeries()
	extractedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {
			{
				Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
				IngestionRunID: 412, ExtractedAt: extractedAt,
				RawFileSHA256: strings.Repeat("c", 64), RequestURL: "https://ine.es/TESTCOD001",
			},
		},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	doc := artifact.Series[0]
	point := doc.Points[0]
	if point.IngestionRunID != 412 {
		t.Fatalf("expected the point to carry ingestion_run_id 412, got %d", point.IngestionRunID)
	}
	rp, ok := doc.Vintages["412"]
	if !ok {
		t.Fatal("expected vintages[\"412\"] to resolve the point's run -- D1's own fix")
	}
	if !rp.ExtractedAt.Equal(extractedAt) {
		t.Fatalf("expected vintages[412].extractedAt == %v, got %v", extractedAt, rp.ExtractedAt)
	}
	if rp.RawFileSHA256 != strings.Repeat("c", 64) {
		t.Fatalf("expected vintages[412].rawFileSha256 to resolve, got %q", rp.RawFileSHA256)
	}
	if doc.Vintage.IngestionRunID != 412 || !doc.Vintage.ExtractedAt.Equal(extractedAt) {
		t.Fatalf("expected the series-level vintage convenience field to match, got %+v", doc.Vintage)
	}
	if doc.Origin.RequestURL != "https://ine.es/TESTCOD001" {
		t.Fatalf("expected origin.requestUrl to resolve from the same run, got %q", doc.Origin.RequestURL)
	}
}

func TestExport_SkipsASeriesWithNoPublishedObservations(t *testing.T) {
	ps := fakePublishedSeries()
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, map[string][]postgres.PublishedObservation{}, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(artifact.Series) != 0 {
		t.Fatalf("expected zero series docs for a series with nothing published yet, got %d", len(artifact.Series))
	}
}

func TestExport_FreshnessDerivesOnlyFromSeriesFreshness(t *testing.T) {
	tests := []struct {
		name  string
		state freshness.State
		want  string
	}{
		{"fresh source stays fresh", freshness.StateFresh, publishing.FreshnessFresh},
		{"failed source reads as source-pending", freshness.StateFailed, publishing.FreshnessSourcePending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := fakePublishedSeries()
			obs := map[string][]postgres.PublishedObservation{
				ps.Slug: {{
					Period: "2026-Q1", Value: ptrf(1), Status: postgres.StatusDefinitive, Version: 1,
					IngestionRunID: 1, ExtractedAt: time.Now(), RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es",
				}},
			}
			deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, tt.state)

			artifact, err := publishing.Export(context.Background(), deps, time.Now(), t.TempDir())
			if err != nil {
				t.Fatalf("Export: %v", err)
			}
			if artifact.Series[0].Freshness != tt.want {
				t.Fatalf("expected freshness %q, got %q", tt.want, artifact.Series[0].Freshness)
			}
		})
	}
}

func TestExport_WithdrawnObservationsAreExcludedFromPointsAndListedSeparately(t *testing.T) {
	ps := fakePublishedSeries()
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {
			{
				Period: "2026-Q1", Value: nil, Status: postgres.StatusWithdrawn, Version: 2,
				IngestionRunID: 1, ExtractedAt: time.Now(), RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es",
			},
		},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Now(), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	doc := artifact.Series[0]
	if len(doc.Points) != 0 {
		t.Fatalf("expected a withdrawn observation to be excluded from points, got %+v", doc.Points)
	}
	if len(doc.Withdrawn) != 1 || doc.Withdrawn[0] != "2026-Q1" {
		t.Fatalf("expected 2026-Q1 to be listed under withdrawn, got %+v", doc.Withdrawn)
	}
}

// TestExport_AValidationFailureAbortsWritesNothingAndLeavesThePreviousArtifactByteIdentical
// is task 3.8/3.9's proof: a series doc with an empty Name fails
// ValidateArtifact's "name" constraint. No file must change.
func TestExport_AValidationFailureAbortsWritesNothingAndLeavesThePreviousArtifactByteIdentical(t *testing.T) {
	outDir := t.TempDir()

	// Seed a "previous artifact" the failing run must not touch.
	seriesDir := filepath.Join(outDir, "series")
	if err := os.MkdirAll(seriesDir, 0o755); err != nil {
		t.Fatalf("seeding previous artifact dir: %v", err)
	}
	prevManifest := []byte(`{"schema_version":1,"previous":true}`)
	prevSeries := []byte(`{"slug":"tasa-de-paro-epa","previous":true}`)
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), prevManifest, 0o644); err != nil {
		t.Fatalf("seeding previous manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seriesDir, "tasa-de-paro-epa.json"), prevSeries, 0o644); err != nil {
		t.Fatalf("seeding previous series doc: %v", err)
	}

	ps := fakePublishedSeries()
	ps.Name = "" // forces ValidateArtifact's "name" rejection
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {{
			Period: "2026-Q1", Value: ptrf(1), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Now(), RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es",
		}},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	_, err := publishing.Export(context.Background(), deps, time.Now(), outDir)
	if err == nil {
		t.Fatal("expected Export to fail validation for a series doc with no name")
	}

	gotManifest, readErr := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if readErr != nil {
		t.Fatalf("reading manifest.json after the failed export: %v", readErr)
	}
	if string(gotManifest) != string(prevManifest) {
		t.Fatalf("expected the previous manifest to stay byte-identical, got %q", gotManifest)
	}
	gotSeries, readErr := os.ReadFile(filepath.Join(seriesDir, "tasa-de-paro-epa.json"))
	if readErr != nil {
		t.Fatalf("reading the previous series doc after the failed export: %v", readErr)
	}
	if string(gotSeries) != string(prevSeries) {
		t.Fatalf("expected the previous series doc to stay byte-identical, got %q", gotSeries)
	}

	entries, err := os.ReadDir(seriesDir)
	if err != nil {
		t.Fatalf("reading series dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected no new file to have been written, got %d entries: %v", len(entries), entries)
	}
}

func TestExport_WritesAValidArtifactAtomicallyToDisk(t *testing.T) {
	outDir := t.TempDir()
	ps := fakePublishedSeries()
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Now(), RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es",
		}},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)

	artifact, err := publishing.Export(context.Background(), deps, time.Now(), outDir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("reading manifest.json: %v", err)
	}
	var manifest publishing.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshalling manifest.json: %v", err)
	}
	if manifest.SchemaVersion != publishing.SchemaVersion {
		t.Fatalf("expected schema_version %d, got %d", publishing.SchemaVersion, manifest.SchemaVersion)
	}
	digest, ok := manifest.Digests["series/tasa-de-paro-epa.json"]
	if !ok || digest == "" {
		t.Fatalf("expected a non-empty digest for series/tasa-de-paro-epa.json, got %+v", manifest.Digests)
	}

	seriesBytes, err := os.ReadFile(filepath.Join(outDir, "series", "tasa-de-paro-epa.json"))
	if err != nil {
		t.Fatalf("reading series/tasa-de-paro-epa.json: %v", err)
	}
	var doc publishing.SeriesDoc
	if err := json.Unmarshal(seriesBytes, &doc); err != nil {
		t.Fatalf("unmarshalling series doc: %v", err)
	}
	if doc.Slug != "tasa-de-paro-epa" || len(doc.Points) != 1 {
		t.Fatalf("expected the on-disk series doc to match the returned artifact, got %+v", doc)
	}
	if artifact.Manifest.Digests["series/tasa-de-paro-epa.json"] != digest {
		t.Fatalf("expected the returned artifact's digest to match the on-disk manifest's digest")
	}
}
