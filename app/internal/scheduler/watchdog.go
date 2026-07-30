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

// RebuildLatencyBreached reports whether the artifact this container has
// published is still not reflected in the DEPLOYED pages after the budget
// elapsed (verify-report CRITICAL-28, link 3: "No deploy-completed instant
// exists anywhere").
//
// PublishLatencyBreached above compares an ingestion success against the
// LOCAL manifest that publishing.Publish wrote seconds earlier in the same
// call, so the only thing it can ever catch is an export that did not run.
// It is structurally blind to the three links that follow: a dispatch never
// sent, a rebuild that failed, and a deploy that never landed. This
// function closes those by comparing two instants that move independently:
//
//	artifactGeneratedAt  the live artifact's manifest.generated_at, rewritten
//	                     by every publish cycle into the export_artifact volume;
//	deployedGeneratedAt  the generated_at of the artifact the running image's
//	                     pre-rendered pages were BUILT from -- shipped in the
//	                     image as /web/build-manifest.json (Dockerfile) and
//	                     therefore changed by nothing except a deploy.
//
// A deploy-completed instant is exactly what deployedGeneratedAt is: the
// pages cannot advance without a new image, and a new image cannot arrive
// without the dispatch, the rebuild and the redeploy all having succeeded.
// So one comparison covers every remaining link at once, without this
// process needing to reach GitHub, Portainer or the public site.
//
// Divergence is NORMAL and is not itself the alarm: every publish cycle
// makes the live artifact newer than the deployed pages, and the whole
// point of the pipeline is that a rebuild then catches up. Only divergence
// that OUTLIVES the budget is a breach -- the same shape
// PublishLatencyBreached already uses, and the reason both take the one
// budget.
//
// A zero artifactGeneratedAt means nothing has ever been published here, so
// there is no rebuild to be waiting for; that case belongs to
// PublishLatencyBreached (a missing export) and returns false, 0 rather
// than measuring elapsed time from the zero instant. A zero
// deployedGeneratedAt is NOT handled here: "the image carries no build
// manifest" means the deployed artifact is unknown, not old, and the caller
// must decline to call this function at all rather than let an unknown
// masquerade as a stale deploy (see app/cmd/concontexto/schedule.go).
func RebuildLatencyBreached(now, artifactGeneratedAt, deployedGeneratedAt time.Time, budget time.Duration) (breached bool, elapsed time.Duration) {
	if artifactGeneratedAt.IsZero() {
		return false, 0
	}
	elapsed = now.Sub(artifactGeneratedAt)
	if elapsed < budget {
		return false, elapsed
	}
	if !deployedGeneratedAt.Before(artifactGeneratedAt) {
		return false, elapsed
	}
	return true, elapsed
}
