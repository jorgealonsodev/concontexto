package publishing_test

// Task 3.2 (RED) / 3.3 (GREEN): ValidateArtifact table-driven tests
// (spec publishing-export, "An invalid artifact is never written"):
// accepts a well-formed artifact; rejects an unresolved break/event
// reference, a non-P/D status, and a cadence-inconsistent period.

import (
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// validDoc returns a minimal, fully well-formed SeriesDoc -- every test
// below starts from a deep-enough copy of this and mutates exactly one
// thing, so a failure always isolates the one constraint under test.
func validDoc() publishing.SeriesDoc {
	return publishing.SeriesDoc{
		SchemaVersion: publishing.SchemaVersion,
		Slug:          "tasa-de-paro-epa",
		Name:          "Tasa de paro (EPA)",
		Unit:          "%",
		Frequency:     "Q",
		Decimals:      2,
		Geo:           "ES",
		Operation:     "ine-epa",
		Source: publishing.SourceRef{
			ID: "ine", Name: "INE", Attribution: "Fuente: INE",
			LicenceName: "lic", LicenceURL: "https://ine.es",
		},
		Origin: publishing.OriginRef{Kind: "ine-series-cod", Ref: "TESTCOD001", RequestURL: "https://servicios.ine.es/x"},
		Vintage: publishing.Vintage{
			IngestionRunID: 1,
			ExtractedAt:    time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC),
		},
		Points: []publishing.Point{
			{Period: "2026-Q1", Value: 10.5, Status: "D", Version: 1, IngestionRunID: 1},
			{Period: "2026-Q2", Value: 10.3, Status: "P", Version: 1, IngestionRunID: 1},
		},
		SourceStatus: map[string]string{"2026-Q2": "Provisional"},
		Withdrawn:    []string{},
		Vintages: map[string]publishing.RunProvenance{
			"1": {
				ExtractedAt:   time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC),
				RawFileSHA256: strings.Repeat("a", 64),
				RequestURL:    "https://servicios.ine.es/x",
			},
		},
		Breaks:    []publishing.BreakRef{{Key: "epa-2021-base", Date: "2021-01-01", Kind: "methodological", NoteMD: "note"}},
		Events:    []publishing.EventRef{{ID: "evt-1", Group: "milestones", Name: "Event", DateStart: "2021-01-01"}},
		Freshness: publishing.FreshnessFresh,
		// pageState is a required object on every series document
		// (PageStateRef's own doc comment) -- a well-formed artifact
		// therefore carries one, and the page-state cases in
		// page_state_test.go each mutate exactly this field.
		PageState: publishing.PageStateRef{Kind: publishing.PageStateFresh},
	}
}

func validArtifact() publishing.Artifact {
	return publishing.Artifact{
		Manifest: publishing.Manifest{
			SchemaVersion: publishing.SchemaVersion,
			GeneratedAt:   time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC),
			Series:        []string{"tasa-de-paro-epa"},
			Digests:       map[string]string{"series/tasa-de-paro-epa.json": strings.Repeat("b", 64)},
		},
		Series: []publishing.SeriesDoc{validDoc()},
	}
}

func TestValidateArtifact_AcceptsAWellFormedArtifact(t *testing.T) {
	if err := publishing.ValidateArtifact(validArtifact()); err != nil {
		t.Fatalf("expected a well-formed artifact to validate, got: %v", err)
	}
}

func TestValidateArtifact_RejectsANonPDStatus(t *testing.T) {
	a := validArtifact()
	a.Series[0].Points[0].Status = "W"
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a withdrawn status inside points")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected the error to name the status constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsACadenceInconsistentPeriod(t *testing.T) {
	a := validArtifact()
	// A monthly-shaped label on a series declared quarterly.
	a.Series[0].Points[1].Period = "2026-06"
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a period whose cadence does not match the series' declared frequency")
	}
	if !strings.Contains(err.Error(), "cadence") {
		t.Fatalf("expected the error to name the cadence constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsAnOutOfOrderOrDuplicatePeriod(t *testing.T) {
	a := validArtifact()
	a.Series[0].Points = append(a.Series[0].Points, publishing.Point{
		Period: "2026-Q1", Value: 1, Status: "D", Version: 1, IngestionRunID: 1,
	})
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a duplicated/out-of-order period")
	}
	if !strings.Contains(err.Error(), "cadence") {
		t.Fatalf("expected the error to name the cadence constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsAnUnresolvedBreakReference(t *testing.T) {
	a := validArtifact()
	a.Series[0].Breaks = []publishing.BreakRef{{Key: "dup"}, {Key: "dup"}}
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a duplicated (unresolved) break reference")
	}
	if !strings.Contains(err.Error(), "break-reference") {
		t.Fatalf("expected the error to name the break-reference constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsAnUnresolvedEventReference(t *testing.T) {
	a := validArtifact()
	a.Series[0].Events = []publishing.EventRef{{ID: "dup", DateStart: "2021-01-01"}, {ID: "dup", DateStart: "2022-01-01"}}
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a duplicated (unresolved) event reference")
	}
	if !strings.Contains(err.Error(), "event-reference") {
		t.Fatalf("expected the error to name the event-reference constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsAPointWhoseRunIsNotInVintages(t *testing.T) {
	a := validArtifact()
	a.Series[0].Points[0].IngestionRunID = 999
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a point referencing an ingestion_run_id absent from vintages")
	}
	if !strings.Contains(err.Error(), "vintages") {
		t.Fatalf("expected the error to name the vintages constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsASchemaVersionMismatch(t *testing.T) {
	a := validArtifact()
	a.Manifest.SchemaVersion = 2
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a manifest schema_version mismatch")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected the error to name the schema_version constraint, got: %v", err)
	}
}
