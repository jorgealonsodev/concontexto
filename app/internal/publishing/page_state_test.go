package publishing_test

// Remediation B (verify-report CRITICAL-4), artifact half: the export
// artifact now carries the page state PRD §6.1.3's three banners are
// driven by, so a real validation failure or a real discontinuation
// reaches a reader from DATA rather than from a hand-edited constant in
// web/src/content/indicators/methodology.ts plus a redeploy.
//
// Closes publishing-export's "Only published data enters the artifact",
// scenario clause "AND the series carries the state that drives PRD
// §6.1.3's validation banner", and gives indicator-page's "The three
// page states of PRD §6.1.3" the data path it was missing.
//
// Every case here drives Export through fake ports -- no database, no
// SQL -- exactly as export_test.go's own suite does. The database half
// (postgres.SeriesValidationOutcome, postgres.ListPublishedSeries's two
// discontinuation columns) is proved separately against a real
// container in app/internal/adapters/postgres.

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

// pageStateObservations is one ordinary published point, enough for
// Export to consider the series published at all (it skips a series
// with no observations).
func pageStateObservations(slug string) map[string][]postgres.PublishedObservation {
	return map[string][]postgres.PublishedObservation{
		slug: {{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es/data",
		}},
	}
}

// exportPageState runs a full Export over one series and returns the
// page state its document carries.
func exportPageState(t *testing.T, ps postgres.PublishedSeries, outcome postgres.ValidationOutcome) publishing.PageStateRef {
	t.Helper()
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, pageStateObservations(ps.Slug), freshness.StateFresh)
	deps.SeriesValidationOutcome = func(context.Context, string) (postgres.ValidationOutcome, error) {
		return outcome, nil
	}
	artifact, err := publishing.Export(context.Background(), deps, time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC), t.TempDir())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(artifact.Series) != 1 {
		t.Fatalf("expected exactly one series doc, got %d", len(artifact.Series))
	}
	return artifact.Series[0].PageState
}

func TestExport_ALiveSeriesWhoseLatestRunSucceededIsFresh(t *testing.T) {
	lastSuccess := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)
	got := exportPageState(t, fakePublishedSeries(), postgres.ValidationOutcome{
		LatestRunFailedValidation: false, LastSucceededAt: &lastSuccess,
	})

	if got.Kind != publishing.PageStateFresh {
		t.Fatalf("expected kind %q, got %q", publishing.PageStateFresh, got.Kind)
	}
	// The date of the last correct update belongs to the validation
	// banner and nowhere else. Emitting it on a fresh page would put a
	// field on the document that no state consumes, inviting the web
	// layer to render "last correct update" copy on a page that has no
	// failure to explain.
	if got.LastCorrectUpdate != nil {
		t.Errorf("expected no lastCorrectUpdate on a fresh page, got %q", *got.LastCorrectUpdate)
	}
	if got.SuccessorSlug != nil {
		t.Errorf("expected no successorSlug on a fresh page, got %q", *got.SuccessorSlug)
	}
}

// indicator-page spec, scenario "A validation failure serves the last
// valid datum with a banner": the banner names {fecha}, and that date is
// the last SUCCEEDED run, not the failed one.
func TestExport_AFailedLatestRunCarriesTheDateOfTheLastCorrectUpdate(t *testing.T) {
	lastSuccess := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)
	got := exportPageState(t, fakePublishedSeries(), postgres.ValidationOutcome{
		LatestRunFailedValidation: true, LastSucceededAt: &lastSuccess,
	})

	if got.Kind != publishing.PageStateValidationFailure {
		t.Fatalf("expected kind %q, got %q", publishing.PageStateValidationFailure, got.Kind)
	}
	if got.LastCorrectUpdate == nil {
		t.Fatal("expected a lastCorrectUpdate date for a validation failure with a prior success")
	}
	if *got.LastCorrectUpdate != "2026-04-30" {
		t.Errorf("expected lastCorrectUpdate 2026-04-30 (the last SUCCEEDED run), got %q", *got.LastCorrectUpdate)
	}
	if got.SuccessorSlug != nil {
		t.Errorf("expected no successorSlug on a validation failure, got %q", *got.SuccessorSlug)
	}
}

// The deliberate edge case: a series whose very first run failed
// validation has a real failure and no date to name. The failure is
// still reported; lastCorrectUpdate is null. See SeriesPageState's own
// doc comment for why null rather than a substituted date or a silent
// downgrade to "fresh".
func TestExport_AFailedFirstRunReportsTheFailureWithNoDateRatherThanHidingIt(t *testing.T) {
	got := exportPageState(t, fakePublishedSeries(), postgres.ValidationOutcome{
		LatestRunFailedValidation: true, LastSucceededAt: nil,
	})

	if got.Kind != publishing.PageStateValidationFailure {
		t.Fatalf("expected a validation failure with no prior success to still report %q, got %q",
			publishing.PageStateValidationFailure, got.Kind)
	}
	if got.LastCorrectUpdate != nil {
		t.Errorf("expected lastCorrectUpdate to be null when no run has ever succeeded, got %q", *got.LastCorrectUpdate)
	}
}

func TestExport_ADiscontinuedSeriesCarriesItsSuccessor(t *testing.T) {
	ps := fakePublishedSeries()
	since := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	successor := "ocupados-epa"
	ps.DiscontinuedSince = &since
	ps.DiscontinuedSuccessorSlug = &successor

	got := exportPageState(t, ps, postgres.ValidationOutcome{})

	if got.Kind != publishing.PageStateDiscontinued {
		t.Fatalf("expected kind %q, got %q", publishing.PageStateDiscontinued, got.Kind)
	}
	if got.SuccessorSlug == nil || *got.SuccessorSlug != "ocupados-epa" {
		t.Fatalf("expected successorSlug ocupados-epa, got %v", got.SuccessorSlug)
	}
	if got.LastCorrectUpdate != nil {
		t.Errorf("expected no lastCorrectUpdate on a discontinued page, got %q", *got.LastCorrectUpdate)
	}
}

// "where one exists" (spec) -- a source can retire a series without
// publishing a replacement, and the permanent banner must still render.
func TestExport_ADiscontinuedSeriesWithNoSuccessorCarriesANullSuccessor(t *testing.T) {
	ps := fakePublishedSeries()
	since := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	ps.DiscontinuedSince = &since

	got := exportPageState(t, ps, postgres.ValidationOutcome{})

	if got.Kind != publishing.PageStateDiscontinued {
		t.Fatalf("expected kind %q, got %q", publishing.PageStateDiscontinued, got.Kind)
	}
	if got.SuccessorSlug != nil {
		t.Errorf("expected a null successorSlug when none is configured, got %q", *got.SuccessorSlug)
	}
}

// Precedence: discontinued outranks validation-failure. A retired series
// is permanently retired; a validation banner layered on top would be
// noise about a pipeline the reader no longer has a stake in.
func TestExport_DiscontinuationOutranksAConcurrentValidationFailure(t *testing.T) {
	ps := fakePublishedSeries()
	since := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	successor := "ocupados-epa"
	ps.DiscontinuedSince = &since
	ps.DiscontinuedSuccessorSlug = &successor
	lastSuccess := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)

	got := exportPageState(t, ps, postgres.ValidationOutcome{
		LatestRunFailedValidation: true, LastSucceededAt: &lastSuccess,
	})

	if got.Kind != publishing.PageStateDiscontinued {
		t.Fatalf("expected discontinued to outrank a validation failure, got kind %q", got.Kind)
	}
	if got.LastCorrectUpdate != nil {
		t.Errorf("expected the suppressed validation banner to carry no date, got %q", *got.LastCorrectUpdate)
	}
}

// The web contract: pageState is a REQUIRED object, never omitted, and
// its three keys are always present -- explicit null rather than an
// absent key. A reader (the Astro Zod loader) can then distinguish "this
// artifact predates page state" from "this series has no successor"
// without guessing.
func TestExport_TheWrittenSeriesJSONAlwaysCarriesTheFullPageStateObject(t *testing.T) {
	ps := fakePublishedSeries()
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, pageStateObservations(ps.Slug), freshness.StateFresh)
	outDir := t.TempDir()
	if _, err := publishing.Export(context.Background(), deps, time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC), outDir); err != nil {
		t.Fatalf("Export: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(outDir, "series", ps.Slug+".json"))
	if err != nil {
		t.Fatalf("reading the written series doc: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding the written series doc: %v", err)
	}
	pageState, ok := decoded["pageState"]
	if !ok {
		t.Fatalf("expected a pageState key in the written document, got keys %v", mapKeys(decoded))
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(pageState, &fields); err != nil {
		t.Fatalf("decoding pageState: %v", err)
	}
	for _, key := range []string{"kind", "lastCorrectUpdate", "successorSlug"} {
		if _, present := fields[key]; !present {
			t.Errorf("expected pageState.%s to be present (explicit null, never omitted), got %s", key, pageState)
		}
	}
	if string(fields["kind"]) != `"fresh"` {
		t.Errorf("expected kind \"fresh\", got %s", fields["kind"])
	}
	if string(fields["lastCorrectUpdate"]) != "null" || string(fields["successorSlug"]) != "null" {
		t.Errorf("expected both optional fields to be explicit null, got %s", pageState)
	}
}

// Remediation batch (verify-report WARNING-17): the SeriesValidationOutcome
// port used to be OPTIONAL -- a nil port meant "no failure known", which
// resolves to PageStateFresh for every live series. That default made the
// one production binding (buildExportDeps, app/cmd/concontexto/
// export_cmd.go) silently load-bearing: deleting that single line would
// have reverted CRITICAL-4 in full -- every page reporting "fresh"
// regardless of its real validation outcome -- with the entire Go suite
// still green, because no test anywhere depended on the port being bound.
//
// The port is now REQUIRED, and this test is the guard. It asserts the
// stronger of the two available properties: not "someone notices", but
// "the artifact cannot be written at all". An unbound port is a
// composition defect in the writer, and the only honest response to
// "I cannot know whether this series' latest run failed" is to refuse to
// publish a page-state claim about it -- exactly the fail-closed reasoning
// firstUnclassifiedStatus already applies to an unclassified observation
// status (app/internal/ingestion/ingest.go) and validatePageState applies
// to an unrecognised kind.
//
// Note what is NOT asserted here: that the OTHER five ports are non-nil.
// They need no such check, because Export calls every one of them
// unconditionally -- a nil one panics on first use, which is loud. This
// port was the only one whose absence was silently absorbed, so it is the
// only one that needs an explicit refusal.
func TestExport_AnUnboundValidationOutcomePortIsRefusedRatherThanReportedAsFresh(t *testing.T) {
	ps := fakePublishedSeries()
	deps := fakeDeps(t, []postgres.PublishedSeries{ps}, pageStateObservations(ps.Slug), freshness.StateFresh)
	deps.SeriesValidationOutcome = nil
	outDir := t.TempDir()

	_, err := publishing.Export(context.Background(), deps, time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC), outDir)
	if err == nil {
		t.Fatal("expected Export to refuse an unbound SeriesValidationOutcome port; a nil port silently reports every series as fresh (CRITICAL-4)")
	}
	if !strings.Contains(err.Error(), "SeriesValidationOutcome") {
		t.Errorf("expected the error to name the unbound port so the composition defect is findable, got: %v", err)
	}

	// Refusing means refusing BEFORE any byte lands: a half-exported
	// artifact carrying fabricated "fresh" page states would be worse than
	// no export at all (spec publishing-export, "MUST NOT write a partial
	// artifact, and MUST leave the previously exported artifact intact").
	if entries, readErr := os.ReadDir(outDir); readErr != nil {
		t.Fatalf("reading the output directory: %v", readErr)
	} else if len(entries) != 0 {
		t.Errorf("expected nothing written when the port is unbound, got %d entries in %s", len(entries), outDir)
	}
}

func mapKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ValidateArtifact is the write-side gate (spec publishing-export, "The
// artifact is validated on write"). The page state is a closed
// three-value enum with two conditionally-populated fields, so every one
// of those constraints has to be enforced before a byte reaches disk --
// otherwise a writer bug ships a document the web loader will reject at
// build time, after the artifact has already replaced a good one.

func TestValidateArtifact_RejectsAnUnrecognisedPageStateKind(t *testing.T) {
	a := validArtifact()
	a.Series[0].PageState = publishing.PageStateRef{Kind: "stale"}
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for an unrecognised page-state kind")
	}
	if !strings.Contains(err.Error(), "page-state") {
		t.Fatalf("expected the error to name the page-state constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsAnEmptyPageStateKind(t *testing.T) {
	a := validArtifact()
	a.Series[0].PageState = publishing.PageStateRef{}
	if err := publishing.ValidateArtifact(a); err == nil {
		t.Fatal("expected a rejection for a missing page-state kind (pageState is required, never omitted)")
	}
}

func TestValidateArtifact_RejectsALastCorrectUpdateOnANonFailureState(t *testing.T) {
	a := validArtifact()
	date := "2026-04-30"
	a.Series[0].PageState = publishing.PageStateRef{Kind: publishing.PageStateFresh, LastCorrectUpdate: &date}
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a lastCorrectUpdate on a fresh page")
	}
	if !strings.Contains(err.Error(), "page-state") {
		t.Fatalf("expected the error to name the page-state constraint, got: %v", err)
	}
}

func TestValidateArtifact_RejectsANonISOLastCorrectUpdate(t *testing.T) {
	a := validArtifact()
	date := "30/04/2026"
	a.Series[0].PageState = publishing.PageStateRef{Kind: publishing.PageStateValidationFailure, LastCorrectUpdate: &date}
	if err := publishing.ValidateArtifact(a); err == nil {
		t.Fatal("expected a rejection for a lastCorrectUpdate that is not an ISO YYYY-MM-DD date")
	}
}

func TestValidateArtifact_RejectsASuccessorOnANonDiscontinuedState(t *testing.T) {
	a := validArtifact()
	successor := "ocupados-epa"
	a.Series[0].PageState = publishing.PageStateRef{Kind: publishing.PageStateFresh, SuccessorSlug: &successor}
	err := publishing.ValidateArtifact(a)
	if err == nil {
		t.Fatal("expected a rejection for a successorSlug on a fresh page")
	}
	if !strings.Contains(err.Error(), "page-state") {
		t.Fatalf("expected the error to name the page-state constraint, got: %v", err)
	}
}

func TestValidateArtifact_AcceptsAValidationFailureWithNoPriorSuccess(t *testing.T) {
	a := validArtifact()
	a.Series[0].PageState = publishing.PageStateRef{Kind: publishing.PageStateValidationFailure}
	if err := publishing.ValidateArtifact(a); err != nil {
		t.Fatalf("expected a validation failure with no prior success to be a valid artifact, got: %v", err)
	}
}
