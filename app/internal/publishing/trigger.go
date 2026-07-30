package publishing

// Task 4.5/4.6/4.7/4.8: Publish wraps Export with rebuild dispatch and
// instant recording (design D-2: "Publish(ctx, deps, asOf) (Export +
// dispatch + latency stamp)"; spec pipeline-operations, "A successful
// ingestion produces an export and dispatches a rebuild"). It is the
// function app/cmd/concontexto/ingest_cmd.go calls once per ingest cycle
// in which at least one series published -- Export itself stays the pure
// read/validate/write primitive both this function and export_cmd.go's
// standalone `export` subcommand share.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
)

// Dispatcher POSTs the rebuild trigger (design D-2: "publishing.Dispatcher
// port; adapter adapters/github/ POSTs a repository_dispatch"). Function-
// typed, not an interface, matching Deps' own established convention --
// a test substitutes a closure with no mock framework and no network.
// generatedAt is the artifact's own manifest.generated_at; manifestDigest
// is manifestDigest(artifact.Manifest) below.
type Dispatcher func(ctx context.Context, generatedAt time.Time, manifestDigest string) error

// PublishResult is one Publish call's outcome: the exported Artifact plus
// the two instants spec pipeline-operations' "A publish cycle is measured
// end to end" scenario needs recorded (ingestion-success and dispatch;
// the third, deploy-completed, is only knowable once the triggered CI
// build finishes and is therefore the scheduler watchdog's own concern,
// not this function's -- see watchdog.go).
type PublishResult struct {
	Artifact Artifact

	// IngestionSuccessAt is asOf -- the instant Export built the artifact
	// from, which by design D-2's own convention IS the ingestion-success
	// instant (Publish is only ever called after a cycle that published).
	IngestionSuccessAt time.Time

	// DispatchedAt is nil when dispatch was skipped (dispatcher is nil)
	// or failed (see the dispatch-failure branch below) -- only a
	// genuinely successful dispatch sets it.
	DispatchedAt *time.Time

	// DispatchSkipped is true when no Dispatcher was supplied, i.e. the
	// operator never asked for one, so nothing was attempted. It is the
	// discriminator DispatchedAt alone cannot provide (verify-report
	// CRITICAL-28, link 2): a nil DispatchedAt means BOTH "not configured,
	// not attempted" and "attempted and failed", and those are opposite
	// operational facts -- the first is a deliberate configuration, the
	// second is a broken pipeline that has already raised
	// alerting.DispatchFailed. Callers log which of the three states a
	// cycle ended in; before this field existed, app/cmd/concontexto's
	// runIngest printed "exported and dispatched" for all three, including
	// the deployed-stack case in which nothing was ever dispatched.
	//
	// Deciding whether "not configured" is itself acceptable is the
	// COMPOSITION's job, not this function's: app/cmd/concontexto resolves
	// APP_REBUILD_DISPATCH and hands Publish either a real dispatcher, a
	// nil one (dispatch deliberately off) or one that fails immediately
	// naming the missing configuration (dispatch expected but unconfigured
	// -- which then travels the dispatch-failure branch below and alerts).
	// Publish itself stays a mechanism and never guesses intent.
	DispatchSkipped bool

	// ArchiveErr records a retention-archiving failure (task 4.9/4.10,
	// ArchiveArtifact in retention.go) -- best-effort, the same convention
	// dispatch failure already establishes: a retained-history gap is a
	// rollback convenience missed, never a reason to fail an otherwise-
	// successful publish cycle. nil means retention was either not
	// configured (historyDir == "") or succeeded.
	ArchiveErr error
}

// manifestDigest computes one stable digest over a manifest's own
// per-file digests -- a single value naming "this exact combination of
// series contents", not merely "this generation instant" (two runs at
// different asOf could otherwise dispatch with no way to tell whether
// anything actually changed). Sorted by path first so the digest is
// deterministic regardless of map iteration order.
func manifestDigest(m Manifest) string {
	paths := make([]string, 0, len(m.Digests))
	for p := range m.Digests {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(h, "%s=%s\n", p, m.Digests[p])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Publish exports the artifact (Export -- a validation failure aborts
// here exactly as it would for a bare Export call, and dispatch never
// runs), archives a retained snapshot of it (task 4.9/4.10, ArchiveArtifact
// in retention.go), and, when dispatch succeeds, hands the deploy pipeline
// a rebuild trigger. dispatch may be nil (design D-2's "recovery,
// boot-time self-heal, fixture generation" callers never need one) --
// Publish then skips dispatch and RECORDS that it did so on
// PublishResult.DispatchSkipped, so the caller can tell "deliberately off"
// from "attempted and failed". Whether nil is legitimate at all is the
// caller's decision and not Publish's: see DispatchSkipped's own comment
// and app/cmd/concontexto's buildDispatcher, which returns a dispatcher
// that fails immediately -- never nil -- when the deployment declares that
// a rebuild dispatch is expected. historyDir is unchanged: empty skips
// retention entirely rather than archiving into some invented default
// location.
//
// A dispatch failure is caught, alerted (alerting.DispatchFailed, design
// D-2: "Dispatch failure is an alert ..., never a retry loop and never
// an ingest failure -- data is safe, only publication latency suffers")
// and swallowed: Publish's own return value never carries it. This is
// the one function in this package that may legitimately hide an error
// from its caller, and it is documented here precisely because that is
// unusual for this codebase's own conventions. A retention failure is
// similarly best-effort but NOT hidden -- it is surfaced on
// PublishResult.ArchiveErr for the caller to log, since (unlike a
// transient dispatch failure) it may indicate a persistent, worth-fixing
// problem with historyDir itself.
func Publish(ctx context.Context, deps Deps, dispatch Dispatcher, asOf time.Time, outDir, historyDir string, retain int) (PublishResult, error) {
	artifact, err := Export(ctx, deps, asOf, outDir)
	if err != nil {
		return PublishResult{}, err
	}

	result := PublishResult{Artifact: artifact, IngestionSuccessAt: asOf}
	if historyDir != "" {
		result.ArchiveErr = ArchiveArtifact(outDir, historyDir, artifact.Manifest.GeneratedAt, retain)
	}
	if dispatch == nil {
		result.DispatchSkipped = true
		return result, nil
	}

	if err := dispatch(ctx, artifact.Manifest.GeneratedAt, manifestDigest(artifact.Manifest)); err != nil {
		// Best-effort, matching every other alert in this codebase: a
		// failed alert delivery must never fail (or re-fail) an
		// already-decided publish cycle.
		_ = alerting.DispatchFailed(ctx, alerting.DefaultSink(), artifact.Manifest.GeneratedAt, err)
		return result, nil
	}

	// dispatchedAt reuses asOf (via the artifact's own already-stamped
	// manifest.GeneratedAt) rather than reading the system clock again --
	// this codebase's established convention (freshness.Resolve,
	// scheduler.Runner.Run, ...) is an explicit, injected instant, never
	// time.Now() read inside the function. Dispatch runs synchronously
	// immediately after Export within this same call, so the two instants
	// coinciding is the honest, injected-clock-consistent answer, not an
	// approximation.
	dispatchedAt := artifact.Manifest.GeneratedAt
	result.DispatchedAt = &dispatchedAt
	return result, nil
}
