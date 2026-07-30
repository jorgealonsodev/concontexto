package publishing_test

// Task 4.11/4.12 (RED): ReadManifest is the read-side counterpart to
// Export's own manifest write -- the scheduler's publish-latency watchdog
// (app/cmd/concontexto/schedule.go) uses it to read the newest LOCAL
// export's generated_at, without needing a live deployed URL.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

func TestReadManifest_RoundTripsWhatExportWrote(t *testing.T) {
	deps := onePublishedSeriesDeps(t)
	outDir := t.TempDir()
	asOf := time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC)

	if _, err := publishing.Export(t.Context(), deps, asOf, outDir); err != nil {
		t.Fatalf("Export: %v", err)
	}

	m, err := publishing.ReadManifest(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if !m.GeneratedAt.Equal(asOf) {
		t.Errorf("expected GeneratedAt %v, got %v", asOf, m.GeneratedAt)
	}
	if m.SchemaVersion != publishing.SchemaVersion {
		t.Errorf("expected schema_version %d, got %d", publishing.SchemaVersion, m.SchemaVersion)
	}
}

func TestReadManifest_AMissingFileIsAnError(t *testing.T) {
	if _, err := publishing.ReadManifest(filepath.Join(t.TempDir(), "manifest.json")); err == nil {
		t.Fatal("expected an error reading a manifest that was never written")
	}
}
