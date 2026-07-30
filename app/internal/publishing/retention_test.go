package publishing_test

// Task 4.9 (RED): artifact retention (design's own Migration/Rollout note,
// "last N artifacts retained, N configurable >= 5"). ArchiveArtifact
// copies the just-written export at outDir into a new, timestamped
// snapshot under historyDir and prunes historyDir down to the newest
// `retain` snapshots -- the live outDir itself is never a pruning
// candidate, by construction (it lives outside historyDir entirely).

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func writeFakeArtifact(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "series"), 0o755); err != nil {
		t.Fatalf("seeding fake artifact dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"schema_version":`+content+`}`), 0o644); err != nil {
		t.Fatalf("seeding fake manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "series", "tasa-de-paro-epa.json"), []byte(`{"slug":"tasa-de-paro-epa"}`), 0o644); err != nil {
		t.Fatalf("seeding fake series doc: %v", err)
	}
}

func TestArchiveArtifact_ARetainedSnapshotReproducesTheArtifactByteIdentical(t *testing.T) {
	outDir := t.TempDir()
	historyDir := t.TempDir()
	writeFakeArtifact(t, outDir, "1")

	generatedAt := time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC)
	if err := publishing.ArchiveArtifact(outDir, historyDir, generatedAt, 5); err != nil {
		t.Fatalf("ArchiveArtifact: %v", err)
	}

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatalf("reading historyDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 snapshot, got %d", len(entries))
	}
	snapshot := filepath.Join(historyDir, entries[0].Name())

	original, err := os.ReadFile(filepath.Join(outDir, "series", "tasa-de-paro-epa.json"))
	if err != nil {
		t.Fatalf("reading original series doc: %v", err)
	}
	rebuilt, err := os.ReadFile(filepath.Join(snapshot, "series", "tasa-de-paro-epa.json"))
	if err != nil {
		t.Fatalf("reading retained series doc: %v", err)
	}
	if string(original) != string(rebuilt) {
		t.Errorf("expected the retained snapshot to reproduce the artifact byte-identical, got %q want %q", rebuilt, original)
	}
}

func TestArchiveArtifact_KeepsOnlyTheNewestRetainSnapshots(t *testing.T) {
	outDir := t.TempDir()
	historyDir := t.TempDir()

	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	const retain = 5
	const runs = retain + 3
	for i := 0; i < runs; i++ {
		writeFakeArtifact(t, outDir, "1")
		if err := publishing.ArchiveArtifact(outDir, historyDir, base.Add(time.Duration(i)*time.Hour), retain); err != nil {
			t.Fatalf("ArchiveArtifact run %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatalf("reading historyDir: %v", err)
	}
	if len(entries) != retain {
		t.Fatalf("expected exactly %d retained snapshots after %d runs, got %d", retain, runs, len(entries))
	}

	// The retained snapshots must be the NEWEST ones -- the earliest 3 runs
	// (out of 8 total) must have been pruned.
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}
	oldestExpectedPrunedInstant := base.Add(2 * time.Hour).UTC().Format("20060102T150405.000000000Z")
	if names[oldestExpectedPrunedInstant] {
		t.Errorf("expected the third-oldest run to have been pruned, but its snapshot is still present")
	}
}

func TestArchiveArtifact_NeverTouchesTheLiveOutDir(t *testing.T) {
	outDir := t.TempDir()
	historyDir := t.TempDir()

	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		writeFakeArtifact(t, outDir, "1")
		if err := publishing.ArchiveArtifact(outDir, historyDir, base.Add(time.Duration(i)*time.Hour), 5); err != nil {
			t.Fatalf("ArchiveArtifact run %d: %v", i, err)
		}
	}

	// The live outDir is never inside historyDir and is never pruned --
	// its own files must still exist after every retention run above,
	// unaffected by however many snapshots were archived or pruned.
	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); err != nil {
		t.Errorf("expected the live outDir's manifest.json to still exist, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "series", "tasa-de-paro-epa.json")); err != nil {
		t.Errorf("expected the live outDir's series doc to still exist, got: %v", err)
	}
}

func TestArchiveArtifact_RetainFloorIsEnforcedEvenBelowFive(t *testing.T) {
	outDir := t.TempDir()
	historyDir := t.TempDir()

	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 7; i++ {
		writeFakeArtifact(t, outDir, "1")
		// retain=1 is below the design's own ">= 5" floor -- ArchiveArtifact
		// must enforce the floor itself, not merely trust a well-behaved
		// caller.
		if err := publishing.ArchiveArtifact(outDir, historyDir, base.Add(time.Duration(i)*time.Hour), 1); err != nil {
			t.Fatalf("ArchiveArtifact run %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatalf("reading historyDir: %v", err)
	}
	if len(entries) != publishing.DefaultRetainedArtifacts {
		t.Fatalf("expected the %d-artifact floor to be enforced regardless of a lower requested retain, got %d retained", publishing.DefaultRetainedArtifacts, len(entries))
	}
}
