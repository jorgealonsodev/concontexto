package ingestion_test

// verify-report CRITICAL-37: the end-to-end job that proves the Go->Astro
// hand-off ingested with `config.ValidationConfig{}` -- no thresholds at
// all -- so the guard that blocks a series in production was switched OFF
// in the one CI job that runs the whole chain. Nothing in the repository
// could have turned that job red for a validation failure, because the
// failure could not occur there.
//
// THIS IS THE FIFTH TIME A CHECK HAS BEEN FOUND WHERE ITS FAILURE CANNOT
// OCCUR, so the two questions worth writing down are: what would have to
// break for THIS test to go red, and can that thing actually break here?
//
//	1. `ineIngestConfig` stops carrying the real per-series `validation:`
//	   block from config/series/*.yaml. Then the COVID quarter publishes,
//	   `ocupados-epa` lands in the artifact, and the absence assertion
//	   below fails. This is exactly the state the repository was in before
//	   this file existed, so it is not hypothetical -- it is a revert.
//	2. `config/series/ocupados-epa.yaml` raises `max_delta_abs` above
//	   1074.1, which is the "blind the guard permanently" remedy
//	   acknowledgement_e2e_test.go's header rejects. Same failure.
//	3. `publishing.Export` stops skipping a series with zero observations
//	   (`if len(obs) == 0 { continue }`), so a blocked series would be
//	   exported as an empty chart. Same failure.
//
// All three are ordinary edits somebody could make next week, and each
// one is caught here rather than in production. What this file does NOT
// assert is the last link -- that `astro build` then exits non-zero on the
// artifact it writes. That is a Node process and cannot run inside `go
// test`; scripts/assert-blocked-series-fails-build.sh owns it, driven by
// the artifact this test exports to E2E_BLOCKED_EXPORT_DIR.
//
// WHY THE BLOCKED ARM AND NOT A "MUST ALL SUCCEED" ARM. The other
// defensible shape was to ingest with the real thresholds over the COVID
// range and require the whole chain to SUCCEED, letting CI go red the day
// a threshold blocks something. That job would be red TODAY and would stay
// red until a human signs config/reconocimientos.yaml -- CI would be
// reporting a pending editorial decision, not a fact about the code, and a
// permanently-red job is an ignored job. So the successful chain keeps
// running over the recorded last-3-period fixtures (e2e_export_test.go,
// now with the real thresholds applied), and the FAILURE path is proven
// here, where it is a genuine, reproducible, green-today assertion.
//
// The data is real INE data, not synthesised: testdata/datos_serie/
// ocupados-epa-covid.json is the recorded EPA387796 response trimmed to
// 2019-Q4/2020-Q1/2020-Q2 (see that directory's source.txt), the quarters
// INE's own press release of 28 July 2020 reports as a fall of 1,074,000
// employed persons.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// blockedSlug is the series the real shipped thresholds really do block on
// a real ingest today. Not an arbitrary pick and not a synthetic one: it
// is the portal's actual live problem, and the reason
// /indicador/ocupados-epa/ cannot be built from a real export right now.
const blockedSlug = "ocupados-epa"

// blockedPeriod is the quarter that breaches. Named because the assertion
// below requires the finding to be about THIS period -- a rule3 finding
// about some other period would mean the fixture, not the COVID collapse,
// is what tripped the guard.
const blockedPeriod = "2020-Q2"

// TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact ingests all six
// frozen slugs with the REAL per-series thresholds and the COVID quarter
// inside `ocupados-epa`'s range, then exports. The five unaffected series
// publish; `ocupados-epa` is blocked by rule3-plausibility and is
// therefore absent from the artifact, which is precisely the input the
// frozen-route guard (web/src/lib/indicator/routes.ts) must refuse to
// build from.
func TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	var blockedResult ingestion.Result
	for _, sc := range sixSeries {
		cod, fixture := loadFixture(t, sc.slug)
		if sc.slug == blockedSlug {
			// The SAME series, the SAME recorded response shape, a
			// different three quarters. Nothing else about the ingest
			// changes -- in particular the thresholds are the shipped
			// ones, not a stricter variant chosen to force this outcome.
			cod, fixture = ocupadosCovidFixture(t)
		}
		seedDimensions(t, ctx, tx, sc, cod)

		server := serveFixture(fixture)
		result, err := ingestion.IngestSeries(ctx, tx, store,
			ine.NewClient(server.URL, server.Client()), ineIngestConfig(t, sc, cod), now)
		server.Close()
		if err != nil {
			t.Fatalf("IngestSeries(%s): %v", sc.slug, err)
		}

		if sc.slug == blockedSlug {
			blockedResult = result
			continue
		}
		if result.Outcome != validation.GatePublish {
			t.Fatalf("expected %s to publish under its real shipped thresholds, got outcome=%v findings=%+v",
				sc.slug, result.Outcome, result.Findings)
		}
	}

	// THE ASSERTION THAT CANNOT PASS WITH VALIDATION SWITCHED OFF. With
	// `config.ValidationConfig{}` this series publishes and every line
	// below fails.
	if blockedResult.Outcome != validation.GateBlock {
		t.Fatalf("expected %s to be BLOCKED by its real shipped thresholds over the COVID quarter, got outcome=%v findings=%+v\n"+
			"This is the production state: a fall of 1074.1 at %s against config/series/%s.yaml's max_delta_abs of 1000.\n"+
			"A publish here means the end-to-end path is ingesting without the thresholds the pipeline really applies.",
			blockedSlug, blockedResult.Outcome, blockedResult.Findings, blockedPeriod, blockedSlug)
	}
	if len(blockedResult.Published) != 0 {
		t.Fatalf("a blocked run must write nothing, got %d observations", len(blockedResult.Published))
	}
	// WHICH rule, and about WHICH period. A block for any other reason --
	// a malformed fixture, a continuity gap, a decode failure -- would
	// satisfy the outcome check above while proving nothing about the
	// plausibility threshold this test exists to exercise.
	var sawPlausibilityAtCovid bool
	for _, f := range blockedResult.Findings {
		if f.Rule == "rule3-plausibility" && f.Period == blockedPeriod && f.Severity == validation.SeverityBlock {
			sawPlausibilityAtCovid = true
		}
	}
	if !sawPlausibilityAtCovid {
		t.Fatalf("expected a blocking rule3-plausibility finding at %s, got %+v", blockedPeriod, blockedResult.Findings)
	}

	outDir := blockedExportOutputDir(t)
	artifact, err := publishing.Export(ctx, exportDeps(tx), now, outDir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// The artifact must be COMPLETE apart from the blocked slug. Without
	// this half, an export that wrote nothing at all would also satisfy
	// "ocupados-epa is absent", and the build failing downstream would
	// then prove only that an empty directory is not a site.
	exported := map[string]bool{}
	for _, doc := range artifact.Series {
		exported[doc.Slug] = true
	}
	for _, sc := range sixSeries {
		_, statErr := os.Stat(filepath.Join(outDir, "series", sc.slug+".json"))
		if sc.slug == blockedSlug {
			if exported[sc.slug] {
				t.Errorf("a blocked series must not reach the export artifact, but %q is in it", sc.slug)
			}
			if statErr == nil {
				t.Errorf("a blocked series must not reach the artifact on disk, but series/%s.json exists", sc.slug)
			}
			continue
		}
		if !exported[sc.slug] {
			t.Errorf("expected the unaffected slug %q in the artifact; without it the build downstream would fail for the wrong reason", sc.slug)
		}
		if statErr != nil {
			t.Errorf("expected series/%s.json on disk: %v", sc.slug, statErr)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json on disk -- the loader reads it first, and its absence would fail the build for the wrong reason: %v", err)
	}
}

// blockedExportOutputDir resolves where the BLOCKED artifact is written,
// with exactly the semantics exportOutputDir has for the honest one (see
// its doc comment): unset means a discarded t.TempDir(), set means the
// bytes survive this process so the CI step that follows can hand them to
// a real `astro build`.
//
// A separate variable from E2E_EXPORT_DIR, not a reused one: the CI job
// produces BOTH artifacts in a single `go test` invocation and hands them
// to two different assertions, one requiring a green build and one
// requiring a red one. Sharing a variable would let the second export
// overwrite the first and make the pair meaningless.
func blockedExportOutputDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("E2E_BLOCKED_EXPORT_DIR")
	if dir == "" {
		return t.TempDir()
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("E2E_BLOCKED_EXPORT_DIR=%q is not resolvable to an absolute path: %v", dir, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatalf("creating E2E_BLOCKED_EXPORT_DIR %q: %v", abs, err)
	}
	return abs
}
