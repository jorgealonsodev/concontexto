// Package scheduler implements the per-source scheduled job (spec
// pipeline-operations, "One scheduled ingestion job per source"; PRD
// §9.2). It owns exactly one policy decision: given the outcome of one
// scheduled attempt against a source, is this incident-worthy, and when
// should the NEXT attempt be scheduled?
//
// It deliberately owns NOTHING about retryability itself. Every source
// adapter (ine, eurostat, xlsx) already classifies its own failures via
// sourceerr.FailureClass and already retries a RetryableTransport failure
// internally, with backoff, before ever returning an error to this
// package (app/internal/adapters/{ine,eurostat,xlsx}'s own
// fetchWithRetry). A named non-retryable failure (sourceerr.SourceRefusal
// -- the INE volume-restriction envelope; sourceerr.ResponseTooLarge --
// Eurostat's 157 MB oversized response) already returns after exactly one
// request from the adapter's own client. Run therefore calls its op
// callback EXACTLY ONCE per invocation and never re-decides which errors
// deserve a retry -- doing so would risk exactly the failure mode Finding
// A (Engram #4690) identified: retrying a permanent refusal forever.
// "Retries with increasing backoff" (spec "A transient failure is
// retried") is a property of REPEATED SCHEDULED INVOCATIONS of Run, not
// of a single call -- Attempt.NextAttemptAt is this package's own
// contribution: WHEN a driving loop (a cron tick, a daemon's ticker --
// not built in this batch) should call Run again.
//
// The 24h incident boundary is never re-derived here either: Run resolves
// it by calling freshness.Resolve/State.RaisesIncident (built in PR 5a-i,
// task 5a.7/5a.8, extended for Eurostat maintenance windows in PR 6b,
// task 6.12/6.13) -- the exact same decision every other part of the
// system already uses, so a source that has been "down" for the scheduler
// and a source that is "stale" for the amber semaphore are never two
// different facts computed two different ways.
package scheduler

import (
	"context"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
)

// Attempt is the outcome of one scheduled tick for one source.
type Attempt struct {
	SourceID string
	Err      error
	State    freshness.State

	// Incident mirrors State.RaisesIncident() -- kept as its own field so
	// a caller never has to re-derive it, matching the same convention
	// freshness.State.RaisesIncident() itself exists for (that method's
	// own doc comment: "one named, tested boundary to call instead of
	// re-deriving ... by hand").
	Incident bool

	// NextAttemptAt is when a driving loop should call Run again after a
	// failed attempt (spec "retries with increasing backoff"). It is the
	// zero time.Time on a successful attempt -- there is nothing to
	// retry.
	NextAttemptAt time.Time
}

// Runner is a stateful per-source scheduled job (spec "The scheduler MUST
// run one job per source"). It tracks consecutiveFailures across calls to
// Run so Backoff can increase between ticks without every caller having
// to thread that count through itself.
type Runner struct {
	SourceID string

	// Backoff computes the delay before the next scheduled attempt, given
	// how many attempts have failed in a row (1-indexed: the value passed
	// is the count INCLUDING the failure that just happened). Tests
	// inject a fast, deterministic function; production tuning of the
	// real backoff curve is deployment configuration, not built here.
	Backoff func(consecutiveFailures int) time.Duration

	// AlertSink receives a KindSourceDown alert (spec "MUST alert on ...
	// a source recorded as down") the moment an attempt's freshness state
	// raises an incident. nil is safe -- alerting.SourceDown treats a nil
	// Sink as alerting.NoopSink, so a Runner built with the zero value
	// (bar SourceID/Backoff) never panics.
	AlertSink alerting.Sink

	consecutiveFailures int
}

// NewRunner builds a Runner for sourceID with the default backoff curve
// (linear, one hour per consecutive failure -- generous enough that two
// failures within an hour never approach the 24h incident window on their
// own, matching the "two failures then success raises no incident"
// scenario) and alerting.DefaultSink() as its alert destination.
func NewRunner(sourceID string) *Runner {
	return &Runner{SourceID: sourceID, Backoff: defaultBackoff, AlertSink: alerting.DefaultSink()}
}

func defaultBackoff(consecutiveFailures int) time.Duration {
	return time.Duration(consecutiveFailures) * time.Hour
}

// Run executes op exactly once (op is a source's own fetch/decode
// operation -- see the package doc comment for why this package never
// wraps it in a second retry loop of its own). lastSuccess and now are
// both explicit parameters, never time.Now() read inside this function,
// matching freshness.Resolve's own contract (and the same pattern PR 4a
// established for the validation rules).
//
// On success, Run resets the consecutive-failure count and returns a
// fresh Attempt with no incident and no scheduled retry.
//
// On failure, Run resolves the source's freshness state as of now against
// lastSuccess. A state that raises an incident (more than 24h without a
// success, spec "Twenty-four hours of failure raises an incident")
// immediately alerts via AlertSink (task 9.7/9.8, wired here rather than
// left to a caller, because this is the exact point the incident fact is
// first known) and is reported on the returned Attempt; either way,
// NextAttemptAt tells a driving loop when to try again.
func (r *Runner) Run(ctx context.Context, lastSuccess *time.Time, now time.Time, op func(ctx context.Context) error) Attempt {
	if err := op(ctx); err != nil {
		r.consecutiveFailures++
		state := freshness.Resolve(lastSuccess, now)
		incident := state.RaisesIncident()
		if incident {
			// Best-effort: a failed alert delivery must never fail the
			// scheduled attempt itself -- the attempt's own Err/State/
			// Incident fields already carry the failure, independent of
			// whether the alert reached its sink.
			_ = alerting.SourceDown(ctx, r.AlertSink, r.SourceID)
		}
		return Attempt{
			SourceID:      r.SourceID,
			Err:           err,
			State:         state,
			Incident:      incident,
			NextAttemptAt: now.Add(r.Backoff(r.consecutiveFailures)),
		}
	}
	r.consecutiveFailures = 0
	return Attempt{SourceID: r.SourceID, State: freshness.StateFresh}
}
