package publishing

// ReadManifest is the read-side counterpart to Export's own manifest write
// (writeFileAtomic, export.go). Task 4.11/4.12: the scheduler's
// publish-latency watchdog (app/cmd/concontexto/schedule.go) reads the
// newest LOCAL export's manifest.json with this function to compare its
// generated_at against a source's most recent successful ingestion --
// see app/internal/scheduler.PublishLatencyBreached for the decision this
// feeds.

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadManifest reads and parses the manifest.json at path.
func ReadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("publishing: reading manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("publishing: parsing manifest %s: %w", path, err)
	}
	return m, nil
}
