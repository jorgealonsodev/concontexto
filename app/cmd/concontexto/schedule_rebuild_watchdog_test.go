package main

// CRITICAL-28 (RED), link 3: "No deploy-completed instant exists anywhere."
//
// publishLatencyWatchdog compares a source's last ingestion success against
// the LOCAL manifest that publishing.Publish wrote seconds earlier in the
// same call, so the only failure it can ever catch is an export that did
// not run. A dispatch never sent, a rebuild that failed and a deploy that
// never landed are all invisible to it -- which is exactly the state the
// live stack was in: /data-derived moved every cycle while the pre-rendered
// pages stayed frozen at the image build, silently.
//
// These tests pin the second comparison that closes those links: the live
// artifact's generated_at against the generated_at of the artifact the
// DEPLOYED pages were built from (the image's own build manifest, written
// by the Dockerfile at /web/build-manifest.json, overridable with
// APP_BUILD_MANIFEST). They also pin the two ways this check must refuse to
// fire, because a watchdog that cries wolf is worse than none:
//
//   - when the deployed artifact is UNKNOWN (no build manifest, i.e. a bare
//     binary or an image built before this existed), and
//   - when rebuild dispatch is deliberately off, where divergence is the
//     operator's own stated intent rather than a fault.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// watchdogAlertSpy captures the alerts a watchdog tick raised.
type watchdogAlertSpy struct{ alerts []alerting.Alert }

func (s *watchdogAlertSpy) Alert(_ context.Context, a alerting.Alert) error {
	s.alerts = append(s.alerts, a)
	return nil
}

// writeManifestAt writes a minimal, real publishing.Manifest to path so the
// watchdog reads it back through the production publishing.ReadManifest
// rather than a hand-rolled stand-in.
func writeManifestAt(t *testing.T, path string, generatedAt time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(path), err)
	}
	b, err := json.Marshal(publishing.Manifest{
		SchemaVersion: 1,
		GeneratedAt:   generatedAt,
		Series:        []string{"tasa-de-paro-epa"},
		Digests:       map[string]string{"series/tasa-de-paro-epa.json": "abc"},
	})
	if err != nil {
		t.Fatalf("marshalling manifest: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// rebuildWatchdogFixture points STATIC_ROOT and APP_BUILD_MANIFEST at a
// temp tree and returns the two manifest paths the watchdog reads.
func rebuildWatchdogFixture(t *testing.T) (livePath, buildPath string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("STATIC_ROOT", filepath.Join(root, "dist"))
	buildPath = filepath.Join(root, "build-manifest.json")
	t.Setenv("APP_BUILD_MANIFEST", buildPath)
	return filepath.Join(root, "dist", "data-derived", "manifest.json"), buildPath
}

func TestPublishLatencyWatchdog_AlertsWhenTheDeployedPagesNeverCaughtUp(t *testing.T) {
	livePath, buildPath := rebuildWatchdogFixture(t)
	t.Setenv("APP_REBUILD_DISPATCH", "required")

	published := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	// The export ran and is current for this ingestion: the OLD watchdog
	// comparison is satisfied and raises nothing. What is stale is the
	// DEPLOY -- the pages were built from an artifact six hours older.
	writeManifestAt(t, livePath, published)
	writeManifestAt(t, buildPath, published.Add(-6*time.Hour))

	spy := &watchdogAlertSpy{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil)

	var logs strings.Builder
	watchdog := publishLatencyWatchdog(context.Background(), 30*time.Minute, &logs)
	watchdog("ine", published.Add(45*time.Minute), published)

	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly one alert for a rebuild that never landed, got %d: %+v", len(spy.alerts), spy.alerts)
	}
	if spy.alerts[0].Kind != alerting.KindPublishLatencyBreach {
		t.Errorf("expected KindPublishLatencyBreach, got %q", spy.alerts[0].Kind)
	}
	if spy.alerts[0].Source != "ine" {
		t.Errorf("the spec requires an alert to name the source, got %q", spy.alerts[0].Source)
	}
}

func TestPublishLatencyWatchdog_SilentWhenTheDeployedPagesCarryTheCurrentArtifact(t *testing.T) {
	livePath, buildPath := rebuildWatchdogFixture(t)
	t.Setenv("APP_REBUILD_DISPATCH", "required")

	published := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeManifestAt(t, livePath, published)
	writeManifestAt(t, buildPath, published)

	spy := &watchdogAlertSpy{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil)

	var logs strings.Builder
	watchdog := publishLatencyWatchdog(context.Background(), 30*time.Minute, &logs)
	watchdog("ine", published.Add(45*time.Minute), published)

	if len(spy.alerts) != 0 {
		t.Fatalf("a rebuild that landed must raise nothing, got %+v", spy.alerts)
	}
}

func TestPublishLatencyWatchdog_NeverInventsADeployBreachWhenTheDeployedArtifactIsUnknown(t *testing.T) {
	livePath, _ := rebuildWatchdogFixture(t)
	t.Setenv("APP_REBUILD_DISPATCH", "required")

	published := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeManifestAt(t, livePath, published)
	// No build manifest at all: a bare binary, or an image built before the
	// stamp existed. "Unknown" is not "stale", and a watchdog that reported
	// a deploy failure it cannot observe would be exactly the fabricated
	// green/red this remediation exists to avoid.

	spy := &watchdogAlertSpy{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil)

	var logs strings.Builder
	watchdog := publishLatencyWatchdog(context.Background(), 30*time.Minute, &logs)
	watchdog("ine", published.Add(45*time.Minute), published)

	if len(spy.alerts) != 0 {
		t.Fatalf("an unknown deployed artifact must never be reported as a stale deploy, got %+v", spy.alerts)
	}
}

func TestPublishLatencyWatchdog_SilentWhenRebuildDispatchIsDeliberatelyOff(t *testing.T) {
	livePath, buildPath := rebuildWatchdogFixture(t)
	t.Setenv("APP_REBUILD_DISPATCH", "off")

	published := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeManifestAt(t, livePath, published)
	writeManifestAt(t, buildPath, published.Add(-6*time.Hour))

	spy := &watchdogAlertSpy{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil)

	var logs strings.Builder
	watchdog := publishLatencyWatchdog(context.Background(), 30*time.Minute, &logs)
	watchdog("ine", published.Add(45*time.Minute), published)

	if len(spy.alerts) != 0 {
		t.Fatalf("with dispatch deliberately off, divergence is the operator's stated intent and must raise nothing, got %+v", spy.alerts)
	}
}

func TestPublishLatencyWatchdog_AStaleExportStillAlertsExactlyOnce(t *testing.T) {
	livePath, buildPath := rebuildWatchdogFixture(t)
	t.Setenv("APP_REBUILD_DISPATCH", "required")

	success := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	// The pre-existing link: an ingestion succeeded and the export is
	// older than it. Both comparisons would fire on this data, so this
	// also pins that they never double-page for one condition.
	writeManifestAt(t, livePath, success.Add(-2*time.Hour))
	writeManifestAt(t, buildPath, success.Add(-6*time.Hour))

	spy := &watchdogAlertSpy{}
	alerting.SetDefaultSink(spy)
	defer alerting.SetDefaultSink(nil)

	var logs strings.Builder
	watchdog := publishLatencyWatchdog(context.Background(), 30*time.Minute, &logs)
	watchdog("ine", success.Add(45*time.Minute), success)

	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly one alert for one stalled cycle, got %d: %+v", len(spy.alerts), spy.alerts)
	}
}
