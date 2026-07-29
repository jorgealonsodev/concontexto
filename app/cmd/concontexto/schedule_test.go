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
	go runScheduler(ctx, runners, newOp, time.Hour, tick, nil)

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
	go runScheduler(ctx, runners, newOp, interval, tick, nil)

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
	go runScheduler(ctx, runners, newOp, 24*time.Hour, tick, nil)

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
		runScheduler(ctx, runners, newOp, time.Hour, tick, nil)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected runScheduler to return promptly once ctx is cancelled")
	}
}
