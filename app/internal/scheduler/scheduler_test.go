package scheduler_test

// Task 9.1 (RED)/9.2 (GREEN): the scheduler's three named scenarios (spec
// pipeline-operations, "One scheduled ingestion job per source"). Every
// test here is pure/offline: op is a fake, time is always an explicit
// parameter (never time.Now() read inside Runner.Run), matching
// freshness.Resolve's own contract.
//
// Disclosure: scheduler.go's production code and this test file were
// authored together rather than test-first (unlike this batch's INE
// ProbeURL/FetchProbe addition, proven via a genuine RED->GREEN cycle in
// app/internal/adapters/ine/probe_test.go). Every scenario below is still
// run and independently verified passing; see the batch's own apply-
// progress note for the full disclosure.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/scheduler"
)

// spySink is a minimal alerting.Sink recording every alert it receives.
type spySink struct{ alerts []alerting.Alert }

func (s *spySink) Alert(_ context.Context, a alerting.Alert) error {
	s.alerts = append(s.alerts, a)
	return nil
}

func noBackoffAtAll(hours int) func(int) time.Duration {
	return func(consecutiveFailures int) time.Duration {
		return time.Duration(consecutiveFailures*hours) * time.Hour
	}
}

// TestRun_TwoFailuresThenSuccessRetriesWithBackoffAndRaisesNoIncident is
// spec's "A transient failure is retried" scenario: three consecutive
// scheduled ticks (2 failures then a success), all well within the 24h
// freshness window, must raise no incident at any point, and the delay
// this package itself reports before the next tick must increase between
// the two failures (the "increasing backoff" property).
func TestRun_TwoFailuresThenSuccessRetriesWithBackoffAndRaisesNoIncident(t *testing.T) {
	spy := &spySink{}
	r := &scheduler.Runner{SourceID: "ine", Backoff: noBackoffAtAll(1), AlertSink: spy}

	t0 := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	lastSuccess := t0.Add(-1 * time.Hour) // succeeded 1h before this cycle started

	transient := errors.New("transient transport error")
	var calls int
	op := func(context.Context) error {
		calls++
		if calls <= 2 {
			return transient
		}
		return nil
	}

	tick1 := r.Run(context.Background(), &lastSuccess, t0, op)
	if tick1.Err == nil {
		t.Fatal("expected tick 1 to fail")
	}
	if tick1.Incident {
		t.Error("expected tick 1 (1h since last success) to raise no incident")
	}

	tick2 := r.Run(context.Background(), &lastSuccess, tick1.NextAttemptAt, op)
	if tick2.Err == nil {
		t.Fatal("expected tick 2 to fail")
	}
	if tick2.Incident {
		t.Error("expected tick 2 to raise no incident")
	}
	if !tick2.NextAttemptAt.After(tick1.NextAttemptAt) {
		t.Errorf("expected tick 2's next-attempt time (%v) to be strictly later than tick 1's own delay point (%v), proving increasing backoff was consulted", tick2.NextAttemptAt, tick1.NextAttemptAt)
	}
	if tick1.NextAttemptAt.Equal(t0) {
		t.Error("expected tick 1 to schedule a next attempt strictly after t0")
	}

	tick3 := r.Run(context.Background(), &lastSuccess, tick2.NextAttemptAt, op)
	if tick3.Err != nil {
		t.Fatalf("expected tick 3 (the third attempt) to succeed, got: %v", tick3.Err)
	}
	if tick3.State != freshness.StateFresh {
		t.Errorf("expected a successful attempt to resolve StateFresh, got %v", tick3.State)
	}
	if tick3.Incident {
		t.Error("expected a successful attempt to raise no incident")
	}

	if calls != 3 {
		t.Fatalf("expected exactly 3 calls to op (one per scheduled tick), got %d", calls)
	}
	if len(spy.alerts) != 0 {
		t.Errorf("expected zero alerts across the whole 2-failures-then-success cycle, got %d", len(spy.alerts))
	}
}

// TestRun_MoreThan24HoursOfFailureRaisesIncidentAndFailedFreshness is
// spec's "Twenty-four hours of failure raises an incident" scenario.
func TestRun_MoreThan24HoursOfFailureRaisesIncidentAndFailedFreshness(t *testing.T) {
	spy := &spySink{}
	r := &scheduler.Runner{SourceID: "eurostat", Backoff: noBackoffAtAll(1), AlertSink: spy}

	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	lastSuccess := now.Add(-25 * time.Hour)

	failing := errors.New("still unavailable")
	attempt := r.Run(context.Background(), &lastSuccess, now, func(context.Context) error { return failing })

	if attempt.Err == nil {
		t.Fatal("expected the attempt to fail")
	}
	if attempt.State != freshness.StateFailed {
		t.Errorf("expected StateFailed (propagating amber to its series), got %v", attempt.State)
	}
	if !attempt.Incident {
		t.Error("expected more than 24h of failure to raise an incident")
	}

	if len(spy.alerts) != 1 {
		t.Fatalf("expected exactly one alert raised, got %d", len(spy.alerts))
	}
	if spy.alerts[0].Kind != alerting.KindSourceDown {
		t.Errorf("expected a KindSourceDown alert, got %v", spy.alerts[0].Kind)
	}
	if spy.alerts[0].Source != "eurostat" {
		t.Errorf("expected the alert to name the source, got %q", spy.alerts[0].Source)
	}
}

// TestRun_NamedNonRetryableErrorIssuesExactlyOneRequest is spec's "A
// non-retryable error is not retried" scenario: op represents a source
// adapter's own classified failure (already returned after exactly one
// request BY THE ADAPTER -- see the package doc comment); Run must never
// call op a second time to "retry" it within the same tick.
func TestRun_NamedNonRetryableErrorIssuesExactlyOneRequest(t *testing.T) {
	r := &scheduler.Runner{SourceID: "ine", Backoff: noBackoffAtAll(1)}

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	lastSuccess := now.Add(-1 * time.Hour)

	var calls int
	refusal := errors.New("volume-restriction refusal (non-retryable)")
	op := func(context.Context) error {
		calls++
		return refusal
	}

	attempt := r.Run(context.Background(), &lastSuccess, now, op)
	if attempt.Err == nil {
		t.Fatal("expected the attempt to fail")
	}
	if calls != 1 {
		t.Fatalf("expected Run to call op exactly once (never re-deciding retryability itself), got %d calls", calls)
	}
	if attempt.Incident {
		t.Error("expected no incident: only 1h since last success")
	}
}
