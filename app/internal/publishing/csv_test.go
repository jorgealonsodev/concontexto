package publishing_test

// Task 4.3 (RED) / 4.4 (GREEN): the CSV projection (design's supporting
// decisions table, "one file per series: period,value,status,
// source_status,version + header comment with licence/attribution").
// buildSeriesCSV is a pure function of one SeriesDoc -- the exact
// in-memory document Export already builds -- so "every /data-derived
// row matches the artifact value-by-value" (spec) is true by
// construction: the CSV writer and the JSON writer read the identical
// struct, never two independently-derived views.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func TestBuildSeriesCSV_HeaderCommentCarriesLicenceAndAttribution(t *testing.T) {
	doc := publishing.SeriesDoc{
		Slug: "tasa-de-paro-epa",
		Source: publishing.SourceRef{
			Attribution: "Fuente: INE, Encuesta de Población Activa",
			LicenceName: "INE — Reutilización de la información",
			LicenceURL:  "https://ine.es/condiciones-de-uso",
		},
	}
	csv := string(publishing.BuildSeriesCSV(doc))
	lines := strings.Split(csv, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 2 header comment lines + a column header, got %d lines: %q", len(lines), csv)
	}
	if lines[0] != "# Fuente: INE, Encuesta de Población Activa" {
		t.Errorf("expected the first line to be the attribution comment, got %q", lines[0])
	}
	if lines[1] != "# INE — Reutilización de la información (https://ine.es/condiciones-de-uso)" {
		t.Errorf("expected the second line to be the licence comment, got %q", lines[1])
	}
	if lines[2] != "period,value,status,source_status,version" {
		t.Errorf("expected the column header on line 3, got %q", lines[2])
	}
}

func TestBuildSeriesCSV_RowsMatchPointsValueByValue(t *testing.T) {
	doc := publishing.SeriesDoc{
		Slug:   "tasa-de-paro-epa",
		Source: publishing.SourceRef{Attribution: "attr", LicenceName: "lic", LicenceURL: "https://x"},
		Points: []publishing.Point{
			{Period: "2002-Q1", Value: 11.47, Status: "D", Version: 3, IngestionRunID: 200},
			{Period: "2026-Q2", Value: 10.29, Status: "P", Version: 1, IngestionRunID: 412},
		},
		SourceStatus: map[string]string{"2026-Q2": "Provisional"},
	}
	got := string(publishing.BuildSeriesCSV(doc))
	if !strings.Contains(got, "\n2002-Q1,11.47,D,,3\n") {
		t.Errorf("expected a row for the definitive point with empty source_status, got:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n2026-Q2,10.29,P,Provisional,1\n") {
		t.Errorf("expected a row for the provisional point carrying its verbatim source_status, got:\n%s", got)
	}
}

func TestExport_WritesOneCSVFilePerSeriesMatchingTheArtifact(t *testing.T) {
	ps := fakePublishedSeries()
	obs := map[string][]postgres.PublishedObservation{
		ps.Slug: {{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: "aaaa", RequestURL: "https://ine.es/data",
		}},
	}
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, obs, freshness.StateFresh)
	outDir := t.TempDir()

	artifact, err := publishing.Export(context.Background(), deps, time.Now(), outDir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	csvBytes, err := os.ReadFile(filepath.Join(outDir, "csv", "tasa-de-paro-epa.csv"))
	if err != nil {
		t.Fatalf("reading csv/tasa-de-paro-epa.csv: %v", err)
	}
	if !strings.Contains(string(csvBytes), "2026-Q1,10.5,D,,1\n") {
		t.Fatalf("expected the CSV row to match the artifact's own point, got:\n%s", csvBytes)
	}
	if len(artifact.Series) != 1 {
		t.Fatalf("expected exactly one series in the returned artifact, got %d", len(artifact.Series))
	}
}
