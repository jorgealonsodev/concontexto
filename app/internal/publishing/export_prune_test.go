package publishing_test

// The stale-file defect, closed here, was observed on the DEPLOYED stack,
// not inferred from reading the code. `ocupados-epa` was blocked by
// rule3-plausibility, so ListPublishedObservations returned nothing for it
// and Export skipped it entirely (export.go's `if len(obs) == 0 {
// continue }`). The manifest that resulted declared nine series and nine
// digests, none of them `ocupados-epa` -- and yet
// /data-derived/csv/ocupados-epa.csv and
// /data-derived/series/ocupados-epa.json both still answered 200, serving
// three quarters of data left over from the image's own seed of
// /web/dist/data-derived.
//
// Export wrote the series it produced and removed nothing, so a series
// that STOPS being published leaves reader-facing data at a URL the
// indicator page still links to: carrying no digest, absent from the
// manifest, and indistinguishable to a reader from current data. That is
// principle P4 inverted -- data served with no provenance, presented as
// current -- which is why the fix belongs inside Export rather than in an
// optional cleanup step a caller could forget to compose (the same
// reasoning Deps.SeriesValidationOutcome's doc comment applies to a port
// whose absence used to be absorbed silently).
//
// Every case below drives the REAL Export against a REAL directory: the
// defect is a filesystem fact, and a fake writer could not have exhibited
// it.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// prunableDeps builds a Deps publishing exactly the named slugs, each with
// one identical observation. The slugs are the only variable that matters
// to a pruning test, so everything else is held constant and uninteresting.
func prunableDeps(t *testing.T, slugs ...string) publishing.Deps {
	t.Helper()
	series := make([]postgres.PublishedSeries, 0, len(slugs))
	obs := map[string][]postgres.PublishedObservation{}
	for _, slug := range slugs {
		ps := fakePublishedSeries()
		ps.Slug = slug
		ps.Name = slug
		series = append(series, ps)
		obs[slug] = []postgres.PublishedObservation{{
			Period: "2026-Q1", Value: ptrf(10.5), Status: postgres.StatusDefinitive, Version: 1,
			IngestionRunID: 1, ExtractedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			RawFileSHA256: strings.Repeat("a", 64), RequestURL: "https://ine.es/data",
		}}
	}
	return fakeDeps(t, series, obs, freshness.StateFresh)
}

// artifactFiles lists the files Export owns under outDir, as the
// manifest-relative paths the manifest's own digest keys use ("series/
// {slug}.json"), so a test can compare a directory against a manifest
// without restating either side's layout.
func artifactFiles(t *testing.T, outDir string) []string {
	t.Helper()
	var out []string
	for _, sub := range []struct{ dir, ext string }{{"series", ".json"}, {"csv", ".csv"}} {
		entries, err := os.ReadDir(filepath.Join(outDir, sub.dir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("reading %s: %v", sub.dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), sub.ext) {
				continue
			}
			out = append(out, sub.dir+"/"+e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func readManifestFile(t *testing.T, outDir string) publishing.Manifest {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("reading manifest.json: %v", err)
	}
	var m publishing.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshalling manifest.json: %v", err)
	}
	return m
}

// TestExport_ASeriesThatDropsOutOfALaterExportLeavesNothingBehind
// reproduces the deployed-stack observation end to end: six series
// exported, then five (ocupados-epa blocked by validation, so it no longer
// resolves any observation). Both of its files must be gone, and the
// directory must describe EXACTLY what the manifest declares -- the whole
// invariant, asserted as one comparison rather than as two hand-listed
// filenames, so a future writer that adds a third per-series file cannot
// satisfy this test while leaving that third file stale.
func TestExport_ASeriesThatDropsOutOfALaterExportLeavesNothingBehind(t *testing.T) {
	outDir := t.TempDir()
	ctx := context.Background()
	asOf := time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC)

	six := []string{"afiliacion-ss", "ipc-general", "ocupados-epa", "pib-cvi", "poblacion-residente", "tasa-de-paro-epa"}
	if _, err := publishing.Export(ctx, prunableDeps(t, six...), asOf, outDir); err != nil {
		t.Fatalf("first Export: %v", err)
	}
	if got := len(artifactFiles(t, outDir)); got != 2*len(six) {
		t.Fatalf("expected the first export to write one JSON and one CSV per series (%d files), got %d", 2*len(six), got)
	}

	five := []string{"afiliacion-ss", "ipc-general", "pib-cvi", "poblacion-residente", "tasa-de-paro-epa"}
	artifact, err := publishing.Export(ctx, prunableDeps(t, five...), asOf.Add(time.Hour), outDir)
	if err != nil {
		t.Fatalf("second Export: %v", err)
	}

	for _, path := range []string{"series/ocupados-epa.json", "csv/ocupados-epa.csv"} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be gone once the series stopped being published (stat err = %v)", path, err)
		}
	}

	// The invariant itself: nothing extra, nothing stale.
	manifest := readManifestFile(t, outDir)
	var declared []string
	for _, slug := range manifest.Series {
		declared = append(declared, "csv/"+slug+".csv", "series/"+slug+".json")
	}
	sort.Strings(declared)
	if strings.Join(artifactFiles(t, outDir), " ") != strings.Join(declared, " ") {
		t.Fatalf("the directory must describe exactly what the manifest declares\n on disk: %v\ndeclared: %v",
			artifactFiles(t, outDir), declared)
	}

	// A removal is reader-facing data disappearing, so it is reported, not
	// performed silently (the same convention the reconcile's inserted/
	// updated/retired counts already establish).
	want := []string{"csv/ocupados-epa.csv", "series/ocupados-epa.json"}
	got := append([]string(nil), artifact.Prune.Removed...)
	sort.Strings(got)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("expected the returned artifact to report %v as removed, got %v", want, artifact.Prune.Removed)
	}
	if artifact.Prune.Skipped {
		t.Fatal("expected the prune guard NOT to have refused: this export declared five series")
	}
}

// TestExport_LeavesEveryFileItDoesNotOwnUntouched is the other half of the
// contract, and the reason the removal is scoped to two directories and two
// file shapes instead of to outDir as a whole. outDir is a SERVED static
// root mounted as a volume in production, and other runtime paths already
// live beside it under /web/dist (see deployedManifestPath in
// app/cmd/concontexto/schedule.go for a file deliberately kept OUTSIDE
// dist for exactly this class of reason). Export may only delete the exact
// file shapes it itself produces, in the exact subdirectories it owns.
//
// The leftover ".tmp-*" file is deliberately included: writeFileAtomic
// creates one in the destination directory on every write, so a
// CONCURRENT export's in-flight temp file could be sitting there. Deleting
// it would corrupt that other writer's rename.
func TestExport_LeavesEveryFileItDoesNotOwnUntouched(t *testing.T) {
	outDir := t.TempDir()

	foreign := map[string]string{
		"robots.txt":              "User-agent: *\n",
		"series/README.md":        "not an export output\n",
		"series/.tmp-inflight":    "another writer's in-flight temp file\n",
		"csv/NOTES":               "no extension, not ours\n",
		"other/stale.json":        "a JSON file outside the two directories Export owns\n",
		"series/nested/deep.json": "inside a subdirectory Export never creates\n",
	}
	for rel, content := range foreign {
		path := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("seeding %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("seeding %s: %v", rel, err)
		}
	}

	artifact, err := publishing.Export(context.Background(), prunableDeps(t, "tasa-de-paro-epa"), time.Now(), outDir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	for rel, content := range foreign {
		got, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("expected %s to survive the export untouched: %v", rel, err)
		}
		if string(got) != content {
			t.Fatalf("expected %s to be byte-identical, got %q", rel, got)
		}
	}
	if len(artifact.Prune.Removed) != 0 {
		t.Fatalf("expected nothing to be reported as removed, got %v", artifact.Prune.Removed)
	}
}

// TestExport_PrunesOnlyAfterTheManifestIsInPlace guards the ORDERING --
// the single most-reasoned decision in this change, and until this test
// existed the only one nothing checked: moving pruneUnpublishedFiles above
// the series/CSV writes left the whole publishing package green.
//
// The property under test is not "the prune runs at some point"; it is
// "nothing is removed until the manifest that justifies the removal is
// actually on disk". A prune that ran first would delete a series' files
// and then, if the manifest write failed, leave the PREVIOUS manifest
// standing -- a manifest declaring a series whose files are gone, which is
// a 404 on a declared path and a digest no reader can ever verify. That is
// the strictly worse of the two failure modes Export's doc comment weighs.
//
// The discriminator is a manifest.json that is a DIRECTORY. Every
// series/{slug}.json and csv/{slug}.csv write still succeeds; only
// writeFileAtomic's final rename onto manifest.json fails (EISDIR). No
// read-only directory is involved -- that would be the setup for the
// unlink-failure path, which is a different claim about a different line,
// and it would block the writes that must succeed for this test to
// discriminate at all.
//
// The second export publishes a slug the first one did not, so the
// assertion that the writes succeeded rests on a file that could only have
// come from this run.
func TestExport_PrunesOnlyAfterTheManifestIsInPlace(t *testing.T) {
	outDir := t.TempDir()
	ctx := context.Background()
	asOf := time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC)

	if _, err := publishing.Export(ctx, prunableDeps(t, "ipc-general", "ocupados-epa"), asOf, outDir); err != nil {
		t.Fatalf("seeding the published artifact: %v", err)
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("removing the seeded manifest: %v", err)
	}
	if err := os.Mkdir(manifestPath, 0o755); err != nil {
		t.Fatalf("replacing manifest.json with a directory: %v", err)
	}

	if _, err := publishing.Export(ctx, prunableDeps(t, "ipc-general", "pib-cvi"), asOf.Add(time.Hour), outDir); err == nil {
		t.Fatal("expected the export to fail: manifest.json is a directory, so no rename can land on it")
	}

	// Proves the failure is the MANIFEST's and not an earlier one: without
	// this, an Export that aborted before writing anything would satisfy
	// the assertion below for entirely the wrong reason.
	if _, err := os.Stat(filepath.Join(outDir, "series", "pib-cvi.json")); err != nil {
		t.Fatalf("expected every write preceding the manifest to have succeeded, so the manifest is the only thing that failed: %v", err)
	}

	for _, path := range []string{"series/ocupados-epa.json", "csv/ocupados-epa.csv"} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(path))); err != nil {
			t.Fatalf("PRUNE RAN BEFORE THE MANIFEST: %s was removed by an export whose manifest never landed, so the manifest on disk now declares a series with no files: %v", path, err)
		}
	}
}

// TestExport_RefusesToPruneWhenTheExportDeclaresNoSeries is the guard
// against turning a stale-file bug into DATA LOSS.
//
// The pruning step derives "what should exist" from what this one export
// produced. If a read-side bug, a truncated connection or an empty
// database ever made Export produce zero series documents, an unguarded
// prune would delete the ENTIRE published artifact -- every series, both
// projections -- and the served site would go from "one stale series" to
// "nothing at all", with no route back except a full re-ingest.
//
// Zero documents is the one failure of that family that is both
// DETECTABLE from inside Export and CATASTROPHIC, so it is the one that is
// refused. A PARTIAL list (five of nine, say) is indistinguishable from a
// legitimate retirement of four series and is not guessed at here. What
// keeps THAT case safe is the read side -- a series with published history
// cannot silently drop out of it -- and not, as this comment once claimed,
// the ingest gate or artifact retention: the gate is batch-scoped and
// blind to a degraded read, and retention's snapshot is taken after the
// prune. pruneUnpublishedFiles' own doc comment carries the full
// correction.
//
// The refusal deliberately leaves the directory holding MORE than the
// manifest declares -- the very state this change exists to end -- and
// reports that it did. A momentarily stale file is recoverable on the next
// good export; a deleted artifact is not.
func TestExport_RefusesToPruneWhenTheExportDeclaresNoSeries(t *testing.T) {
	outDir := t.TempDir()
	ctx := context.Background()

	if _, err := publishing.Export(ctx, prunableDeps(t, "ipc-general", "tasa-de-paro-epa"), time.Now(), outDir); err != nil {
		t.Fatalf("seeding the published artifact: %v", err)
	}
	before := artifactFiles(t, outDir)

	artifact, err := publishing.Export(ctx, prunableDeps(t), time.Now(), outDir)
	if err != nil {
		t.Fatalf("Export with no published series: %v", err)
	}
	if len(artifact.Series) != 0 {
		t.Fatalf("expected an export declaring no series, got %d", len(artifact.Series))
	}

	if after := artifactFiles(t, outDir); strings.Join(after, " ") != strings.Join(before, " ") {
		t.Fatalf("expected an empty export to delete NOTHING\n before: %v\n  after: %v", before, after)
	}
	if !artifact.Prune.Skipped {
		t.Fatal("expected the refusal to be reported as Prune.Skipped -- a guard that fires in silence cannot be told from one that never fired")
	}
	if len(artifact.Prune.Removed) != 0 {
		t.Fatalf("expected nothing removed, got %v", artifact.Prune.Removed)
	}
}
