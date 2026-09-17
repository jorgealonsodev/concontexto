// Package probe resolves and classifies the synthetic daily probe's
// results (see targets.go's own package doc for the probe's full spec
// citation). outcome.go is the OTHER pure/offline half, alongside
// targets.go: no build tag, so it compiles and runs in the default
// `go test ./...` suite exactly like targets.go does, and it never opens
// a socket -- it only classifies an error the live-tagged suite already
// produced.
//
// odd/tasks/probe-transport-vs-churn.md's verified problem: the probe
// could not tell "the source never answered" (INE refusing connections
// from a GitHub-hosted runner while Eurostat passed fine on the SAME
// egress -- no identifier churned) from "the source answered and the
// answer was wrong" (a genuinely churned identifier or a renamed
// response field), and printed the identical "possible identifier churn"
// message for both. That muted a daily alarm for a non-problem, which is
// exactly how the real churn it exists to catch would go unnoticed.
package probe

import (
	"errors"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// Outcome is what a single probe target's fetch-and-decode attempt
// resolved to, once Classify has looked at whatever error (if any) that
// attempt produced.
type Outcome string

const (
	// OutcomeHealthy means the target responded and decoded with no
	// error at all.
	OutcomeHealthy Outcome = "healthy"
	// OutcomeUnreachable means the source never answered this attempt --
	// a network error, a request timeout, or a 5xx -- exactly
	// sourceerr.RetryableTransport's definition. This is a transport
	// problem, not identifier churn, and the caller (live_test.go) skips
	// rather than fails on it.
	OutcomeUnreachable Outcome = "unreachable"
	// OutcomeBroken means the source DID answer and the answer itself is
	// what is wrong: a refused request (SourceRefusal), a structurally
	// valid but empty payload (SilentEmpty), a renamed field or
	// periodicity mismatch (SchemaDrift), an oversized response
	// (ResponseTooLarge), or any error this package does not recognise
	// at all. The caller fails the probe on this outcome.
	OutcomeBroken Outcome = "broken"
)

// Classify resolves err into an Outcome. It defers entirely to
// sourceerr.FailureClass.Retryable() -- the key insight recorded in the
// task doc: ingestion/sourceerr ALREADY encodes the transport-vs-churn
// distinction this probe needs, so Classify asks the taxonomy's own
// question ("may retrying ever help?") instead of re-deriving a second
// opinion that could drift from it. It deliberately does not compare
// against sourceerr.RetryableTransport directly for the same reason:
// Retryable() is the one place that already owns "which classes are
// transport-only", and asking the same question a second way here is
// exactly how the two would eventually disagree.
//
// err is unwrapped with errors.As, not a type assertion, because every
// realistic caller (fetchWithRetry's own callers, this package's
// live_test.go) adds request context with fmt.Errorf("...: %w", err)
// before Classify ever sees it; a bare type assertion would silently
// misclassify every wrapped error as unclassified.
//
// Any error that is not a *sourceerr.Error at all -- a bug in a future
// adapter, a panic recovered into a plain error, anything this package
// does not recognise -- resolves to OutcomeBroken, not OutcomeUnreachable
// and not a silent pass. The probe exists to catch identifier churn, and
// an error class it cannot name is exactly the case where staying quiet
// would recreate the same muted-alarm failure mode this whole change
// fixes, just one layer down: fail closed, never fail silent.
func Classify(err error) Outcome {
	if err == nil {
		return OutcomeHealthy
	}

	var classified *sourceerr.Error
	if errors.As(err, &classified) {
		if classified.Class.Retryable() {
			return OutcomeUnreachable
		}
		return OutcomeBroken
	}

	return OutcomeBroken
}
