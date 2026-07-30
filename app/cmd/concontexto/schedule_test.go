package main

// Remediation batch (verify-report CRITICAL C4): runScheduler's own
// scheduling/backoff behaviour, proven WITHOUT Docker via a fake op
// factory and a manually-fed tick channel -- the same injected-clock
// discipline app/internal/scheduler's own tests already use for
// scheduler.Runner.Run itself. schedule_integration_test.go separately
// proves one genuine end-to-end cycle against real Postgres.

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/scheduler"
)

// callCounter is a concurrency-safe per-source call counter: newOp is
// invoked from runScheduler's own goroutine, ticks are sent from the
// test goroutine, so every counter read must be race-safe.
type callCounter struct {
	mu        sync.Mutex
	calls     map[string]int
	scheduled chan struct{}
}

func newCallCounter() *callCounter {
	return &callCounter{calls: map[string]int{}, scheduled: make(chan struct{}, 64)}
}

func (c *callCounter) record(sourceID string) {
	c.mu.Lock()
	c.calls[sourceID]++
	c.mu.Unlock()
	c.scheduled <- struct{}{}
}

func (c *callCounter) count(sourceID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[sourceID]
}

// waitForCalls blocks until at least n cycles (across every source) have
// been recorded or the deadline expires.
func (c *callCounter) waitForCalls(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-c.scheduled:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for scheduled call %d/%d", i+1, n)
		}
	}
}

func TestRunScheduler_FirstTickRunsEveryConfiguredSource(t *testing.T) {
	counter := newCallCounter()
	newOp := func(sourceID string) func(context.Context) error {
		return func(context.Context) error {
			counter.record(sourceID)
			return nil
		}
	}

	runners := map[string]*scheduler.Runner{
		"ine":      scheduler.NewRunner("ine"),
		"eurostat": scheduler.NewRunner("eurostat"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	go runScheduler(ctx, runners, newOp, time.Hour, tick, nil, nil)

	tick <- time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	counter.waitForCalls(t, 2)

	if got := counter.count("ine"); got != 1 {
		t.Errorf("expected ine to run exactly once on the first tick, got %d", got)
	}
	if got := counter.count("eurostat"); got != 1 {
		t.Errorf("expected eurostat to run exactly once on the first tick, got %d", got)
	}
}

func TestRunScheduler_SuccessfulSourceWaitsTheFullIntervalBeforeItsNextCycle(t *testing.T) {
	counter := newCallCounter()
	newOp := func(sourceID string) func(context.Context) error {
		return func(context.Context) error {
			counter.record(sourceID)
			return nil
		}
	}
	runners := map[string]*scheduler.Runner{"ine": scheduler.NewRunner("ine")}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	interval := time.Hour
	go runScheduler(ctx, runners, newOp, interval, tick, nil, nil)

	base := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	tick <- base
	counter.waitForCalls(t, 1)

	// A tick well before base+interval must NOT re-run the source.
	tick <- base.Add(30 * time.Minute)
	select {
	case <-counter.scheduled:
		t.Fatal("expected no cycle before the interval elapsed")
	case <-time.After(200 * time.Millisecond):
	}
	if got := counter.count("ine"); got != 1 {
		t.Fatalf("expected exactly 1 call before the interval elapsed, got %d", got)
	}

	// A tick at/after base+interval must run it again.
	tick <- base.Add(interval)
	counter.waitForCalls(t, 1)
	if got := counter.count("ine"); got != 2 {
		t.Errorf("expected a second call once the interval elapsed, got %d", got)
	}
}

func TestRunScheduler_FailedSourceIsRetriedAtItsRunnerBackoffNotTheFullInterval(t *testing.T) {
	counter := newCallCounter()
	newOp := func(sourceID string) func(context.Context) error {
		return func(context.Context) error {
			counter.record(sourceID)
			return context.Canceled // any non-nil error, classification is scheduler.Runner's own concern
		}
	}
	// A 5-minute backoff, deliberately far shorter than the 24h interval
	// success would use, so a retry inside that gap distinguishes the
	// two schedules unambiguously.
	runner := &scheduler.Runner{SourceID: "ine", Backoff: func(int) time.Duration { return 5 * time.Minute }}
	runners := map[string]*scheduler.Runner{"ine": runner}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	go runScheduler(ctx, runners, newOp, 24*time.Hour, tick, nil, nil)

	base := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	tick <- base
	counter.waitForCalls(t, 1)

	// Just past the runner's own 5-minute backoff -- well inside the
	// 24h success interval -- must retry.
	tick <- base.Add(6 * time.Minute)
	counter.waitForCalls(t, 1)
	if got := counter.count("ine"); got != 2 {
		t.Fatalf("expected the failed source to retry at its own backoff, got %d calls", got)
	}
}

// TestScheduleInterval_FallsBackToDefaultWhenUnsetOrInvalid covers
// scheduleInterval's own APP_SCHEDULE_INTERVAL parsing (documented,
// check-env-example-enforced, previously 0.0% covered): unset and
// unparsable both fall back to the 24h default without ever failing;
// a valid override is honoured verbatim.
func TestScheduleInterval_FallsBackToDefaultWhenUnsetOrInvalid(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "unset uses the 24h default", env: "", want: defaultScheduleInterval},
		{name: "an unparsable duration falls back to the default", env: "not-a-duration", want: defaultScheduleInterval},
		{name: "a valid override is honoured", env: "1h30m", want: 90 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_SCHEDULE_INTERVAL", tt.env)
			var logs bytes.Buffer
			if got := scheduleInterval(&logs); got != tt.want {
				t.Fatalf("scheduleInterval() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestRunScheduler_InvokesWatchdogEveryTickOnceASuccessIsKnown covers task
// 4.11/4.12's own wiring point: the publish-latency watchdog
// (app/internal/scheduler.PublishLatencyBreached) must be evaluated on
// EVERY tick for every source that already has a known success,
// independent of whether that source's own ingest cycle is due this tick
// -- a stalled rebuild can only be caught by checking regularly, not only
// when the 24h ingest interval happens to come back around. The watchdog
// cannot fire on the very FIRST tick a source is seen (there is nothing
// known yet to compare against, by construction -- lastSuccess is only
// populated AFTER that tick's own op returns).
func TestRunScheduler_InvokesWatchdogEveryTickOnceASuccessIsKnown(t *testing.T) {
	counter := newCallCounter()
	newOp := func(sourceID string) func(context.Context) error {
		return func(context.Context) error {
			counter.record(sourceID)
			return nil
		}
	}
	runners := map[string]*scheduler.Runner{"ine": scheduler.NewRunner("ine")}

	type watchdogCall struct {
		sourceID    string
		now         time.Time
		lastSuccess time.Time
	}
	var mu sync.Mutex
	var calls []watchdogCall
	watchdogCalled := make(chan struct{}, 64)
	watchdog := func(sourceID string, now, lastSuccess time.Time) {
		mu.Lock()
		calls = append(calls, watchdogCall{sourceID, now, lastSuccess})
		mu.Unlock()
		watchdogCalled <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	// A 24h interval means the source will NOT be due again on the second
	// or third tick below -- the watchdog must still fire for it.
	go runScheduler(ctx, runners, newOp, 24*time.Hour, tick, nil, watchdog)

	base := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	tick <- base
	counter.waitForCalls(t, 1)

	// Second tick: lastSuccess is now known (seeded by the first tick's
	// own successful op) -- the watchdog MUST fire, even though the
	// source's own ingest is not due again for another 24h.
	secondTick := base.Add(2 * time.Hour)
	tick <- secondTick
	select {
	case <-watchdogCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the watchdog to fire on the second tick, once a success is known, even though the source's ingest was not due")
	}

	// Third tick, still well before the 24h re-ingest interval: the
	// watchdog must fire again, proving it runs on a REGULAR cadence, not
	// only once.
	thirdTick := base.Add(4 * time.Hour)
	tick <- thirdTick
	select {
	case <-watchdogCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the watchdog to fire on the third tick too")
	}

	if got := counter.count("ine"); got != 1 {
		t.Fatalf("expected the source's own op to run exactly once (not due again on ticks 2/3), got %d", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected exactly 2 watchdog invocations (none on the first tick, one each on the second and third), got %d: %+v", len(calls), calls)
	}
	if calls[0].sourceID != "ine" || !calls[0].now.Equal(secondTick) || !calls[0].lastSuccess.Equal(base) {
		t.Errorf("expected the first watchdog call to report (ine, %v, %v), got %+v", secondTick, base, calls[0])
	}
	if calls[1].sourceID != "ine" || !calls[1].now.Equal(thirdTick) || !calls[1].lastSuccess.Equal(base) {
		t.Errorf("expected the second watchdog call to report (ine, %v, %v), got %+v", thirdTick, base, calls[1])
	}
}

func TestRunScheduler_StopsWhenContextIsCancelled(t *testing.T) {
	counter := newCallCounter()
	newOp := func(sourceID string) func(context.Context) error {
		return func(context.Context) error {
			counter.record(sourceID)
			return nil
		}
	}
	runners := map[string]*scheduler.Runner{"ine": scheduler.NewRunner("ine")}

	ctx, cancel := context.WithCancel(context.Background())
	tick := make(chan time.Time)
	done := make(chan struct{})
	go func() {
		runScheduler(ctx, runners, newOp, time.Hour, tick, nil, nil)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected runScheduler to return promptly once ctx is cancelled")
	}
}
