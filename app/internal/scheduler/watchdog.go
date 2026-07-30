package scheduler

// Task 4.11/4.12: the publish-latency watchdog (spec pipeline-operations,
// "An ingestion not followed by a rebuild alerts operators"; design D-2,
// "a scheduler watchdog compares the live manifest's generated_at against
// the newest local export and alerts on budget breach"). This file owns
// the pure DECISION only -- the wiring that calls it once per scheduler
// tick, reads the local manifest.json and raises alerting.PublishLatencyBreach
// lives in app/cmd/concontexto/schedule.go (mirroring how this package's
// own Runner.Run owns the freshness/incident DECISION while
// app/cmd/concontexto composes the real clock, alert sink and driving
// loop around it).

import "time"

// DefaultPublishLatencyBudget is the fallback publish-latency budget (spec
// pipeline-operations: "MUST default to 30 minutes"; design D-2: "30
// minutes from ingest success to deployed page -- well inside §19.3's 24h;
// covers CI queue + build + Portainer pull").
const DefaultPublishLatencyBudget = 30 * time.Minute

// PublishLatencyBreached reports whether a source's most recent successful
// ingestion has not yet been reflected in its exported artifact after the
// configured budget elapsed. now, lastIngestionSuccess and
// manifestGeneratedAt are all explicit parameters -- this function never
// reads the system clock itself, matching freshness.Resolve and Run's own
// established convention (see this package's own doc comment: "time is
// always an explicit parameter").
//
// manifestGeneratedAt is the local export's own manifest.json generated_at
// (publishing.Manifest.GeneratedAt) -- the signal this codebase can
// observe for "did the rebuild follow" without needing a live deployed URL
// (PORTAINER_WEBHOOK_URL/VPS provisioning is a separate, disclosed
// dependency -- design.md's own Open Questions -- that blocks end-to-end
// DEPLOY verification only, never this budget check). A manifest generated
// at or after lastIngestionSuccess means the export step already picked up
// this ingestion cycle (and, when a rebuild dispatcher is configured,
// already triggered the rebuild for it -- Publish always dispatches
// synchronously within the same call as a successful Export, see
// trigger.go); one still generated strictly before it means the export --
// and therefore the rebuild it triggers -- has not run for this success
// yet.
//
// A zero-value lastIngestionSuccess (no known success at all) is the
// caller's own concern: this function only compares two already-supplied
// instants and never decides whether either is meaningful.
func PublishLatencyBreached(now, lastIngestionSuccess, manifestGeneratedAt time.Time, budget time.Duration) (breached bool, elapsed time.Duration) {
	elapsed = now.Sub(lastIngestionSuccess)
	if elapsed < budget {
		return false, elapsed
	}
	if !manifestGeneratedAt.Before(lastIngestionSuccess) {
		return false, elapsed
	}
	return true, elapsed
}
