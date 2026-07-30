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
// Publish then simply skips dispatch, matching the "not configured, not
// attempted" convention APIConfig/other optional wiring already
// establishes elsewhere in this codebase. historyDir is the same: empty
// skips retention entirely rather than archiving into some invented
// default location.
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
